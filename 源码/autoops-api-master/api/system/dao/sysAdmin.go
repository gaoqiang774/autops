package dao

import (
	"errors"
	"strings"
	"time"

	"dodevops-api/api/system/model"
	"dodevops-api/common/util"
	. "dodevops-api/pkg/db"

	"github.com/go-sql-driver/mysql"
	"gorm.io/gorm"
)

var ErrSysAdminUsernameExists = errors.New("sys admin username already exists")

// 用户详情
func SysAdminDetail(dto model.LoginDto) (sysAdmin model.SysAdmin) {
	username := dto.Username
	Db.Where("username = ?", username).First(&sysAdmin)
	return sysAdmin
}

// 根据用户名查询用户
func GetSysAdminByUsername(username string) (sysAdmin model.SysAdmin) {
	Db.Where("username = ?", username).First(&sysAdmin)
	return sysAdmin
}

// GetSysAdminAuthzState keeps the MCP identity lookup compatible with this
// branch, which does not persist JWT authorization fingerprints.
func GetSysAdminAuthzState(adminID uint) (model.SysAdmin, string, error) {
	var admin model.SysAdmin
	err := Db.First(&admin, adminID).Error
	return admin, "", err
}

// GetDefaultSystemAdmin resolves the first enabled account with the active
// admin role when mcp.userId is not explicitly configured.
func GetDefaultSystemAdmin() (model.SysAdmin, error) {
	var admin model.SysAdmin
	err := Db.Table("sys_admin sa").
		Select("sa.*").
		Joins("JOIN sys_admin_role sar ON sar.admin_id = sa.id").
		Joins("JOIN sys_role sr ON sr.id = sar.role_id").
		Where("sa.status = ? AND sr.status = ? AND sr.role_key = ?", 1, 1, "admin").
		Order("sa.id ASC").
		Take(&admin).Error
	return admin, err
}

// IsSystemAdminUser checks the active role assignment instead of relying on a
// fixed account ID or username.
func IsSystemAdminUser(userID uint) (bool, error) {
	if userID == 0 {
		return false, nil
	}
	var count int64
	err := Db.Table("sys_admin_role sar").
		Joins("JOIN sys_role sr ON sr.id = sar.role_id").
		Where("sar.admin_id = ? AND sr.status = ? AND sr.role_key = ?", userID, 1, "admin").
		Count(&count).Error
	return count > 0, err
}

// GetUserBusinessScope returns the service labels assigned to a non-admin
// account. System administrators bypass this scope in the caller.
func GetUserBusinessScope(userID uint) ([]string, error) {
	if userID == 0 {
		return nil, errors.New("用户ID不能为空")
	}
	var values []string
	err := Db.Table("user_business_scope").
		Where("user_id = ?", userID).
		Order("service_value ASC").
		Pluck("service_value", &values).Error
	return values, err
}

// 新增用户
func CreateSysAdmin(dto model.AddSysAdminDto) error {
	sysAdminByUsername := GetSysAdminByUsername(dto.Username)
	if sysAdminByUsername.ID > 0 {
		return ErrSysAdminUsernameExists
	}
	sysAdmin := model.SysAdmin{
		PostId:     dto.PostId,
		DeptId:     dto.DeptId,
		Username:   dto.Username,
		Nickname:   dto.Nickname,
		Password:   util.EncryptionMd5(dto.Password),
		Email:      strings.TrimSpace(dto.Email),
		Icon:       strings.TrimSpace(dto.Icon),
		Note:       dto.Note,
		Status:     dto.Status,
		CreateTime: util.HTime{Time: time.Now()},
	}
	return Db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&sysAdmin).Error; err != nil {
			if isSysAdminUsernameDuplicateError(err) {
				return ErrSysAdminUsernameExists
			}
			return err
		}
		adminRole := model.SysAdminRole{AdminId: sysAdmin.ID, RoleId: dto.RoleId}
		return tx.Create(&adminRole).Error
	})
}

func isSysAdminUsernameDuplicateError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	var mysqlErr *mysql.MySQLError
	if errors.As(err, &mysqlErr) && mysqlErr.Number == 1062 {
		message = strings.ToLower(mysqlErr.Message)
	}
	return strings.Contains(message, "duplicate") && strings.Contains(message, "username")
}

// 根据id查询用户详情
func GetSysAdminInfo(Id int) (sysAdminInfo model.SysAdminInfo) {
	Db.Table("sys_admin").
		Select("sys_admin.*, sys_admin_role.role_id").
		Joins("LEFT JOIN sys_admin_role ON sys_admin.id = sys_admin_role.admin_id").
		Joins("LEFT JOIN sys_role ON sys_admin_role.role_id = sys_role.id").
		First(&sysAdminInfo, Id)
	return sysAdminInfo
}

// 修改用户
func UpdateSysAdmin(dto model.UpdateSysAdminDto) (*model.SysAdminInfo, error) {
	var existing model.SysAdmin
	if err := Db.First(&existing, dto.Id).Error; err != nil {
		return nil, err
	}
	updates := map[string]interface{}{
		"post_id":  dto.PostId,
		"dept_id":  dto.DeptId,
		"username": strings.TrimSpace(dto.Username),
		"nickname": strings.TrimSpace(dto.Nickname),
		"status":   dto.Status,
		"note":     dto.Note,
	}
	if dto.Icon != "" {
		updates["icon"] = dto.Icon
	}
	if err := Db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.SysAdmin{}).Where("id = ?", dto.Id).Updates(updates).Error; err != nil {
			if isSysAdminUsernameDuplicateError(err) {
				return ErrSysAdminUsernameExists
			}
			return err
		}
		if err := tx.Where("admin_id = ?", dto.Id).Delete(&model.SysAdminRole{}).Error; err != nil {
			return err
		}
		return tx.Create(&model.SysAdminRole{AdminId: dto.Id, RoleId: dto.RoleId}).Error
	}); err != nil {
		return nil, err
	}
	info := GetSysAdminInfo(int(dto.Id))
	return &info, nil
}

// 根据id删除用户
func DeleteSysAdminById(dto model.SysAdminIdDto) {
	Db.First(&model.SysAdmin{}, dto.Id)
	Db.Delete(&model.SysAdmin{}, dto.Id)
	Db.Where("admin_id = ?", dto.Id).Delete(&model.SysAdminRole{})
}

// 修改用户状态
func UpdateSysAdminStatus(dto model.UpdateSysAdminStatusDto) {
	var sysAdmin model.SysAdmin
	Db.First(&sysAdmin, dto.Id)
	sysAdmin.Status = dto.Status
	Db.Save(&sysAdmin)
}

// 重置密码
func ResetSysAdminPassword(dto model.ResetSysAdminPasswordDto) {
	var sysAdmin model.SysAdmin
	Db.First(&sysAdmin, dto.Id)
	sysAdmin.Password = util.EncryptionMd5(dto.Password) // 密码加密
	Db.Save(&sysAdmin)
}

// 分页查询用户列表
func GetSysAdminList(PageSize, PageNum int, Username, Status, BeginTime, EndTime string) (sysAdminVo []model.SysAdminVo, count int64) {
	curDb := Db.Table("sys_admin").
		Select("sys_admin.*, sys_admin_role.role_id, sys_post.post_name, sys_role.role_name, sys_dept.dept_name").
		Joins("LEFT JOIN sys_post ON sys_admin.post_id = sys_post.id").
		Joins("LEFT JOIN sys_admin_role ON sys_admin.id = sys_admin_role.admin_id").
		Joins("LEFT JOIN sys_role ON sys_role.id = sys_admin_role.role_id").
		Joins("LEFT JOIN sys_dept ON sys_dept.id = sys_admin.dept_id")
	if Username != "" {
		curDb = curDb.Where("sys_admin.username LIKE ?", "%"+Username+"%")
	}
	if Status != "" {
		curDb = curDb.Where("sys_admin.status = ?", Status)
	}
	if BeginTime != "" && EndTime != "" {
		curDb = curDb.Where("sys_admin.create_time BETWEEN ? AND ?", BeginTime, EndTime)
	}
	curDb.Count(&count)
	curDb.Limit(PageSize).Offset((PageNum - 1) * PageSize).Order("sys_admin.id ASC").Find(&sysAdminVo)
	return sysAdminVo, count
}

// 修改个人信息
func UpdatePersonal(dto model.UpdatePersonalDto) (sysAdmin model.SysAdmin) {
	Db.First(&sysAdmin, dto.Id)
	if dto.Icon != "" {
		sysAdmin.Icon = dto.Icon
	}
	if dto.Username != "" {
		sysAdmin.Username = dto.Username
	}
	if dto.Nickname != "" {
		sysAdmin.Nickname = dto.Nickname
	}
	if dto.Email != "" {
		sysAdmin.Email = dto.Email
	}
	Db.Save(&sysAdmin)
	return sysAdmin
}

// 修改个人密码
func UpdatePersonalPassword(dto model.UpdatePersonalPasswordDto) (sysAdmin model.SysAdmin) {
	Db.First(&sysAdmin, dto.Id)
	sysAdmin.Password = dto.NewPassword
	Db.Save(&sysAdmin)
	return sysAdmin
}

// ========== LDAP相关DAO方法 ==========

// SysAdminDao LDAP辅助DAO结构
type SysAdminDao struct{}

// GetSysAdminByUsername 根据用户名获取用户
func (dao SysAdminDao) GetSysAdminByUsername(username string) model.SysAdmin {
	return GetSysAdminByUsername(username)
}

// CreateSysAdminForLdap 为LDAP用户创建系统账号
func (dao SysAdminDao) CreateSysAdminForLdap(admin *model.SysAdmin) error {
	admin.CreateTime = util.HTime{Time: time.Now()}
	return Db.Create(admin).Error
}

// UpdateSysAdminLdapInfo 更新LDAP用户信息
func (dao SysAdminDao) UpdateSysAdminLdapInfo(admin model.SysAdmin) error {
	return Db.Model(&model.SysAdmin{}).Where("id = ?", admin.ID).Updates(map[string]interface{}{
		"nickname": admin.Nickname,
		"email":    admin.Email,
	}).Error
}

// AssignRoleToUser 为用户分配角色
func (dao SysAdminDao) AssignRoleToUser(userID, roleID uint) error {
	// 先检查是否已存在
	var count int64
	Db.Model(&model.SysAdminRole{}).Where("admin_id = ? AND role_id = ?", userID, roleID).Count(&count)
	if count > 0 {
		return nil // 已存在，跳过
	}

	adminRole := model.SysAdminRole{
		AdminId: userID,
		RoleId:  roleID,
	}
	return Db.Create(&adminRole).Error
}

// SysRoleDao 角色DAO结构
type SysRoleDao struct{}

// GetRoleByName 根据角色名称获取角色
func (dao SysRoleDao) GetRoleByName(roleName string) model.SysRole {
	var role model.SysRole
	Db.Where("role_name = ?", roleName).Or("role_name = ?", strings.ToLower(roleName)).First(&role)
	return role
}

// 获取系统用户总数
func GetTotalUserCount() int64 {
	var count int64
	Db.Model(&model.SysAdmin{}).Count(&count)
	return count
}
