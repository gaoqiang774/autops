package service

import (
	"dodevops-api/api/cmdb/dao"
	"dodevops-api/api/cmdb/model"
	systemModel "dodevops-api/api/system/model"
	"dodevops-api/common/constant"
	"dodevops-api/common/result"
	"dodevops-api/common/util"
	"time"

	"github.com/gin-gonic/gin"
)

// 接口名称修改为 CmdbGroupServiceInterface 避免冲突
type CmdbGroupServiceInterface interface {
	CreateCmdbGroup(c *gin.Context, group model.CmdbGroup) // 创建分组
	GetAllCmdbGroups(c *gin.Context)                       // 获取所有分组
	GetAllCmdbGroupsWithHosts(c *gin.Context)              // 获取所有分组及关联主机
	UpdateCmdbGroup(c *gin.Context, group model.CmdbGroup) // 更新分组
	DeleteCmdbGroup(c *gin.Context, id uint)               // 删除分组
	GetCmdbGroupByName(c *gin.Context, name string)        // 根据名称查询分组
}

type CmdbGroupServiceImpl struct{}

func getCmdbGroupCurrentAdmin(c *gin.Context) (*systemModel.JwtAdmin, bool) {
	userObj, exists := c.Get(constant.ContextKeyUserObj)
	if !exists {
		result.FailedWithCode(c, constant.INVALID_PARAMS, "not logged in")
		return nil, false
	}

	admin, ok := userObj.(*systemModel.JwtAdmin)
	if !ok || admin == nil {
		result.FailedWithCode(c, constant.INVALID_PARAMS, "get user info failed")
		return nil, false
	}
	return admin, true
}

func isCmdbGroupAdmin(admin *systemModel.JwtAdmin) bool {
	return admin != nil && (admin.ID == 1 || admin.Username == "admin")
}

func getVisibleCmdbGroupHosts(c *gin.Context, hostDao dao.CmdbHostDao) ([]model.CmdbHost, bool, bool) {
	admin, ok := getCmdbGroupCurrentAdmin(c)
	if !ok {
		return nil, false, false
	}

	if isCmdbGroupAdmin(admin) {
		return hostDao.GetCmdbHostList(), true, true
	}

	authDao := dao.NewBastionHostAuthorizationDao()
	authorizedHostIds, err := authDao.GetUserAuthorizedHostIds(admin.ID)
	if err != nil {
		result.FailedWithCode(c, constant.INVALID_PARAMS, "get authorization info failed")
		return nil, false, false
	}
	if len(authorizedHostIds) == 0 {
		return []model.CmdbHost{}, false, true
	}

	hosts, err := hostDao.GetCmdbHostsByIds(authorizedHostIds)
	if err != nil {
		result.FailedWithCode(c, constant.INVALID_PARAMS, "get host info failed")
		return nil, false, false
	}
	return hosts, false, true
}

func pruneCmdbGroupTreeByHostCount(groups []model.CmdbGroup) []model.CmdbGroup {
	pruned := make([]model.CmdbGroup, 0, len(groups))
	for _, group := range groups {
		group.Children = pruneCmdbGroupTreeByHostCount(group.Children)
		if group.HostCount > 0 {
			pruned = append(pruned, group)
		}
	}
	return pruned
}

func pruneCmdbGroupHostTreeByHostCount(groups []model.CmdbGroupHostDto) []model.CmdbGroupHostDto {
	pruned := make([]model.CmdbGroupHostDto, 0, len(groups))
	for _, group := range groups {
		group.Children = pruneCmdbGroupHostTreeByHostCount(group.Children)
		if group.HostCount > 0 {
			pruned = append(pruned, group)
		}
	}
	return pruned
}

// 新增分组
func (s CmdbGroupServiceImpl) CreateCmdbGroup(c *gin.Context, group model.CmdbGroup) {
	dao := dao.NewCmdbGroupDao()
	// 修改为精确匹配：检查同一父分组下是否存在同名分组
	if dao.CheckNameExists(group.Name, group.ParentID) {
		result.FailedWithCode(c, constant.GROUP_EXIST, "分组已存在无法创建")
		return
	}
	now := util.HTime{Time: time.Now()}
	group.CreateTime = now
	group.UpdateTime = now
	err := dao.CreateCmdbGroup(&group)
	if err != nil {
		result.Failed(c, constant.GROUP_EXIST, "创建分组失败")
		return
	}
	result.Success(c, true)
}

// 查询所有分组并返回树形结构
func (s CmdbGroupServiceImpl) GetAllCmdbGroups(c *gin.Context) {
	groupDao := dao.NewCmdbGroupDao()
	hostDao := dao.NewCmdbHostDao()

	// 获取所有分组
	groups := groupDao.GetCmdbGroupList()
	hosts, adminView, ok := getVisibleCmdbGroupHosts(c, hostDao)
	if !ok {
		return
	}

	tree := model.BuildCmdbGroupTreeWithHostCount(groups, hosts)
	if !adminView {
		tree = pruneCmdbGroupTreeByHostCount(tree)
	}
	result.Success(c, tree)
}

// 查询所有分组及关联主机(树形结构)
func (s CmdbGroupServiceImpl) GetAllCmdbGroupsWithHosts(c *gin.Context) {
	groupDao := dao.NewCmdbGroupDao()
	hostDao := dao.NewCmdbHostDao()

	// 获取所有分组
	groups := groupDao.GetCmdbGroupList()

	// 获取主机列表（根据用户权限过滤）
	hosts, adminView, ok := getVisibleCmdbGroupHosts(c, hostDao)
	if !ok {
		return
	}

	// 转换为VO
	var hostVos []model.CmdbHostVo
	for _, host := range hosts {
		hostVos = append(hostVos, model.CmdbHostVo{
			ID:          host.ID,
			HostName:    host.HostName,
			Name:        host.Name,
			GroupID:     host.GroupID,
			PrivateIP:   host.PrivateIP,
			PublicIP:    host.PublicIP,
			SSHIP:       host.SSHIP,
			SSHName:     host.SSHName,
			SSHKeyID:    host.SSHKeyID,
			SSHPort:     host.SSHPort,
			Remark:      host.Remark,
			Vendor:      host.Vendor,
			HostType:    host.HostType,
			Region:      host.Region,
			InstanceID:  host.InstanceID,
			OS:          host.OS,
			Status:      host.Status,
			CPU:         host.CPU,
			Memory:      host.Memory,
			Disk:        host.Disk,
			BillingType: host.BillingType,
			CreateTime:  host.CreateTime,
			ExpireTime:  host.ExpireTime,
			UpdateTime:  host.UpdateTime,
		})
	}

	tree := model.BuildCmdbGroupHostTree(groups, hostVos)
	if !adminView {
		tree = pruneCmdbGroupHostTreeByHostCount(tree)
	}
	result.Success(c, tree)
}

// 获取云厂商名称
func getVendorName(vendor int) string {
	switch vendor {
	case 1:
		return "自建"
	case 2:
		return "阿里云"
	case 3:
		return "腾讯云"
	case 4:
		return "鐧惧害浜?"
	case 5:
		return "鍗庝负浜?"
	case 6:
		return "AWS"
	case 7:
		return "澶╃考浜?"
	default:
		return "未知"
	}
}

// 更新分组
func (s CmdbGroupServiceImpl) UpdateCmdbGroup(c *gin.Context, group model.CmdbGroup) {
	dao := dao.NewCmdbGroupDao()
	group.UpdateTime = util.HTime{Time: time.Now()}
	err := dao.UpdateCmdbGroup(group.ID, &group)
	if err != nil {
		result.FailedWithCode(c, constant.GROUP_EXIST, err.Error())
		return
	}
	result.Success(c, true)
}

// 删除分组（级联删除所有子分组）
func (s CmdbGroupServiceImpl) DeleteCmdbGroup(c *gin.Context, id uint) {
	groupDao := dao.NewCmdbGroupDao()
	defaultGroup, err := groupDao.GetDefaultCmdbGroup()
	if err == nil && id == defaultGroup.ID {
		result.Failed(c, constant.GROUP_EXIST, "默认业务组无法删除")
		return
	}

	// 执行删除（DAO层会检查分组及子分组是否有主机）
	err = groupDao.DeleteCmdbGroup(id)
	if err != nil {
		result.Failed(c, constant.GROUP_EXIST, err.Error())
		return
	}
	result.Success(c, true)
}

// 根据名称查询分组
func (s CmdbGroupServiceImpl) GetCmdbGroupByName(c *gin.Context, name string) {
	dao := dao.NewCmdbGroupDao()
	group, err := dao.GetCmdbGroupByName(name)
	if err != nil {
		result.Failed(c, constant.GROUP_EXIST, "查询分组失败")
		return
	}
	result.Success(c, group)
}

// 全局服务调用方法
func GetCmdbGroupService() CmdbGroupServiceInterface {
	return &CmdbGroupServiceImpl{}
}
