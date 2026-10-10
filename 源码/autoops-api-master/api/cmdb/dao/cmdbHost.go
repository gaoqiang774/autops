package dao

import (
	"dodevops-api/api/cmdb/model"
	"dodevops-api/common"
	"dodevops-api/common/util"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const (
	hostCachePrefix = "cmdb:host:"
	cacheExpireTime = 5 * time.Minute
)

type CmdbHostDao struct {
	db *gorm.DB
}

func (d *CmdbHostDao) silentDB() *gorm.DB {
	return d.db.Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Silent)})
}

func normalizeHostSort(sortBy, sortOrder string) string {
	fieldMap := map[string]string{
		"id":          "id",
		"updateTime":  "update_time",
		"update_time": "update_time",
		"createTime":  "create_time",
		"create_time": "create_time",
		"hostName":    "host_name",
		"host_name":   "host_name",
		"status":      "status",
	}

	dbField, exists := fieldMap[sortBy]
	if !exists {
		dbField = "id"
	}

	sortOrder = strings.ToLower(strings.TrimSpace(sortOrder))
	if sortOrder != "asc" && sortOrder != "desc" {
		sortOrder = "desc"
	}

	orderClause := dbField + " " + sortOrder
	if dbField == "update_time" {
		orderClause = "COALESCE(update_time, create_time) " + sortOrder + ", id " + sortOrder
	}
	return orderClause
}

func (d *CmdbHostDao) buildHostFilterQuery(filter model.CmdbHostFilter) *gorm.DB {
	query := d.db.Model(&model.CmdbHost{}).Preload("Group")

	if len(filter.AllowedHostIDs) > 0 {
		query = query.Where("id IN ?", filter.AllowedHostIDs)
	}
	if filter.GroupID > 0 {
		groupDao := NewCmdbGroupDao()
		groupIDs := groupDao.GetAllChildGroupIds(filter.GroupID)
		query = query.Where("group_id IN ?", groupIDs)
	}
	if len(filter.Statuses) > 0 {
		if len(filter.Statuses) == 1 {
			query = query.Where("status = ?", filter.Statuses[0])
		} else {
			query = query.Where("status IN ?", filter.Statuses)
		}
	}
	if len(filter.HostTypes) > 0 {
		if len(filter.HostTypes) == 1 {
			query = query.Where("host_type = ?", filter.HostTypes[0])
		} else {
			query = query.Where("host_type IN ?", filter.HostTypes)
		}
	}
	if filter.Keyword != "" {
		like := "%" + strings.TrimSpace(filter.Keyword) + "%"
		query = query.Where(
			"host_name LIKE ? OR name LIKE ? OR private_ip LIKE ? OR public_ip LIKE ? OR ssh_ip LIKE ?",
			like, like, like, like, like,
		)
	}

	return query
}

func NewCmdbHostDao() CmdbHostDao {
	return CmdbHostDao{
		db: common.GetDB(),
	}
}

// 鑾峰彇涓绘満鍒楄〃(鍒嗛〉) - 鏍规嵁涓绘満ID鍒楄〃杩囨护
func (d *CmdbHostDao) GetCmdbHostListWithPageByIds(page, pageSize int, sortBy, sortOrder string, hostIds []uint) ([]model.CmdbHost, int64) {
	var list []model.CmdbHost
	var total int64
	orderClause := normalizeHostSort(sortBy, sortOrder)

	// 浣跨敤棰勫姞杞紾roup淇℃伅骞朵紭鍖栨煡璇紝娣诲姞ID杩囨护
	db := d.db.Preload("Group").Where("id IN ?", hostIds)
	db.Model(&model.CmdbHost{}).Count(&total)
	db.Order(orderClause).Offset((page - 1) * pageSize).Limit(pageSize).Find(&list)

	return list, total
}

// 鑾峰彇涓绘満鍒楄〃(鍒嗛〉)
func (d *CmdbHostDao) GetCmdbHostListWithPage(page, pageSize int, sortBy, sortOrder string) ([]model.CmdbHost, int64) {
	var list []model.CmdbHost
	var total int64
	orderClause := normalizeHostSort(sortBy, sortOrder)

	// 浣跨敤棰勫姞杞紾roup淇℃伅骞朵紭鍖栨煡璇?
	db := d.db.Preload("Group")
	db.Model(&model.CmdbHost{}).Count(&total)
	db.Order(orderClause).Offset((page - 1) * pageSize).Limit(pageSize).Find(&list)

	return list, total
}

// 鑾峰彇涓绘満鍒楄〃
func (d *CmdbHostDao) GetCmdbHostList() []model.CmdbHost {
	var list []model.CmdbHost
	d.db.Preload("Group").Find(&list)
	return list
}

// 鏍规嵁ID鑾峰彇涓绘満
func (d *CmdbHostDao) GetCmdbHostById(id uint) (model.CmdbHost, error) {
	var host model.CmdbHost
	err := d.silentDB().Preload("Group").Where("id = ?", id).First(&host).Error
	return host, err
}

// 鏍规嵁鍚嶇О鑾峰彇涓绘満
func (d *CmdbHostDao) GetCmdbHostByName(name string) (model.CmdbHost, error) {
	var host model.CmdbHost
	err := d.silentDB().Where("host_name = ? OR name = ?", name, name).First(&host).Error
	return host, err
}

// 鏍规嵁IP鑾峰彇鍗曚釜涓绘満
func (d *CmdbHostDao) GetCmdbHostByIP(ip string) (model.CmdbHost, error) {
	var host model.CmdbHost
	err := d.silentDB().Preload("Group").Where("private_ip = ? OR public_ip = ? OR ssh_ip = ?", ip, ip, ip).First(&host).Error
	return host, err
}

func (d *CmdbHostDao) GetCmdbHostByInstanceIdentifier(identifier string) (model.CmdbHost, error) {
	var host model.CmdbHost
	err := d.silentDB().Preload("Group").
		Where("private_ip = ? OR public_ip = ? OR ssh_ip = ? OR name = ? OR host_name = ?", identifier, identifier, identifier, identifier, identifier).
		First(&host).Error
	return host, err
}

func (d *CmdbHostDao) GetCmdbHostsByHostnameIdentifier(identifier string) ([]model.CmdbHost, error) {
	var hosts []model.CmdbHost
	err := d.silentDB().Preload("Group").
		Where("name = ? OR host_name = ?", identifier, identifier).
		Order("id ASC").
		Find(&hosts).Error
	return hosts, err
}

// 妫€鏌ヤ富鏈哄悕绉版槸鍚﹀瓨鍦?
func (d *CmdbHostDao) CheckNameExists(name string) bool {
	var count int64
	d.db.Model(&model.CmdbHost{}).Where("host_name = ?", name).Count(&count)
	return count > 0
}

// 鍒涘缓涓绘満
func (d *CmdbHostDao) CreateCmdbHost(host *model.CmdbHost) error {
	now := util.HTime{Time: time.Now()}
	if host.CreateTime.Time.IsZero() {
		host.CreateTime = now
	}
	if host.UpdateTime.Time.IsZero() {
		host.UpdateTime = now
	}
	return d.db.Create(host).Error
}

// 鎵归噺鍒涘缓涓绘満
func (d *CmdbHostDao) BatchCreateCmdbHosts(hosts []model.CmdbHost) error {
	now := util.HTime{Time: time.Now()}
	for i := range hosts {
		if hosts[i].CreateTime.Time.IsZero() {
			hosts[i].CreateTime = now
		}
		if hosts[i].UpdateTime.Time.IsZero() {
			hosts[i].UpdateTime = now
		}
	}
	return d.db.Create(&hosts).Error
}

// 鏇存柊涓绘満
func (d *CmdbHostDao) UpdateCmdbHost(id uint, host *model.CmdbHost) error {
	if host.UpdateTime.Time.IsZero() {
		host.UpdateTime = util.HTime{Time: time.Now()}
	}
	return d.db.Model(&model.CmdbHost{}).Where("id = ?", id).Updates(host).Error
}

// 鍒犻櫎涓绘満
func (d *CmdbHostDao) DeleteCmdbHost(id uint) error {
	return d.db.Delete(&model.CmdbHost{}, id).Error
}

// 鏍规嵁瀹炰緥ID鑾峰彇涓绘満
func (d *CmdbHostDao) GetCmdbHostByInstanceID(instanceID string) (*model.CmdbHost, error) {
	var host model.CmdbHost
	err := d.db.Where("instance_id = ?", instanceID).First(&host).Error
	if err != nil {
		return nil, err
	}
	return &host, nil
}

// 鏍规嵁SSH IP鑾峰彇涓绘満
func (d *CmdbHostDao) GetCmdbHostBySSHIP(sshIP string) *model.CmdbHost {
	var host model.CmdbHost
	err := d.db.Where("ssh_ip = ?", sshIP).First(&host).Error
	if err != nil {
		return nil
	}
	return &host
}

// 閫掑綊鑾峰彇鎵€鏈夊瓙鍒嗙粍ID锛堝寘鎷嚜韬級
func (d *CmdbHostDao) getAllChildGroupIds(groupId uint) []uint {
	var groupIds []uint
	groupIds = append(groupIds, groupId) // 鍖呭惈鑷韩

	var childGroups []model.CmdbGroup
	d.db.Where("parent_id = ?", groupId).Find(&childGroups)

	for _, child := range childGroups {
		childIds := d.getAllChildGroupIds(child.ID)
		groupIds = append(groupIds, childIds...)
	}

	return groupIds
}

// 鏍规嵁鍒嗙粍ID鑾峰彇涓绘満鍒楄〃锛堝寘鎷墍鏈夊瓙鍒嗙粍鐨勪富鏈猴紝鏀寔鍒嗛〉锛?
func (d *CmdbHostDao) GetCmdbHostsByGroupId(groupId uint) []model.CmdbHost {
	var list []model.CmdbHost
	// 鑾峰彇褰撳墠鍒嗙粍鍙婃墍鏈夊瓙鍒嗙粍鐨処D
	groupIds := d.getAllChildGroupIds(groupId)
	d.db.Preload("Group").Where("group_id IN ?", groupIds).Find(&list)
	return list
}

// 鏍规嵁鍒嗙粍ID鑾峰彇涓绘満鍒楄〃锛堝寘鎷墍鏈夊瓙鍒嗙粍鐨勪富鏈猴紝鏀寔鍒嗛〉+鏉冮檺杩囨护锛?
func (d *CmdbHostDao) GetCmdbHostsByGroupIdWithPageFiltered(groupId uint, page, pageSize int, authorizedHostIds []uint) ([]model.CmdbHost, int64) {
	var list []model.CmdbHost
	var count int64

	// 鑾峰彇褰撳墠鍒嗙粍鍙婃墍鏈夊瓙鍒嗙粍鐨処D
	groupIds := d.getAllChildGroupIds(groupId)

	// 璁＄畻鎬绘暟 - 鍚屾椂杩囨护鍒嗙粍鍜屾潈闄?
	d.db.Model(&model.CmdbHost{}).
		Where("group_id IN ? AND id IN ?", groupIds, authorizedHostIds).
		Count(&count)

	// 鍒嗛〉鏌ヨ
	offset := (page - 1) * pageSize
	d.db.Preload("Group").
		Where("group_id IN ? AND id IN ?", groupIds, authorizedHostIds).
		Offset(offset).
		Limit(pageSize).
		Find(&list)

	return list, count
}

// 鏍规嵁鍒嗙粍ID鑾峰彇涓绘満鍒楄〃锛堝寘鎷墍鏈夊瓙鍒嗙粍鐨勪富鏈猴紝鏀寔鍒嗛〉锛?
func (d *CmdbHostDao) GetCmdbHostsByGroupIdWithPage(groupId uint, page, pageSize int) ([]model.CmdbHost, int64) {
	var list []model.CmdbHost
	var count int64

	// 鑾峰彇褰撳墠鍒嗙粍鍙婃墍鏈夊瓙鍒嗙粍鐨処D
	groupIds := d.getAllChildGroupIds(groupId)

	// 璁＄畻鎬绘暟
	d.db.Model(&model.CmdbHost{}).Where("group_id IN ?", groupIds).Count(&count)

	// 鍒嗛〉鏌ヨ
	offset := (page - 1) * pageSize
	d.db.Preload("Group").Where("group_id IN ?", groupIds).Offset(offset).Limit(pageSize).Find(&list)

	return list, count
}

// 鏍规嵁涓绘満鍚嶇О銆佸唴缃慖P銆佸缃慖P妯＄硦鏌ヨ
func (d *CmdbHostDao) GetCmdbHostsByHostNameLike(name string) []model.CmdbHost {
	return d.SearchCmdbHosts(name)
}

func (d *CmdbHostDao) SearchCmdbHosts(keyword string) []model.CmdbHost {
	var list []model.CmdbHost
	keyword = strings.TrimSpace(keyword)
	if keyword == "" {
		return list
	}

	like := "%" + keyword + "%"
	d.db.Preload("Group").
		Where("host_name LIKE ? OR name LIKE ? OR private_ip LIKE ? OR public_ip LIKE ? OR ssh_ip LIKE ?",
			like, like, like, like, like).
		Find(&list)
	return list
}

// 鏍规嵁IP鏌ヨ(鍖归厤鍐呯綉IP銆佸叕缃慖P鎴朣SH IP)
func (d *CmdbHostDao) GetCmdbHostsByIP(ip string) []model.CmdbHost {
	return d.SearchCmdbHosts(ip)
}

// 鏍规嵁鐘舵€佹煡璇?
func (d *CmdbHostDao) GetCmdbHostsByStatus(status int) []model.CmdbHost {
	var list []model.CmdbHost
	d.db.Preload("Group").Where("status = ?", status).Find(&list)
	return list
}

// 根据主机类型/厂商查询
func (d *CmdbHostDao) GetCmdbHostsByVendor(vendor int) []model.CmdbHost {
	var list []model.CmdbHost
	d.db.Preload("Group").Where("vendor = ?", vendor).Find(&list)
	return list
}

// 根据主机系统类型查询
func (d *CmdbHostDao) GetCmdbHostsByHostType(hostType int) []model.CmdbHost {
	var list []model.CmdbHost
	d.db.Preload("Group").Where("host_type = ?", hostType).Find(&list)
	return list
}

func (d *CmdbHostDao) GetCmdbHostsByHostTypeWithPage(hostType, page, pageSize int, sortBy, sortOrder string) ([]model.CmdbHost, int64) {
	var list []model.CmdbHost
	var total int64
	orderClause := normalizeHostSort(sortBy, sortOrder)

	db := d.db.Preload("Group").Where("host_type = ?", hostType)
	db.Model(&model.CmdbHost{}).Where("host_type = ?", hostType).Count(&total)
	db.Order(orderClause).Offset((page - 1) * pageSize).Limit(pageSize).Find(&list)

	return list, total
}

func (d *CmdbHostDao) QueryCmdbHosts(filter model.CmdbHostFilter) ([]model.CmdbHost, error) {
	var list []model.CmdbHost
	query := d.buildHostFilterQuery(filter)
	err := query.Order(normalizeHostSort(filter.SortBy, filter.SortOrder)).Find(&list).Error
	return list, err
}

func (d *CmdbHostDao) QueryCmdbHostsWithPage(filter model.CmdbHostFilter, page, pageSize int) ([]model.CmdbHost, int64, error) {
	var list []model.CmdbHost
	var total int64

	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}

	query := d.buildHostFilterQuery(filter)
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	err := query.Order(normalizeHostSort(filter.SortBy, filter.SortOrder)).
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&list).Error
	return list, total, err
}

// 鏍规嵁ID鍒楄〃鎵归噺鑾峰彇涓绘満
func (d *CmdbHostDao) GetCmdbHostsByIds(ids []uint) ([]model.CmdbHost, error) {
	var hosts []model.CmdbHost
	err := d.db.Preload("Group").Where("id IN ?", ids).Find(&hosts).Error
	return hosts, err
}

// 缁熻涓绘満鏁伴噺(鎸夊巶鍟嗗垎缁?鍖呮嫭鑷缓)
// 杩斿洖: map[vendor]count, 鍏朵腑vendor: 1->鑷缓, 2->闃块噷浜? 3->鑵捐浜? 4->鐧惧害浜? 5->鍗庝负浜? 6->AWS
func (d *CmdbHostDao) GetCloudHostStatistics() map[int]int64 {
	type VendorCount struct {
		Vendor int   `json:"vendor"`
		Count  int64 `json:"count"`
	}

	var results []VendorCount
	d.db.Model(&model.CmdbHost{}).
		Select("vendor, count(*) as count").
		Group("vendor").
		Find(&results)

	// 杞崲涓簃ap
	stats := make(map[int]int64)
	for _, r := range results {
		stats[r.Vendor] = r.Count
	}

	return stats
}

// 鑾峰彇浜戜富鏈烘€绘暟(鍖呮嫭鑷缓)
func (d *CmdbHostDao) GetCloudHostTotalCount() int64 {
	var count int64
	d.db.Model(&model.CmdbHost{}).Count(&count)
	return count
}

// 鎵归噺鏇存柊涓绘満鍒嗙粍
func (d *CmdbHostDao) BatchUpdateHostGroup(hostIDs []uint, groupID uint) error {
	return d.db.Model(&model.CmdbHost{}).Where("id IN ?", hostIDs).Updates(map[string]interface{}{
		"group_id":    groupID,
		"update_time": time.Now(),
	}).Error
}

// 鎵归噺鍒犻櫎涓绘満
func (d *CmdbHostDao) BatchDeleteHost(hostIDs []uint) error {
	return d.db.Where("id IN ?", hostIDs).Delete(&model.CmdbHost{}).Error
}
