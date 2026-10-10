// 资产分组
// model/cmdb_group.go
package model

import (
	"dodevops-api/common/util"
	"sort"
	"time"
)

const DefaultCmdbGroupName = "默认业务组"

type CmdbGroup struct {
	ID         uint        `gorm:"column:id;comment:'主键';primaryKey;NOT NULL" json:"id"`                 // 主键ID
	ParentID   uint        `gorm:"column:parent_id;default:0;comment:'父级分组ID';NOT NULL" json:"parentId"` // 父级分组ID（0 表示根分组）
	Name       string      `gorm:"column:name;varchar(50);comment:'分组名称';NOT NULL" json:"name"`          // 分组名称
	CreateTime util.HTime  `gorm:"column:create_time;comment:'创建时间';NOT NULL" json:"createTime"`         // 创建时间
	UpdateTime util.HTime  `gorm:"column:update_time;comment:'更新时间'" json:"updateTime"`                  // 更新时间
	Children   []CmdbGroup `json:"children" gorm:"-"`                                                    // 子分组（虚拟字段，用于树形展示）
	Hosts      []CmdbHost  `gorm:"foreignKey:GroupID" json:"hosts"`                                      // 关联的主机列表
	HostCount  int         `gorm:"-" json:"hostCount"`                                                   // 主机数量（虚拟字段，包含所有子分组的主机数量）
}

func (CmdbGroup) TableName() string {
	return "cmdb_group"
}

// Id参数
type CmdbGroupIdDto struct {
	Id uint `json:"id"` // ID
}

// BuildTree 构建树形结构
func BuildCmdbGroupTree(groups []CmdbGroup) []CmdbGroup {
	groupMap := make(map[uint]CmdbGroup)
	for _, group := range groups {
		groupMap[group.ID] = group
	}

	var tree []CmdbGroup
	for i := range groups {
		if groups[i].ParentID == 0 {
			tree = append(tree, buildSubTree(groups[i], groupMap))
		}
	}
	sortCmdbGroupsRecursive(tree)
	return tree
}

// 递归构建子树
func buildSubTree(group CmdbGroup, groupMap map[uint]CmdbGroup) CmdbGroup {
	for _, child := range groupMap {
		if child.ParentID == group.ID {
			group.Children = append(group.Children, buildSubTree(child, groupMap))
		}
	}
	return group
}

// BuildCmdbGroupTreeWithHostCount 构建包含主机数量的树形结构
func BuildCmdbGroupTreeWithHostCount(groups []CmdbGroup, hosts []CmdbHost) []CmdbGroup {
	// 创建分组映射
	groupMap := make(map[uint]CmdbGroup)
	for _, group := range groups {
		groupMap[group.ID] = group
	}

	// 统计每个分组的直接主机数量
	directHostCount := make(map[uint]int)
	for _, host := range hosts {
		directHostCount[host.GroupID]++
	}

	var tree []CmdbGroup
	for i := range groups {
		if groups[i].ParentID == 0 {
			tree = append(tree, buildSubTreeWithHostCount(groups[i], groupMap, directHostCount))
		}
	}
	sortCmdbGroupsRecursive(tree)
	return tree
}

// 递归构建包含主机数量的子树
func buildSubTreeWithHostCount(group CmdbGroup, groupMap map[uint]CmdbGroup, directHostCount map[uint]int) CmdbGroup {
	// 获取当前分组的直接主机数量
	group.HostCount = directHostCount[group.ID]

	// 递归处理子分组并累加主机数量
	for _, child := range groupMap {
		if child.ParentID == group.ID {
			childWithCount := buildSubTreeWithHostCount(child, groupMap, directHostCount)
			group.Children = append(group.Children, childWithCount)
			// 累加子分组的主机数量到父分组
			group.HostCount += childWithCount.HostCount
		}
	}
	return group
}

func sortCmdbGroupsRecursive(groups []CmdbGroup) {
	sort.SliceStable(groups, func(i, j int) bool {
		return cmdbGroupLess(groups[i], groups[j])
	})

	for i := range groups {
		sortCmdbGroupsRecursive(groups[i].Children)
	}
}

func cmdbGroupLess(left, right CmdbGroup) bool {
	leftSortTime := cmdbGroupSortTime(left)
	rightSortTime := cmdbGroupSortTime(right)
	if !leftSortTime.Equal(rightSortTime) {
		return leftSortTime.After(rightSortTime)
	}

	if !left.CreateTime.Time.Equal(right.CreateTime.Time) {
		return left.CreateTime.Time.After(right.CreateTime.Time)
	}

	return left.ID > right.ID
}

func cmdbGroupSortTime(group CmdbGroup) time.Time {
	if !group.UpdateTime.Time.IsZero() {
		return group.UpdateTime.Time
	}
	return group.CreateTime.Time
}
