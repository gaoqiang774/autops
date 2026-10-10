package model

import (
	"strings"

	"dodevops-api/common/util"
)

// SysNavCategory 导航分类
type SysNavCategory struct {
	ID          uint       `gorm:"column:id;comment:'主键';primaryKey;NOT NULL" json:"id"`
	Name        string     `gorm:"column:name;type:varchar(64);comment:'分类名称';NOT NULL" json:"name"`
	Code        string     `gorm:"column:code;type:varchar(64);uniqueIndex;comment:'分类编码';NOT NULL" json:"code"`
	Icon        string     `gorm:"column:icon;type:varchar(100);comment:'分类图标'" json:"icon"`
	Description string     `gorm:"column:description;type:varchar(500);comment:'分类描述'" json:"description"`
	Sort        uint       `gorm:"column:sort;comment:'排序'" json:"sort"`
	Status      int        `gorm:"column:status;default:1;comment:'状态：1->启用,2->禁用';NOT NULL" json:"status"`
	CreateBy    uint       `gorm:"column:create_by;comment:'创建人ID'" json:"createBy"`
	CreateTime  util.HTime `gorm:"column:create_time;comment:'创建时间';NOT NULL" json:"createTime"`
	UpdateTime  util.HTime `gorm:"column:update_time;comment:'更新时间'" json:"updateTime"`
}

func (SysNavCategory) TableName() string {
	return "sys_nav_category"
}

// SysNavItem 导航链接
type SysNavItem struct {
	ID             uint       `gorm:"column:id;comment:'主键';primaryKey;NOT NULL" json:"id"`
	CategoryID     uint       `gorm:"column:category_id;index;comment:'分类ID';NOT NULL" json:"categoryId"`
	Name           string     `gorm:"column:name;type:varchar(100);comment:'导航名称';NOT NULL" json:"name"`
	URL            string     `gorm:"column:url;type:varchar(500);comment:'导航地址';NOT NULL" json:"url"`
	Icon           string     `gorm:"column:icon;type:varchar(255);comment:'图标地址或图标名'" json:"icon"`
	Description    string     `gorm:"column:description;type:varchar(500);comment:'说明'" json:"description"`
	Environment    string     `gorm:"column:environment;type:varchar(32);comment:'环境：dev/test/pre/prod/none'" json:"environment"`
	Tags           string     `gorm:"column:tags;type:varchar(255);comment:'标签，逗号分隔'" json:"tags"`
	Scope          string     `gorm:"column:scope;type:varchar(16);index;comment:'可见范围：public/private/acl';NOT NULL" json:"scope"`
	OwnerID        uint       `gorm:"column:owner_id;index;comment:'所属用户ID'" json:"ownerId"`
	OwnerName      string     `gorm:"column:owner_name;type:varchar(64);comment:'所属用户名'" json:"ownerName"`
	VisibleUserIDs string     `gorm:"column:visible_user_ids;type:text;comment:'可见用户ID，逗号分隔'" json:"visibleUserIds"`
	VisibleRoleIDs string     `gorm:"column:visible_role_ids;type:text;comment:'可见角色ID，逗号分隔'" json:"visibleRoleIds"`
	VisibleDeptIDs string     `gorm:"column:visible_dept_ids;type:text;comment:'可见部门ID，逗号分隔'" json:"visibleDeptIds"`
	SSOEnabled     bool       `gorm:"column:sso_enabled;comment:'是否支持单点登录'" json:"ssoEnabled"`
	SSOMode        string     `gorm:"column:sso_mode;type:varchar(32);comment:'SSO模式：oidc/cas/saml/ticket/proxy/none'" json:"ssoMode"`
	OpenMode       string     `gorm:"column:open_mode;type:varchar(16);comment:'打开方式：blank/self';default:'blank'" json:"openMode"`
	Sort           uint       `gorm:"column:sort;comment:'排序'" json:"sort"`
	Status         int        `gorm:"column:status;default:1;comment:'状态：1->启用,2->禁用';NOT NULL" json:"status"`
	CreateTime     util.HTime `gorm:"column:create_time;comment:'创建时间';NOT NULL" json:"createTime"`
	UpdateTime     util.HTime `gorm:"column:update_time;comment:'更新时间'" json:"updateTime"`
}

func (SysNavItem) TableName() string {
	return "sys_nav_item"
}

type SysNavIDDto struct {
	Id uint `json:"id"`
}

type SysNavCategoryDto struct {
	Id          uint   `json:"id"`
	Name        string `json:"name"`
	Code        string `json:"code"`
	Icon        string `json:"icon"`
	Description string `json:"description"`
	Sort        uint   `json:"sort"`
	Status      int    `json:"status"`
}

type SysNavCategoryQuery struct {
	Name   string `form:"name"`
	Status string `form:"status"`
}

type SysNavItemDto struct {
	Id             uint   `json:"id"`
	CategoryID     uint   `json:"categoryId"`
	Name           string `json:"name"`
	URL            string `json:"url"`
	Icon           string `json:"icon"`
	Description    string `json:"description"`
	Environment    string `json:"environment"`
	Tags           string `json:"tags"`
	Scope          string `json:"scope"`
	VisibleUserIDs []uint `json:"visibleUserIds"`
	VisibleRoleIDs []uint `json:"visibleRoleIds"`
	VisibleDeptIDs []uint `json:"visibleDeptIds"`
	SSOEnabled     bool   `json:"ssoEnabled"`
	SSOMode        string `json:"ssoMode"`
	OpenMode       string `json:"openMode"`
	Sort           uint   `json:"sort"`
	Status         int    `json:"status"`
}

type SysNavItemQuery struct {
	PageNum    int    `form:"pageNum"`
	PageSize   int    `form:"pageSize"`
	CategoryID uint   `form:"categoryId"`
	Name       string `form:"name"`
	Scope      string `form:"scope"`
	Status     string `form:"status"`
	OwnerID    uint   `form:"ownerId"`
}

type SysNavItemVo struct {
	ID              uint       `json:"id"`
	CategoryID      uint       `json:"categoryId"`
	CategoryName    string     `json:"categoryName"`
	Name            string     `json:"name"`
	URL             string     `json:"url"`
	Icon            string     `json:"icon"`
	Description     string     `json:"description"`
	Environment     string     `json:"environment"`
	Tags            string     `json:"tags"`
	Scope           string     `json:"scope"`
	OwnerID         uint       `json:"ownerId"`
	OwnerName       string     `json:"ownerName"`
	VisibleUserIDs  []uint     `json:"visibleUserIds" gorm:"-"`
	VisibleRoleIDs  []uint     `json:"visibleRoleIds" gorm:"-"`
	VisibleDeptIDs  []uint     `json:"visibleDeptIds" gorm:"-"`
	VisibleUserText string     `json:"-" gorm:"column:visible_user_ids"`
	VisibleRoleText string     `json:"-" gorm:"column:visible_role_ids"`
	VisibleDeptText string     `json:"-" gorm:"column:visible_dept_ids"`
	SSOEnabled      bool       `json:"ssoEnabled"`
	SSOMode         string     `json:"ssoMode"`
	OpenMode        string     `json:"openMode"`
	Sort            uint       `json:"sort"`
	Status          int        `json:"status"`
	CreateTime      util.HTime `json:"createTime"`
	UpdateTime      util.HTime `json:"updateTime"`
}

type SysNavVisibleGroup struct {
	Category SysNavCategory `json:"category"`
	Items    []SysNavItemVo `json:"items"`
}

func NormalizeNavURL(rawURL string) string {
	url := strings.TrimSpace(rawURL)
	if url == "" {
		return ""
	}
	lowerURL := strings.ToLower(url)
	if strings.HasPrefix(lowerURL, "http://") ||
		strings.HasPrefix(lowerURL, "https://") ||
		strings.HasPrefix(url, "/") {
		return url
	}
	return "http://" + url
}
