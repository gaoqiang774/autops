package service

import (
	"encoding/json"
	"reflect"
	"testing"

	"dodevops-api/api/system/model"

	"github.com/go-playground/validator/v10"
)

func TestAddSysAdminAllowsOptionalCredentials(t *testing.T) {
	dto := model.AddSysAdminDto{
		PostId: 1, RoleId: 1, DeptId: 1,
		Username: "zhangsan", Nickname: "张三", Status: 1,
	}
	if err := validator.New().Struct(dto); err != nil {
		t.Fatalf("password and email should be optional: %v", err)
	}
}

func TestAddSysAdminDecodesOptionalProfileFields(t *testing.T) {
	var dto model.AddSysAdminDto
	payload := []byte(`{"email":"zhangsan@example.com","icon":"/api/v1/upload/avatar/zhangsan.svg"}`)
	if err := json.Unmarshal(payload, &dto); err != nil {
		t.Fatalf("decode add-user payload: %v", err)
	}
	if dto.Email != "zhangsan@example.com" {
		t.Fatalf("email = %q", dto.Email)
	}
	if dto.Icon != "/api/v1/upload/avatar/zhangsan.svg" {
		t.Fatalf("icon = %q", dto.Icon)
	}
}

func TestInitializeSysAdminPassword(t *testing.T) {
	tests := []struct {
		name     string
		password string
		want     string
	}{
		{name: "missing", want: defaultSysAdminPassword},
		{name: "blank", password: "   ", want: defaultSysAdminPassword},
		{name: "explicit", password: "custom-password", want: "custom-password"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := initializeSysAdminPassword(tt.password); got != tt.want {
				t.Fatalf("initializeSysAdminPassword() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestUserModelsDoNotExposeRemovedFields(t *testing.T) {
	types := []reflect.Type{
		reflect.TypeOf(model.SysAdmin{}),
		reflect.TypeOf(model.JwtAdmin{}),
		reflect.TypeOf(model.AddSysAdminDto{}),
		reflect.TypeOf(model.UpdateSysAdminDto{}),
		reflect.TypeOf(model.SysAdminInfo{}),
		reflect.TypeOf(model.SysAdminVo{}),
	}
	for _, typ := range types {
		if _, ok := typ.FieldByName("UserID"); ok {
			t.Fatalf("%s should not contain UserID", typ.Name())
		}
	}

	password, ok := reflect.TypeOf(model.SysAdmin{}).FieldByName("Password")
	if !ok || password.Tag.Get("json") != "-" {
		t.Fatal("SysAdmin password must not be exposed in JSON")
	}
}

func TestUpdateSysAdminAllowsRemovedOptionalFields(t *testing.T) {
	dto := model.UpdateSysAdminDto{
		Id: 89, PostId: 1, DeptId: 5, RoleId: 1,
		Username: "admin", Nickname: "管理员", Status: 1,
	}
	if err := validator.New().Struct(dto); err != nil {
		t.Fatalf("update should not require user_id or email: %v", err)
	}
}
