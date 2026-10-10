package service

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	cmdbDao "dodevops-api/api/cmdb/dao"
	"dodevops-api/api/cmdb/model"
	"dodevops-api/common/constant"
	"dodevops-api/common/result"
	"dodevops-api/common/util"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type CmdbIdcAssetServiceInterface interface {
	CreateAsset(c *gin.Context, dto *model.CreateCmdbIdcAssetDto)
	UpdateAsset(c *gin.Context, dto *model.UpdateCmdbIdcAssetDto)
	UpdateAssetPosition(c *gin.Context, dto *model.UpdateCmdbIdcAssetPositionDto)
	DeleteAsset(c *gin.Context, id uint)
	GetAssetById(c *gin.Context, id uint)
	GetAssetList(c *gin.Context, query *model.CmdbIdcAssetQuery)
	ImportAssetsFromExcel(c *gin.Context, rows []model.ExcelCmdbIdcAssetRow)
}

type CmdbIdcAssetServiceImpl struct {
	assetDao cmdbDao.CmdbIdcAssetDao
	roomDao  cmdbDao.CmdbIdcRoomDao
	vmDao    cmdbDao.CmdbIdcVmDao
}

func GetCmdbIdcAssetService() CmdbIdcAssetServiceInterface {
	return &CmdbIdcAssetServiceImpl{
		assetDao: cmdbDao.NewCmdbIdcAssetDao(),
		roomDao:  cmdbDao.NewCmdbIdcRoomDao(),
		vmDao:    cmdbDao.NewCmdbIdcVmDao(),
	}
}

func (s *CmdbIdcAssetServiceImpl) CreateAsset(c *gin.Context, dto *model.CreateCmdbIdcAssetDto) {
	ip := strings.TrimSpace(dto.IP)
	if ip == "" {
		result.Failed(c, constant.INVALID_PARAMS, "IP不能为空")
		return
	}

	existAsset, err := s.assetDao.GetAssetByIP(ip)
	if err == nil && existAsset != nil {
		result.Failed(c, constant.IDC_ASSET_IP_EXISTS, "IP已存在")
		return
	}
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		result.Failed(c, constant.IDC_ASSET_CREATE_FAILED, err.Error())
		return
	}

	room, cabinet, err := s.resolveRoomAndCabinet(dto.RoomID, dto.RoomName, dto.CabinetID, dto.CabinetName, true)
	if err != nil {
		result.Failed(c, constant.INVALID_PARAMS, err.Error())
		return
	}

	cabinetUSize, err := normalizeCabinetUSize(dto.CabinetUSize)
	if err != nil {
		result.Failed(c, constant.INVALID_PARAMS, err.Error())
		return
	}
	cabinetPosition, err := s.validatePlacement(cabinet, dto.CabinetPosition, cabinetUSize, 0)
	if err != nil {
		result.Failed(c, constant.INVALID_PARAMS, err.Error())
		return
	}

	now := util.HTime{Time: time.Now()}
	asset := model.CmdbIdcAsset{
		RoomID:               room.ID,
		CabinetID:            cabinetIDValue(cabinet),
		CabinetPosition:      cabinetPosition,
		CabinetUSize:         cabinetUSize,
		IP:                   ip,
		Brand:                strings.TrimSpace(dto.Brand),
		Model:                strings.TrimSpace(dto.Model),
		CPU:                  strings.TrimSpace(dto.CPU),
		Disk:                 strings.TrimSpace(dto.Disk),
		Memory:               strings.TrimSpace(dto.Memory),
		Purpose:              strings.TrimSpace(dto.Purpose),
		RemoteManagementCard: strings.TrimSpace(dto.RemoteManagementCard),
		Remark:               strings.TrimSpace(dto.Remark),
		PurchaseDate:         normalizeDateString(dto.PurchaseDate),
		WarrantyStartDate:    normalizeDateString(dto.WarrantyStartDate),
		WarrantyEndDate:      normalizeDateString(dto.WarrantyEndDate),
		UsageYears:           normalizeUsageYears(dto.UsageYears, dto.PurchaseDate),
		AssetEvaluation:      strings.TrimSpace(dto.AssetEvaluation),
		CreateTime:           now,
		UpdateTime:           now,
	}

	if err := s.assetDao.CreateAsset(&asset); err != nil {
		result.Failed(c, constant.IDC_ASSET_CREATE_FAILED, err.Error())
		return
	}

	result.Success(c, s.buildAssetVO(asset, room.Name, cabinetNameValue(cabinet), 0))
}

func (s *CmdbIdcAssetServiceImpl) UpdateAsset(c *gin.Context, dto *model.UpdateCmdbIdcAssetDto) {
	existAsset, err := s.assetDao.GetAssetById(dto.ID)
	if err != nil {
		result.Failed(c, constant.IDC_ASSET_NOT_FOUND, "资产不存在")
		return
	}

	ip := strings.TrimSpace(dto.IP)
	if ip == "" {
		result.Failed(c, constant.INVALID_PARAMS, "IP不能为空")
		return
	}

	sameIPAsset, err := s.assetDao.GetAssetByIP(ip)
	if err == nil && sameIPAsset != nil && sameIPAsset.ID != existAsset.ID {
		result.Failed(c, constant.IDC_ASSET_IP_EXISTS, "IP已存在")
		return
	}
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		result.Failed(c, constant.IDC_ASSET_UPDATE_FAILED, err.Error())
		return
	}

	roomChanged := dto.RoomID > 0 || strings.TrimSpace(dto.RoomName) != ""
	roomID := dto.RoomID
	roomName := dto.RoomName
	if roomID == 0 && strings.TrimSpace(roomName) == "" {
		roomID = existAsset.RoomID
	}

	cabinetID := dto.CabinetID
	cabinetName := dto.CabinetName
	if !roomChanged && cabinetID == 0 && strings.TrimSpace(cabinetName) == "" && existAsset.CabinetID > 0 {
		cabinetID = existAsset.CabinetID
	}

	cabinetPositionInput := dto.CabinetPosition
	if !roomChanged && cabinetID == 0 && strings.TrimSpace(cabinetName) == "" && cabinetPositionInput == 0 && existAsset.CabinetPosition > 0 {
		cabinetPositionInput = existAsset.CabinetPosition
	}

	cabinetUSizeInput := dto.CabinetUSize
	if cabinetUSizeInput <= 0 {
		cabinetUSizeInput = existAsset.CabinetUSize
	}

	room, cabinet, err := s.resolveRoomAndCabinet(roomID, roomName, cabinetID, cabinetName, false)
	if err != nil {
		result.Failed(c, constant.INVALID_PARAMS, err.Error())
		return
	}

	cabinetUSize, err := normalizeCabinetUSize(cabinetUSizeInput)
	if err != nil {
		result.Failed(c, constant.INVALID_PARAMS, err.Error())
		return
	}
	cabinetPosition, err := s.validatePlacement(cabinet, cabinetPositionInput, cabinetUSize, existAsset.ID)
	if err != nil {
		result.Failed(c, constant.INVALID_PARAMS, err.Error())
		return
	}

	updateData := model.CmdbIdcAsset{
		RoomID:               room.ID,
		CabinetID:            cabinetIDValue(cabinet),
		CabinetPosition:      cabinetPosition,
		CabinetUSize:         cabinetUSize,
		IP:                   ip,
		Brand:                strings.TrimSpace(dto.Brand),
		Model:                strings.TrimSpace(dto.Model),
		CPU:                  strings.TrimSpace(dto.CPU),
		Disk:                 strings.TrimSpace(dto.Disk),
		Memory:               strings.TrimSpace(dto.Memory),
		Purpose:              strings.TrimSpace(dto.Purpose),
		RemoteManagementCard: strings.TrimSpace(dto.RemoteManagementCard),
		Remark:               strings.TrimSpace(dto.Remark),
		PurchaseDate:         normalizeDateString(dto.PurchaseDate),
		WarrantyStartDate:    normalizeDateString(dto.WarrantyStartDate),
		WarrantyEndDate:      normalizeDateString(dto.WarrantyEndDate),
		UsageYears:           normalizeUsageYears(dto.UsageYears, dto.PurchaseDate),
		AssetEvaluation:      strings.TrimSpace(dto.AssetEvaluation),
		UpdateTime:           util.HTime{Time: time.Now()},
	}

	if err := s.assetDao.UpdateAsset(existAsset.ID, &updateData); err != nil {
		result.Failed(c, constant.IDC_ASSET_UPDATE_FAILED, err.Error())
		return
	}

	result.Success(c, true)
}

func (s *CmdbIdcAssetServiceImpl) UpdateAssetPosition(c *gin.Context, dto *model.UpdateCmdbIdcAssetPositionDto) {
	existAsset, err := s.assetDao.GetAssetById(dto.ID)
	if err != nil {
		result.Failed(c, constant.IDC_ASSET_NOT_FOUND, "资产不存在")
		return
	}

	targetCabinetID := dto.CabinetID
	if targetCabinetID == 0 {
		targetCabinetID = existAsset.CabinetID
	}

	var cabinet *model.CmdbIdcCabinet
	targetRoomID := existAsset.RoomID
	if targetCabinetID > 0 {
		cabinetValue, err := s.roomDao.GetCabinetById(targetCabinetID)
		if err != nil {
			result.Failed(c, constant.IDC_CABINET_NOT_FOUND, "机柜不存在")
			return
		}
		cabinet = &cabinetValue
		targetRoomID = cabinet.RoomID
	}

	cabinetUSizeInput := dto.CabinetUSize
	if cabinetUSizeInput <= 0 {
		cabinetUSizeInput = existAsset.CabinetUSize
	}
	cabinetUSize, err := normalizeCabinetUSize(cabinetUSizeInput)
	if err != nil {
		result.Failed(c, constant.INVALID_PARAMS, err.Error())
		return
	}

	cabinetPosition, err := s.validatePlacement(cabinet, dto.CabinetPosition, cabinetUSize, existAsset.ID)
	if err != nil {
		result.Failed(c, constant.INVALID_PARAMS, err.Error())
		return
	}

	updateData := model.CmdbIdcAsset{
		RoomID:          targetRoomID,
		CabinetID:       targetCabinetID,
		CabinetPosition: cabinetPosition,
		CabinetUSize:    cabinetUSize,
		UpdateTime:      util.HTime{Time: time.Now()},
	}
	if err := s.assetDao.UpdateAsset(existAsset.ID, &updateData); err != nil {
		result.Failed(c, constant.IDC_ASSET_UPDATE_FAILED, err.Error())
		return
	}

	result.Success(c, true)
}

func (s *CmdbIdcAssetServiceImpl) DeleteAsset(c *gin.Context, id uint) {
	if _, err := s.assetDao.GetAssetById(id); err != nil {
		result.Failed(c, constant.IDC_ASSET_NOT_FOUND, "资产不存在")
		return
	}
	vmCount, err := s.vmDao.CountByPhysicalAssetID(id)
	if err != nil {
		result.Failed(c, constant.IDC_ASSET_DELETE_FAILED, err.Error())
		return
	}
	if vmCount > 0 {
		result.Failed(c, constant.IDC_ASSET_DELETE_FAILED, "物理机下存在关联虚拟机，无法删除")
		return
	}
	if err := s.assetDao.DeleteAsset(id); err != nil {
		result.Failed(c, constant.IDC_ASSET_DELETE_FAILED, err.Error())
		return
	}
	result.Success(c, true)
}

func (s *CmdbIdcAssetServiceImpl) GetAssetById(c *gin.Context, id uint) {
	asset, err := s.assetDao.GetAssetById(id)
	if err != nil {
		result.Failed(c, constant.IDC_ASSET_NOT_FOUND, "资产不存在")
		return
	}
	vmCount, err := s.vmDao.CountByPhysicalAssetID(asset.ID)
	if err != nil {
		result.Failed(c, constant.IDC_ASSET_NOT_FOUND, err.Error())
		return
	}
	result.Success(c, s.buildAssetVO(asset, asset.Room.Name, asset.Cabinet.Name, vmCount))
}

func (s *CmdbIdcAssetServiceImpl) GetAssetList(c *gin.Context, query *model.CmdbIdcAssetQuery) {
	if query.SortBy == "" {
		query.SortBy = "id"
	}
	if query.SortOrder == "" {
		query.SortOrder = "desc"
	}

	list, total, err := s.assetDao.GetAssetList(query)
	if err != nil {
		result.Failed(c, constant.IDC_ASSET_NOT_FOUND, err.Error())
		return
	}

	vmCountMap, err := s.vmDao.CountByPhysicalAssetIDs(collectAssetIDs(list))
	if err != nil {
		result.Failed(c, constant.IDC_ASSET_NOT_FOUND, err.Error())
		return
	}

	vos := s.buildAssetVOList(list, vmCountMap)
	result.SuccessWithPage(c, vos, total, query.Page, query.PageSize)
}

func (s *CmdbIdcAssetServiceImpl) ImportAssetsFromExcel(c *gin.Context, rows []model.ExcelCmdbIdcAssetRow) {
	if len(rows) == 0 {
		result.Failed(c, constant.INVALID_PARAMS, "Excel中没有可导入的数据")
		return
	}

	successCount := 0
	failCount := 0
	failedRows := make([]string, 0)
	seenIPs := make(map[string]struct{}, len(rows))

	for index, row := range rows {
		rowNo := index + 2
		ip := strings.TrimSpace(row.IP)
		if ip == "" {
			failCount++
			failedRows = append(failedRows, fmt.Sprintf("第%d行：IP不能为空", rowNo))
			continue
		}
		if _, ok := seenIPs[ip]; ok {
			failCount++
			failedRows = append(failedRows, fmt.Sprintf("第%d行：IP重复(%s)", rowNo, ip))
			continue
		}
		seenIPs[ip] = struct{}{}

		existAsset, err := s.assetDao.GetAssetByIP(ip)
		if err == nil && existAsset != nil {
			failCount++
			failedRows = append(failedRows, fmt.Sprintf("第%d行：IP已存在(%s)", rowNo, ip))
			continue
		}
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			failCount++
			failedRows = append(failedRows, fmt.Sprintf("第%d行：查询失败(%s)", rowNo, err.Error()))
			continue
		}

		room, cabinet, err := s.resolveRoomAndCabinet(0, row.RoomName, 0, row.CabinetName, true)
		if err != nil {
			failCount++
			failedRows = append(failedRows, fmt.Sprintf("第%d行：%s", rowNo, err.Error()))
			continue
		}

		cabinetUSize, err := normalizeCabinetUSize(row.CabinetUSize)
		if err != nil {
			failCount++
			failedRows = append(failedRows, fmt.Sprintf("第%d行：%s", rowNo, err.Error()))
			continue
		}
		cabinetPosition, err := s.validatePlacement(cabinet, row.CabinetPosition, cabinetUSize, 0)
		if err != nil {
			failCount++
			failedRows = append(failedRows, fmt.Sprintf("第%d行：%s", rowNo, err.Error()))
			continue
		}

		now := util.HTime{Time: time.Now()}
		asset := model.CmdbIdcAsset{
			RoomID:               room.ID,
			CabinetID:            cabinetIDValue(cabinet),
			CabinetPosition:      cabinetPosition,
			CabinetUSize:         cabinetUSize,
			IP:                   ip,
			Brand:                strings.TrimSpace(row.Brand),
			Model:                strings.TrimSpace(row.Model),
			CPU:                  strings.TrimSpace(row.CPU),
			Disk:                 strings.TrimSpace(row.Disk),
			Memory:               strings.TrimSpace(row.Memory),
			Purpose:              strings.TrimSpace(row.Purpose),
			RemoteManagementCard: strings.TrimSpace(row.RemoteManagementCard),
			Remark:               strings.TrimSpace(row.Remark),
			PurchaseDate:         normalizeDateString(row.PurchaseDate),
			WarrantyStartDate:    normalizeDateString(row.WarrantyStartDate),
			WarrantyEndDate:      normalizeDateString(row.WarrantyEndDate),
			UsageYears:           normalizeUsageYears(parseFloatValue(row.UsageYears), row.PurchaseDate),
			AssetEvaluation:      strings.TrimSpace(row.AssetEvaluation),
			CreateTime:           now,
			UpdateTime:           now,
		}

		if err := s.assetDao.CreateAsset(&asset); err != nil {
			failCount++
			failedRows = append(failedRows, fmt.Sprintf("第%d行：%s", rowNo, err.Error()))
			continue
		}
		successCount++
	}

	result.Success(c, gin.H{
		"success":    successCount,
		"fail":       failCount,
		"total":      len(rows),
		"failedRows": failedRows,
	})
}

func (s *CmdbIdcAssetServiceImpl) resolveRoomAndCabinet(roomID uint, roomName string, cabinetID uint, cabinetName string, allowCreate bool) (model.CmdbIdcRoom, *model.CmdbIdcCabinet, error) {
	trimmedRoomName := strings.TrimSpace(roomName)
	trimmedCabinetName := strings.TrimSpace(cabinetName)

	var room model.CmdbIdcRoom
	var cabinet *model.CmdbIdcCabinet
	var err error

	if cabinetID > 0 {
		cabinetValue, cabinetErr := s.roomDao.GetCabinetById(cabinetID)
		if cabinetErr != nil {
			return room, nil, fmt.Errorf("机柜不存在")
		}
		cabinet = &cabinetValue
		if roomID == 0 && trimmedRoomName == "" {
			roomID = cabinet.RoomID
		}
	}

	switch {
	case roomID > 0:
		room, err = s.roomDao.GetRoomById(roomID)
		if err != nil {
			return room, nil, fmt.Errorf("机房不存在")
		}
	case trimmedRoomName != "":
		roomValue, roomErr := s.roomDao.GetRoomByName(trimmedRoomName)
		if roomErr == nil {
			room = *roomValue
		} else if errors.Is(roomErr, gorm.ErrRecordNotFound) && allowCreate {
			now := util.HTime{Time: time.Now()}
			newRoom := model.CmdbIdcRoom{
				Name:       trimmedRoomName,
				CreateTime: now,
				UpdateTime: now,
			}
			if createErr := s.roomDao.CreateRoom(&newRoom); createErr != nil {
				return room, nil, fmt.Errorf("创建机房失败: %s", createErr.Error())
			}
			room = newRoom
		} else if roomErr != nil {
			if errors.Is(roomErr, gorm.ErrRecordNotFound) {
				return room, nil, fmt.Errorf("机房不存在")
			}
			return room, nil, roomErr
		}
	default:
		return room, nil, fmt.Errorf("机房不能为空")
	}

	if cabinet != nil {
		if cabinet.RoomID != room.ID {
			return room, nil, fmt.Errorf("机柜不属于当前机房")
		}
		return room, cabinet, nil
	}

	if trimmedCabinetName == "" {
		return room, nil, nil
	}

	cabinetValue, cabinetErr := s.roomDao.GetCabinetByRoomAndName(room.ID, trimmedCabinetName)
	if cabinetErr == nil {
		return room, cabinetValue, nil
	}
	if !errors.Is(cabinetErr, gorm.ErrRecordNotFound) {
		return room, nil, cabinetErr
	}
	if !allowCreate {
		return room, nil, fmt.Errorf("机柜不存在")
	}

	now := util.HTime{Time: time.Now()}
	newCabinet := model.CmdbIdcCabinet{
		RoomID:     room.ID,
		Name:       trimmedCabinetName,
		TotalU:     42,
		CreateTime: now,
		UpdateTime: now,
	}
	if err := s.roomDao.CreateCabinet(&newCabinet); err != nil {
		return room, nil, fmt.Errorf("创建机柜失败: %s", err.Error())
	}
	return room, &newCabinet, nil
}

func (s *CmdbIdcAssetServiceImpl) validatePlacement(cabinet *model.CmdbIdcCabinet, position, uSize int, excludeID uint) (int, error) {
	if position < 0 {
		return 0, fmt.Errorf("机柜位置不能小于0")
	}
	if position > 0 && cabinet == nil {
		return 0, fmt.Errorf("未指定机柜时不能设置机柜位置")
	}
	if position == 0 || cabinet == nil {
		return position, nil
	}

	totalU, err := normalizeCabinetTotalU(cabinet.TotalU)
	if err != nil {
		return 0, err
	}
	endU := position + uSize - 1
	if endU > totalU {
		return 0, fmt.Errorf("位置超出机柜范围，结束U位不能大于%d", totalU)
	}

	hasConflict, err := s.assetDao.HasCabinetPositionConflict(cabinet.ID, position, endU, excludeID)
	if err != nil {
		return 0, err
	}
	if hasConflict {
		return 0, fmt.Errorf("机柜U位冲突，请调整位置或U高度")
	}
	return position, nil
}

func (s *CmdbIdcAssetServiceImpl) buildAssetVOList(list []model.CmdbIdcAsset, vmCountMap map[uint]int64) []model.CmdbIdcAssetVo {
	vos := make([]model.CmdbIdcAssetVo, 0, len(list))
	for _, item := range list {
		vos = append(vos, s.buildAssetVO(item, item.Room.Name, item.Cabinet.Name, vmCountMap[item.ID]))
	}
	return vos
}

func (s *CmdbIdcAssetServiceImpl) buildAssetVO(asset model.CmdbIdcAsset, roomName, cabinetName string, vmCount int64) model.CmdbIdcAssetVo {
	return model.CmdbIdcAssetVo{
		ID:                   asset.ID,
		RoomID:               asset.RoomID,
		RoomName:             roomName,
		CabinetID:            asset.CabinetID,
		CabinetName:          cabinetName,
		CabinetPosition:      asset.CabinetPosition,
		CabinetUSize:         normalizeCabinetUSizeDefault(asset.CabinetUSize),
		VmCount:              vmCount,
		IP:                   asset.IP,
		Brand:                asset.Brand,
		Model:                asset.Model,
		CPU:                  asset.CPU,
		Disk:                 asset.Disk,
		Memory:               asset.Memory,
		Purpose:              asset.Purpose,
		RemoteManagementCard: asset.RemoteManagementCard,
		Remark:               asset.Remark,
		PurchaseDate:         asset.PurchaseDate,
		WarrantyStartDate:    asset.WarrantyStartDate,
		WarrantyEndDate:      asset.WarrantyEndDate,
		UsageYears:           asset.UsageYears,
		AssetEvaluation:      asset.AssetEvaluation,
		CreateTime:           asset.CreateTime,
		UpdateTime:           asset.UpdateTime,
	}
}

func collectAssetIDs(list []model.CmdbIdcAsset) []uint {
	ids := make([]uint, 0, len(list))
	for _, item := range list {
		if item.ID == 0 {
			continue
		}
		ids = append(ids, item.ID)
	}
	return ids
}

func cabinetIDValue(cabinet *model.CmdbIdcCabinet) uint {
	if cabinet == nil {
		return 0
	}
	return cabinet.ID
}

func cabinetNameValue(cabinet *model.CmdbIdcCabinet) string {
	if cabinet == nil {
		return ""
	}
	return cabinet.Name
}

func normalizeCabinetUSize(value int) (int, error) {
	if value <= 0 {
		return 1, nil
	}
	if value > 20 {
		return 0, fmt.Errorf("机柜U高度不能大于20")
	}
	return value, nil
}

func normalizeCabinetUSizeDefault(value int) int {
	if value <= 0 {
		return 1
	}
	return value
}

func normalizeDateString(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	if parsed, ok := parseFlexibleDate(trimmed); ok {
		return parsed.Format("2006-01-02")
	}
	return trimmed
}

func normalizeUsageYears(value float64, purchaseDate string) float64 {
	if value > 0 {
		return roundFloat(value, 2)
	}

	parsedDate, ok := parseFlexibleDate(strings.TrimSpace(purchaseDate))
	if !ok {
		return 0
	}

	years := time.Since(parsedDate).Hours() / 24 / 365
	if years < 0 {
		return 0
	}
	return roundFloat(years, 2)
}

func parseFlexibleDate(value string) (time.Time, bool) {
	if value == "" {
		return time.Time{}, false
	}

	formats := []string{
		"2006-01-02",
		"2006/01/02",
		"2006.01.02",
		"2006-1-2",
		"2006/1/2",
		"2006.1.2",
		"2006-01-02 15:04:05",
		"2006/01/02 15:04:05",
		"2006-01-02 15:04",
		"2006/01/02 15:04",
	}

	for _, format := range formats {
		parsed, err := time.ParseInLocation(format, value, time.Local)
		if err == nil {
			return parsed, true
		}
	}

	return time.Time{}, false
}

func parseFloatValue(value string) float64 {
	trimmed := strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(value, ",", ""), "年", ""))
	if trimmed == "" {
		return 0
	}

	floatValue, err := strconv.ParseFloat(trimmed, 64)
	if err != nil {
		return 0
	}
	return floatValue
}

func roundFloat(value float64, precision int) float64 {
	pow := math.Pow10(precision)
	return math.Round(value*pow) / pow
}
