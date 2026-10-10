package model

import "dodevops-api/common/util"

const (
	AssetAuthorizationSubjectUser  = "user"
	AssetAuthorizationSubjectGroup = "group"

	AssetTypeHost       = "host"
	AssetTypeK8SCluster = "k8s_cluster"
	AssetTypeLogProject = "log_project"

	AssetScopeAll       = ""
	AssetScopeNamespace = "namespace"
	AssetScopeLogStore  = "log_store"
)

type BastionAssetAuthorization struct {
	ID            uint        `gorm:"column:id;comment:'主键';primaryKey;NOT NULL" json:"id"`
	SubjectType   string      `gorm:"column:subject_type;varchar(10);comment:'授权对象类型:user/group';NOT NULL;index:idx_asset_subject_resource,priority:1" json:"subjectType"`
	SubjectID     uint        `gorm:"column:subject_id;comment:'授权对象ID(用户ID或用户组ID)';NOT NULL;index:idx_asset_subject_resource,priority:2" json:"subjectId"`
	AssetType     string      `gorm:"column:asset_type;varchar(32);comment:'资产类型:host/k8s_cluster等';NOT NULL;index:idx_asset_subject_resource,priority:3" json:"assetType"`
	AssetID       uint        `gorm:"column:asset_id;comment:'资产ID';NOT NULL;index:idx_asset_subject_resource,priority:4" json:"assetId"`
	ScopeType     string      `gorm:"column:scope_type;varchar(32);comment:'资产子范围类型，空值表示整个资产';NOT NULL;default:'';index:idx_asset_subject_resource,priority:5" json:"scopeType"`
	ScopeValue    string      `gorm:"column:scope_value;varchar(255);comment:'资产子范围值，如K8S命名空间';NOT NULL;default:'';index:idx_asset_subject_resource,priority:6" json:"scopeValue"`
	EffectiveTime *util.HTime `gorm:"column:effective_time;comment:'生效时间'" json:"effectiveTime"`
	ExpireTime    *util.HTime `gorm:"column:expire_time;comment:'失效时间'" json:"expireTime"`
	CreateTime    util.HTime  `gorm:"column:create_time;comment:'创建时间';NOT NULL" json:"createTime"`
	UpdateTime    *util.HTime `gorm:"column:update_time;comment:'更新时间'" json:"updateTime"`
}

func (BastionAssetAuthorization) TableName() string {
	return "bastion_asset_authorization"
}

type AssetAuthorizationResourceDto struct {
	AssetID       uint        `json:"assetId" example:"1"`
	ScopeType     string      `json:"scopeType" example:"namespace"`
	ScopeValues   []string    `json:"scopeValues" example:"default,prod"`
	AllScopes     bool        `json:"allScopes" example:"false"`
	EffectiveTime *util.HTime `json:"effectiveTime" swaggertype:"string" example:"2026-04-24 09:00:00"`
	ExpireTime    *util.HTime `json:"expireTime" swaggertype:"string" example:"2026-04-30 23:59:59"`
}

type AuthorizeAssetsToUserDto struct {
	UserID         uint                            `validate:"required" json:"userId" example:"1001"`
	AssetType      string                          `validate:"required" json:"assetType" example:"k8s_cluster"`
	AssetIDs       []uint                          `json:"assetIds" example:"1,2"`
	ScopeType      string                          `json:"scopeType" example:"namespace"`
	ScopeValues    []string                        `json:"scopeValues" example:"default,prod"`
	AllScopes      bool                            `json:"allScopes" example:"false"`
	EffectiveTime  *util.HTime                     `json:"effectiveTime" swaggertype:"string" example:"2026-04-24 09:00:00"`
	ExpireTime     *util.HTime                     `json:"expireTime" swaggertype:"string" example:"2026-04-30 23:59:59"`
	Authorizations []AssetAuthorizationResourceDto `json:"authorizations"`
}

type AuthorizeAssetsToGroupDto struct {
	GroupID        uint                            `validate:"required" json:"groupId" example:"2001"`
	AssetType      string                          `validate:"required" json:"assetType" example:"k8s_cluster"`
	AssetIDs       []uint                          `json:"assetIds" example:"1,2"`
	ScopeType      string                          `json:"scopeType" example:"namespace"`
	ScopeValues    []string                        `json:"scopeValues" example:"default,prod"`
	AllScopes      bool                            `json:"allScopes" example:"false"`
	EffectiveTime  *util.HTime                     `json:"effectiveTime" swaggertype:"string" example:"2026-04-24 09:00:00"`
	ExpireTime     *util.HTime                     `json:"expireTime" swaggertype:"string" example:"2026-04-30 23:59:59"`
	Authorizations []AssetAuthorizationResourceDto `json:"authorizations"`
}

type RevokeAssetsFromUserDto struct {
	UserID      uint     `validate:"required" json:"userId" example:"1001"`
	AssetType   string   `validate:"required" json:"assetType" example:"k8s_cluster"`
	AssetID     uint     `validate:"required" json:"assetId" example:"1"`
	ScopeType   string   `json:"scopeType" example:"namespace"`
	ScopeValues []string `json:"scopeValues" example:"default,prod"`
	AllScopes   bool     `json:"allScopes" example:"false"`
}

type RevokeAssetsFromGroupDto struct {
	GroupID     uint     `validate:"required" json:"groupId" example:"2001"`
	AssetType   string   `validate:"required" json:"assetType" example:"k8s_cluster"`
	AssetID     uint     `validate:"required" json:"assetId" example:"1"`
	ScopeType   string   `json:"scopeType" example:"namespace"`
	ScopeValues []string `json:"scopeValues" example:"default,prod"`
	AllScopes   bool     `json:"allScopes" example:"false"`
}

type GetAuthorizedAssetsDto struct {
	SubjectType    string `form:"subjectType" binding:"required,oneof=user group"`
	SubjectID      uint   `form:"subjectId" binding:"required"`
	AssetType      string `form:"assetType"`
	AssetID        uint   `form:"assetId"`
	IncludeExpired bool   `form:"includeExpired"`
	Page           int    `form:"page" binding:"required,min=1"`
	PageSize       int    `form:"pageSize" binding:"required,min=1,max=100"`
}

type CheckUserAssetPermissionDto struct {
	UserID     uint   `form:"userId" validate:"required"`
	AssetType  string `form:"assetType" validate:"required"`
	AssetID    uint   `form:"assetId" validate:"required"`
	ScopeType  string `form:"scopeType"`
	ScopeValue string `form:"scopeValue"`
}

type BastionAssetAuthorizationVo struct {
	ID                  uint        `json:"id"`
	SubjectType         string      `json:"subjectType"`
	SubjectID           uint        `json:"subjectId"`
	AssetType           string      `json:"assetType"`
	AssetID             uint        `json:"assetId"`
	AssetName           string      `json:"assetName"`
	ScopeType           string      `json:"scopeType"`
	ScopeValue          string      `json:"scopeValue"`
	ScopeName           string      `json:"scopeName,omitempty"`
	ScopeDisplayName    string      `json:"scopeDisplayName,omitempty"`
	AllScopes           bool        `json:"allScopes"`
	AuthorizationStatus string      `json:"authorizationStatus"`
	EffectiveTime       *util.HTime `json:"effectiveTime"`
	ExpireTime          *util.HTime `json:"expireTime"`
	CreateTime          util.HTime  `json:"createTime"`
	UpdateTime          *util.HTime `json:"updateTime"`
}
