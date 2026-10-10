package service

import (
	"dodevops-api/api/system/dao"
	"dodevops-api/api/system/model"
	"dodevops-api/pkg/license"
	"strings"
)

type SessionAccess struct {
	LeftMenuList   []model.LeftMenuVo `json:"leftMenuList"`
	PermissionList []string           `json:"permissionList"`
	License        license.Status     `json:"license"`
}

func BuildSessionAccess(adminID uint, username ...string) SessionAccess {
	status := license.DefaultManager().Status()
	isVisitor := len(username) > 0 && strings.EqualFold(strings.TrimSpace(username[0]), "test")
	leftMenuList := dao.QueryLeftMenuList(adminID)
	if isVisitor {
		leftMenuList = dao.QueryAllLeftMenuList()
	}
	leftMenuVo := make([]model.LeftMenuVo, 0, len(leftMenuList))
	for _, value := range leftMenuList {
		menuSvoList := dao.QueryMenuVoList(adminID, value.Id)
		if isVisitor {
			menuSvoList = dao.QueryAllMenuVoList(value.Id)
		}
		item := model.LeftMenuVo{
			MenuSvoList: menuSvoList,
			Id:          value.Id,
			MenuName:    value.MenuName,
			Icon:        value.Icon,
			Url:         value.Url,
		}
		leftMenuVo = append(leftMenuVo, item)
	}
	leftMenuVo = filterMenusByLicense(leftMenuVo, status.Licensed)

	permissionList := make([]string, 0)
	if !isVisitor {
		permissions := dao.QueryPermissionList(adminID)
		for _, value := range permissions {
			permissionList = append(permissionList, value.Value)
		}
	}
	return SessionAccess{
		LeftMenuList:   leftMenuVo,
		PermissionList: permissionList,
		License:        status,
	}
}

func filterMenusByLicense(menus []model.LeftMenuVo, licensed bool) []model.LeftMenuVo {
	if licensed {
		return menus
	}
	filtered := make([]model.LeftMenuVo, 0, 2)
	for _, menu := range menus {
		switch menu.MenuName {
		case "资产管理", "系统管理":
			filtered = append(filtered, menu)
		}
	}
	return filtered
}
