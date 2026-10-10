package model

import (
	"dodevops-api/common/util"
	"strconv"
	"strings"
	"time"
)

type BastionHostAuthorization struct {
	ID            uint        `gorm:"column:id;comment:'主键';primaryKey;NOT NULL" json:"id"`
	SubjectType   string      `gorm:"column:subject_type;varchar(10);comment:'授权对象类型:user/group';NOT NULL;index:idx_subject_host,priority:1" json:"subjectType"`
	SubjectID     uint        `gorm:"column:subject_id;comment:'授权对象ID(用户ID或用户组ID)';NOT NULL;index:idx_subject_host,priority:2" json:"subjectId"`
	HostID        uint        `gorm:"column:host_id;comment:'主机ID';NOT NULL;index:idx_subject_host,priority:3" json:"hostId"`
	AuthIDs       string      `gorm:"column:auth_ids;type:text;comment:'授权登录账号ID列表，逗号分隔'" json:"-"`
	EffectiveTime *util.HTime `gorm:"column:effective_time;comment:'生效时间'" json:"effectiveTime"`
	ExpireTime    *util.HTime `gorm:"column:expire_time;comment:'失效时间'" json:"expireTime"`
	CreateTime    util.HTime  `gorm:"column:create_time;comment:'创建时间';NOT NULL" json:"createTime"`
	UpdateTime    *util.HTime `gorm:"column:update_time;comment:'更新时间'" json:"updateTime"`
}

func (BastionHostAuthorization) TableName() string {
	return "bastion_host_authorization"
}

type AuthorizationHostLifecycleDto struct {
	HostID        uint        `json:"hostId" example:"11"`
	AuthIDs       []uint      `json:"authIds" example:"37,145"`
	SSHKeyIDs     []uint      `json:"sshKeyIds" example:"37,145"`
	EffectiveTime *util.HTime `json:"effectiveTime" swaggertype:"string" example:"2026-04-24 09:00:00"`
	ExpireTime    *util.HTime `json:"expireTime" swaggertype:"string" example:"2026-04-30 23:59:59"`
}

type AuthorizeHostsToUserDto struct {
	UserID         uint                            `validate:"required" json:"userId" example:"1001"`
	HostIDs        []uint                          `json:"hostIds" example:"11,12,13"`
	AuthIDs        []uint                          `json:"authIds" example:"37,145"`
	SSHKeyIDs      []uint                          `json:"sshKeyIds" example:"37,145"`
	EffectiveTime  *util.HTime                     `json:"effectiveTime" swaggertype:"string" example:"2026-04-24 09:00:00"`
	ExpireTime     *util.HTime                     `json:"expireTime" swaggertype:"string" example:"2026-04-30 23:59:59"`
	Authorizations []AuthorizationHostLifecycleDto `json:"authorizations"`
}

type AuthorizeHostsToGroupDto struct {
	GroupID        uint                            `validate:"required" json:"groupId" example:"2001"`
	HostIDs        []uint                          `json:"hostIds" example:"11,12,13"`
	AuthIDs        []uint                          `json:"authIds" example:"37,145"`
	SSHKeyIDs      []uint                          `json:"sshKeyIds" example:"37,145"`
	EffectiveTime  *util.HTime                     `json:"effectiveTime" swaggertype:"string" example:"2026-04-24 09:00:00"`
	ExpireTime     *util.HTime                     `json:"expireTime" swaggertype:"string" example:"2026-04-30 23:59:59"`
	Authorizations []AuthorizationHostLifecycleDto `json:"authorizations"`
}

type RevokeHostsFromUserDto struct {
	UserID  uint   `validate:"required" json:"userId" example:"1001"`
	HostIDs []uint `validate:"required" json:"hostIds" example:"11,12,13"`
}

type RevokeHostsFromGroupDto struct {
	GroupID uint   `validate:"required" json:"groupId" example:"2001"`
	HostIDs []uint `validate:"required" json:"hostIds" example:"11,12,13"`
}

type GetAuthorizedHostsDto struct {
	SubjectType    string `form:"subjectType" binding:"required,oneof=user group"`
	SubjectID      uint   `form:"subjectId" binding:"required"`
	IncludeExpired bool   `form:"includeExpired"`
	Page           int    `form:"page" binding:"required,min=1"`
	PageSize       int    `form:"pageSize" binding:"required,min=1,max=100"`
}

type CheckUserHostPermissionDto struct {
	UserID uint `form:"userId" validate:"required"`
	HostID uint `form:"hostId" validate:"required"`
	AuthID uint `form:"authId"`
}

type BastionHostAuthorizationVo struct {
	ID          uint       `json:"id"`
	SubjectType string     `json:"subjectType"`
	SubjectID   uint       `json:"subjectId"`
	SubjectName string     `json:"subjectName"`
	HostID      uint       `json:"hostId"`
	HostName    string     `json:"hostName"`
	SSHIP       string     `json:"sshIp"`
	CreateTime  util.HTime `json:"createTime"`
}

const (
	AuthorizationStatusPending = "pending"
	AuthorizationStatusActive  = "active"
	AuthorizationStatusExpired = "expired"
)

func ResolveAuthorizationStatus(effectiveTime, expireTime *util.HTime, now time.Time) string {
	if effectiveTime != nil && effectiveTime.Time.After(now) {
		return AuthorizationStatusPending
	}
	if expireTime != nil && !expireTime.Time.After(now) {
		return AuthorizationStatusExpired
	}
	return AuthorizationStatusActive
}

type BastionAuthorizedHostVo struct {
	CmdbHostVo
	AuthorizationID     uint                   `json:"authorizationId"`
	SubjectType         string                 `json:"subjectType"`
	SubjectID           uint                   `json:"subjectId"`
	AuthorizedAuthIDs   []uint                 `json:"authorizedAuthIds"`
	AuthorizedSSHAuths  []CmdbHostAuthOptionVo `json:"authorizedSshAuths"`
	AuthorizationStatus string                 `json:"authorizationStatus"`
	EffectiveTime       *util.HTime            `json:"effectiveTime"`
	AuthExpireTime      *util.HTime            `json:"authExpireTime"`
	AuthCreateTime      util.HTime             `json:"authCreateTime"`
	AuthUpdateTime      *util.HTime            `json:"authUpdateTime"`
}

func NormalizeAuthorizationAuthIDs(groups ...[]uint) []uint {
	ids := make([]uint, 0)
	seen := map[uint]struct{}{}
	for _, group := range groups {
		for _, id := range group {
			if id == 0 {
				continue
			}
			if _, ok := seen[id]; ok {
				continue
			}
			seen[id] = struct{}{}
			ids = append(ids, id)
		}
	}
	return ids
}

func EncodeAuthorizationAuthIDs(authIDs []uint) string {
	authIDs = NormalizeAuthorizationAuthIDs(authIDs)
	if len(authIDs) == 0 {
		return ""
	}
	parts := make([]string, 0, len(authIDs))
	for _, authID := range authIDs {
		parts = append(parts, strconv.FormatUint(uint64(authID), 10))
	}
	return strings.Join(parts, ",")
}

func DecodeAuthorizationAuthIDs(raw string) []uint {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	ids := make([]uint, 0, len(parts))
	for _, part := range parts {
		id, err := strconv.ParseUint(strings.TrimSpace(part), 10, 64)
		if err != nil || id == 0 {
			continue
		}
		ids = append(ids, uint(id))
	}
	return NormalizeAuthorizationAuthIDs(ids)
}
