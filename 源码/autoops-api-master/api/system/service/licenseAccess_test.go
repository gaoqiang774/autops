package service

import (
	"testing"

	"dodevops-api/api/system/model"
)

func TestFilterMenusByLicense(t *testing.T) {
	menus := []model.LeftMenuVo{
		{MenuName: "资产管理"},
		{MenuName: "容器管理"},
		{MenuName: "AIOps运维"},
		{MenuName: "系统管理"},
	}
	licensed := filterMenusByLicense(menus, true)
	if len(licensed) != len(menus) {
		t.Fatalf("licensed menu count = %d, want %d", len(licensed), len(menus))
	}
	restricted := filterMenusByLicense(menus, false)
	if len(restricted) != 2 || restricted[0].MenuName != "资产管理" || restricted[1].MenuName != "系统管理" {
		t.Fatalf("unlicensed menus = %+v", restricted)
	}
}
