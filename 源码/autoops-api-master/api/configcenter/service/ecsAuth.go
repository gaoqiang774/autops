package service

import (
	"dodevops-api/api/configcenter/dao"
	"dodevops-api/api/configcenter/model"
	"dodevops-api/common/constant"
	"dodevops-api/common/result"
	"dodevops-api/common/util"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type EcsAuthServiceInterface interface {
	GetEcsAuthList(c *gin.Context)
	GetEcsAuthListWithPage(c *gin.Context, page, pageSize int)
	GetEcsAuthByName(c *gin.Context, name string)
	GetEcsAuthById(c *gin.Context, id uint)
	DecryptEcsAuth(c *gin.Context, id uint)
	CreateEcsAuth(c *gin.Context, dto *model.CreateEcsPasswordAuthDto)
	ImportEcsPasswordAuth(c *gin.Context, dto *model.ImportEcsPasswordAuthDto)
	UpdateEcsAuth(c *gin.Context, id uint, dto *model.CreateEcsPasswordAuthDto)
	MigrateEcsAuthSecrets(c *gin.Context)
	DeleteEcsAuth(c *gin.Context, id uint)
}

type EcsAuthServiceImpl struct {
	dao dao.EcsAuthDao
}

type ecsPasswordImportRow struct {
	Line     int
	Name     string
	Port     int
	Username string
	Password string
}

func buildEcsAuthVo(auth model.EcsAuth) model.EcsAuthVo {
	masked := auth
	masked.MaskSecrets()

	return model.EcsAuthVo{
		ID:                   auth.ID,
		Name:                 auth.Name,
		Type:                 auth.Type,
		Username:             auth.Username,
		Password:             masked.Password,
		PublicKey:            masked.PublicKey,
		Port:                 auth.Port,
		CreateTime:           auth.CreateTime,
		UpdateTime:           auth.UpdateTime,
		Remark:               auth.Remark,
		BastionHost:          auth.BastionHost,
		BastionPort:          auth.BastionPort,
		BastionUsername:      auth.BastionUsername,
		BastionAuthType:      auth.BastionAuthType,
		BastionPassword:      masked.BastionPassword,
		BastionPrivateKey:    masked.BastionPrivateKey,
		TargetAuthType:       auth.TargetAuthType,
		TargetPassword:       masked.TargetPassword,
		TargetPrivateKey:     masked.TargetPrivateKey,
		HasPassword:          auth.HasPassword(),
		HasPublicKey:         auth.HasPublicKey(),
		HasBastionPassword:   auth.HasBastionPassword(),
		HasBastionPrivateKey: auth.HasBastionPrivateKey(),
		HasTargetPassword:    auth.HasTargetPassword(),
		HasTargetPrivateKey:  auth.HasTargetPrivateKey(),
	}
}

func mergeEcsSecretValue(newValue, existingValue string) string {
	if newValue == "" || model.IsMaskedSecretValue(newValue) {
		return existingValue
	}
	return newValue
}

func ecsAuthSecretsChanged(before, after model.EcsAuth) bool {
	return before.Password != after.Password ||
		before.PublicKey != after.PublicKey ||
		before.BastionPassword != after.BastionPassword ||
		before.BastionPrivateKey != after.BastionPrivateKey ||
		before.TargetPassword != after.TargetPassword ||
		before.TargetPrivateKey != after.TargetPrivateKey
}

func nowHTime() util.HTime {
	return util.HTime{Time: time.Now()}
}

func (s *EcsAuthServiceImpl) GetEcsAuthList(c *gin.Context) {
	list := s.dao.GetEcsAuthList()
	vos := make([]model.EcsAuthVo, 0, len(list))
	for _, auth := range list {
		vos = append(vos, buildEcsAuthVo(auth))
	}
	result.Success(c, vos)
}

func (s *EcsAuthServiceImpl) GetEcsAuthListWithPage(c *gin.Context, page, pageSize int) {
	list, total := s.dao.GetEcsAuthListWithPage(page, pageSize)
	vos := make([]model.EcsAuthVo, 0, len(list))
	for _, auth := range list {
		vos = append(vos, buildEcsAuthVo(auth))
	}

	pageResult := result.PageResult{
		List:     vos,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}
	result.Success(c, pageResult)
}

func (s *EcsAuthServiceImpl) CreateEcsAuth(c *gin.Context, dto *model.CreateEcsPasswordAuthDto) {
	if s.dao.CheckNameExists(dto.Name) {
		result.FailedWithCode(c, constant.ECS_AUTH_NAME_EXISTS, "凭据名称已存在")
		return
	}

	now := nowHTime()
	auth := model.EcsAuth{
		Name:              dto.Name,
		Username:          dto.Username,
		Password:          dto.Password,
		Port:              dto.Port,
		CreateTime:        now,
		UpdateTime:        now,
		Remark:            dto.Remark,
		Type:              dto.Type,
		PublicKey:         dto.PublicKey,
		BastionHost:       dto.BastionHost,
		BastionPort:       dto.BastionPort,
		BastionUsername:   dto.BastionUsername,
		BastionAuthType:   dto.BastionAuthType,
		BastionPassword:   dto.BastionPassword,
		BastionPrivateKey: dto.BastionPrivateKey,
		TargetAuthType:    dto.TargetAuthType,
		TargetPassword:    dto.TargetPassword,
		TargetPrivateKey:  dto.TargetPrivateKey,
	}
	if err := auth.EncryptSecrets(); err != nil {
		result.FailedWithCode(c, constant.ECS_AUTH_CREATE_FAILED, err.Error())
		return
	}

	if err := s.dao.CreateEcsAuth(&auth); err != nil {
		result.FailedWithCode(c, constant.ECS_AUTH_CREATE_FAILED, err.Error())
		return
	}
	result.Success(c, true)
}

func (s *EcsAuthServiceImpl) ImportEcsPasswordAuth(c *gin.Context, dto *model.ImportEcsPasswordAuthDto) {
	rows, failedRows, total := parseEcsPasswordImportRows(dto.TableData)
	if total == 0 {
		result.Failed(c, constant.INVALID_PARAMS, "表格中没有可导入的数据")
		return
	}

	seenNames := make(map[string]struct{}, len(rows))
	successCount := 0

	for _, row := range rows {
		if _, exists := seenNames[row.Name]; exists {
			failedRows = append(failedRows, model.ImportEcsPasswordAuthFailedRow{
				Line:   row.Line,
				Name:   row.Name,
				Reason: "导入数据中存在重复的凭据名称",
			})
			continue
		}
		seenNames[row.Name] = struct{}{}

		if s.dao.CheckNameExists(row.Name) {
			failedRows = append(failedRows, model.ImportEcsPasswordAuthFailedRow{
				Line:   row.Line,
				Name:   row.Name,
				Reason: "凭据名称已存在",
			})
			continue
		}

		now := nowHTime()
		auth := model.EcsAuth{
			Name:       row.Name,
			Type:       1,
			Username:   row.Username,
			Password:   row.Password,
			Port:       row.Port,
			Remark:     dto.Remark,
			CreateTime: now,
			UpdateTime: now,
		}
		if err := auth.EncryptSecrets(); err != nil {
			failedRows = append(failedRows, model.ImportEcsPasswordAuthFailedRow{
				Line:   row.Line,
				Name:   row.Name,
				Reason: err.Error(),
			})
			continue
		}
		if err := s.dao.CreateEcsAuth(&auth); err != nil {
			failedRows = append(failedRows, model.ImportEcsPasswordAuthFailedRow{
				Line:   row.Line,
				Name:   row.Name,
				Reason: err.Error(),
			})
			continue
		}
		successCount++
	}

	responseData := gin.H{
		"total":   total,
		"success": successCount,
		"fail":    len(failedRows),
	}
	if len(failedRows) > 0 {
		responseData["failedRows"] = failedRows
	}
	result.Success(c, responseData)
}

func parseEcsPasswordImportRows(tableData string) ([]ecsPasswordImportRow, []model.ImportEcsPasswordAuthFailedRow, int) {
	normalized := strings.ReplaceAll(tableData, "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")
	lines := strings.Split(normalized, "\n")

	rows := make([]ecsPasswordImportRow, 0, len(lines))
	failedRows := make([]model.ImportEcsPasswordAuthFailedRow, 0)
	total := 0

	for idx, rawLine := range lines {
		line := strings.TrimSpace(rawLine)
		if line == "" {
			continue
		}

		fields := splitEcsImportFields(line)
		if len(fields) == 0 {
			continue
		}
		if isEcsImportHeader(fields) {
			continue
		}

		total++
		lineNo := idx + 1
		if len(fields) < 4 {
			failedRows = append(failedRows, model.ImportEcsPasswordAuthFailedRow{
				Line:   lineNo,
				Name:   "",
				Reason: "字段数量不足，需要 name port username password",
			})
			continue
		}

		name := strings.TrimSpace(fields[0])
		portText := strings.TrimSpace(fields[1])
		username := strings.TrimSpace(fields[2])
		password := strings.TrimSpace(strings.Join(fields[3:], " "))

		if name == "" {
			failedRows = append(failedRows, model.ImportEcsPasswordAuthFailedRow{
				Line:   lineNo,
				Name:   "",
				Reason: "name 不能为空",
			})
			continue
		}

		port, err := strconv.Atoi(portText)
		if err != nil || port <= 0 || port > 65535 {
			failedRows = append(failedRows, model.ImportEcsPasswordAuthFailedRow{
				Line:   lineNo,
				Name:   name,
				Reason: "端口格式错误或超出范围(1-65535)",
			})
			continue
		}

		if username == "" {
			failedRows = append(failedRows, model.ImportEcsPasswordAuthFailedRow{
				Line:   lineNo,
				Name:   name,
				Reason: "username 不能为空",
			})
			continue
		}

		if password == "" {
			failedRows = append(failedRows, model.ImportEcsPasswordAuthFailedRow{
				Line:   lineNo,
				Name:   name,
				Reason: "password 不能为空",
			})
			continue
		}

		rows = append(rows, ecsPasswordImportRow{
			Line:     lineNo,
			Name:     name,
			Port:     port,
			Username: username,
			Password: password,
		})
	}

	return rows, failedRows, total
}

func splitEcsImportFields(line string) []string {
	line = strings.TrimSpace(line)
	if line == "" {
		return nil
	}

	if strings.Contains(line, ",") || strings.Contains(line, "，") {
		replaced := strings.ReplaceAll(line, "，", ",")
		parts := strings.Split(replaced, ",")
		fields := make([]string, 0, len(parts))
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if part != "" {
				fields = append(fields, part)
			}
		}
		return fields
	}

	return strings.Fields(line)
}

func isEcsImportHeader(fields []string) bool {
	if len(fields) < 4 {
		return false
	}

	field0 := normalizeEcsImportHeader(fields[0])
	field1 := normalizeEcsImportHeader(fields[1])
	field2 := normalizeEcsImportHeader(fields[2])
	field3 := normalizeEcsImportHeader(fields[3])

	isName := field0 == "name" || field0 == "名称" || field0 == "凭据名称" || field0 == "ip" || field0 == "主机" || field0 == "主机名"
	isPort := field1 == "port" || field1 == "端口" || field1 == "sshport" || field1 == "ssh端口"
	isUser := field2 == "username" || field2 == "user" || field2 == "用户名" || field2 == "用户" || field2 == "ssh用户名"
	isPassword := field3 == "password" || field3 == "passwd" || field3 == "pwd" || field3 == "密码" || field3 == "口令"

	return isName && isPort && isUser && isPassword
}

func normalizeEcsImportHeader(value string) string {
	v := strings.TrimSpace(strings.ToLower(value))
	v = strings.ReplaceAll(v, "：", "")
	v = strings.ReplaceAll(v, ":", "")
	v = strings.ReplaceAll(v, " ", "")
	return v
}

func (s *EcsAuthServiceImpl) UpdateEcsAuth(c *gin.Context, id uint, dto *model.CreateEcsPasswordAuthDto) {
	existing, err := s.dao.GetEcsAuthById(id)
	if err != nil {
		result.FailedWithCode(c, constant.ECS_AUTH_NOT_FOUND, "鍑嵁涓嶅瓨鍦?")
		return
	}
	if err = existing.DecryptSecrets(); err != nil {
		result.FailedWithCode(c, constant.ECS_AUTH_UPDATE_FAILED, err.Error())
		return
	}

	auth := model.EcsAuth{
		Name:              dto.Name,
		Username:          dto.Username,
		Password:          mergeEcsSecretValue(dto.Password, existing.Password),
		Port:              dto.Port,
		Remark:            dto.Remark,
		Type:              dto.Type,
		PublicKey:         mergeEcsSecretValue(dto.PublicKey, existing.PublicKey),
		UpdateTime:        nowHTime(),
		BastionHost:       dto.BastionHost,
		BastionPort:       dto.BastionPort,
		BastionUsername:   dto.BastionUsername,
		BastionAuthType:   dto.BastionAuthType,
		BastionPassword:   mergeEcsSecretValue(dto.BastionPassword, existing.BastionPassword),
		BastionPrivateKey: mergeEcsSecretValue(dto.BastionPrivateKey, existing.BastionPrivateKey),
		TargetAuthType:    dto.TargetAuthType,
		TargetPassword:    mergeEcsSecretValue(dto.TargetPassword, existing.TargetPassword),
		TargetPrivateKey:  mergeEcsSecretValue(dto.TargetPrivateKey, existing.TargetPrivateKey),
	}
	if err = auth.EncryptSecrets(); err != nil {
		result.FailedWithCode(c, constant.ECS_AUTH_UPDATE_FAILED, err.Error())
		return
	}

	if err = s.dao.UpdateEcsAuth(id, &auth); err != nil {
		result.FailedWithCode(c, constant.ECS_AUTH_UPDATE_FAILED, err.Error())
		return
	}
	result.Success(c, true)
}

func (s *EcsAuthServiceImpl) MigrateEcsAuthSecrets(c *gin.Context) {
	list := s.dao.GetEcsAuthList()
	migrated := 0
	skipped := 0
	failedRows := make([]gin.H, 0)

	for _, auth := range list {
		original := auth
		if err := auth.EncryptSecrets(); err != nil {
			failedRows = append(failedRows, gin.H{
				"id":     auth.ID,
				"name":   auth.Name,
				"reason": err.Error(),
			})
			continue
		}

		if !ecsAuthSecretsChanged(original, auth) {
			skipped++
			continue
		}

		auth.UpdateTime = nowHTime()
		if err := s.dao.UpdateEcsAuth(auth.ID, &auth); err != nil {
			failedRows = append(failedRows, gin.H{
				"id":     auth.ID,
				"name":   auth.Name,
				"reason": err.Error(),
			})
			continue
		}
		migrated++
	}

	result.Success(c, gin.H{
		"total":    len(list),
		"migrated": migrated,
		"skipped":  skipped,
		"failed":   len(failedRows),
		"details":  failedRows,
	})
}

func (s *EcsAuthServiceImpl) DeleteEcsAuth(c *gin.Context, id uint) {
	auth, err := s.dao.GetEcsAuthById(id)
	if err != nil {
		result.FailedWithCode(c, constant.ECS_AUTH_NOT_FOUND, "凭据不存在")
		return
	}

	if auth.Type == 3 {
		result.Failed(c, int(result.ApiCode.FAILED), "免密认证无法删除")
		return
	}

	if err = s.dao.DeleteEcsAuth(id); err != nil {
		result.FailedWithCode(c, constant.ECS_AUTH_DELETE_FAILED, err.Error())
		return
	}
	result.Success(c, true)
}

func (s *EcsAuthServiceImpl) GetEcsAuthByName(c *gin.Context, name string) {
	list, err := s.dao.SearchEcsAuthByName(name)
	if err != nil {
		result.FailedWithCode(c, int(result.ApiCode.FAILED), err.Error())
		return
	}

	vos := make([]model.EcsAuthVo, 0, len(list))
	for _, auth := range list {
		vos = append(vos, buildEcsAuthVo(auth))
	}
	result.Success(c, vos)
}

func (s *EcsAuthServiceImpl) GetEcsAuthById(c *gin.Context, id uint) {
	auth, err := s.dao.GetEcsAuthById(id)
	if err != nil {
		result.FailedWithCode(c, constant.ECS_AUTH_NOT_FOUND, "凭据不存在")
		return
	}
	result.Success(c, buildEcsAuthVo(auth))
}

func (s *EcsAuthServiceImpl) DecryptEcsAuth(c *gin.Context, id uint) {
	auth, err := s.dao.GetEcsAuthById(id)
	if err != nil {
		result.FailedWithCode(c, constant.ECS_AUTH_NOT_FOUND, "凭据不存在")
		return
	}
	if err = auth.DecryptSecrets(); err != nil {
		result.FailedWithCode(c, int(result.ApiCode.FAILED), err.Error())
		return
	}

	result.Success(c, model.EcsAuthSecretVo{
		Password:          auth.Password,
		PublicKey:         auth.PublicKey,
		BastionPassword:   auth.BastionPassword,
		BastionPrivateKey: auth.BastionPrivateKey,
		TargetPassword:    auth.TargetPassword,
		TargetPrivateKey:  auth.TargetPrivateKey,
	})
}

func GetEcsAuthService() EcsAuthServiceInterface {
	return &EcsAuthServiceImpl{
		dao: dao.NewEcsAuthDao(),
	}
}
