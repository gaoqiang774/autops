package model

import "time"

// TaskAnsible Ansible任务主表
type TaskAnsible struct {
	ID            uint              `gorm:"primaryKey;comment:'主键ID'" json:"id"`
	Name          string            `gorm:"size:100;not null;uniqueIndex;comment:'任务名称'" json:"name"`
	Description   string            `gorm:"type:text;comment:'任务描述'" json:"description"`
	Type          int               `gorm:"not null;default:1;comment:'任务类型:1-手动,2-Git,3-K8s'" json:"type"`
	GitRepo       string            `gorm:"size:255;comment:'Git仓库地址'" json:"gitRepo"`
	HostGroups    string            `gorm:"type:text;not null;comment:'主机分组JSON'" json:"hostGroups"`
	AllHostIDs    string            `gorm:"type:text;not null;comment:'所有主机ID JSON数组'" json:"allHostIDs"`
	GlobalVars    string            `gorm:"type:text;comment:'全局变量JSON'" json:"globalVars"`
	Status        int               `json:"status" gorm:"not null;default:1;index:idx_task_status;comment:'任务状态:1-等待中,2-运行中,3-成功,4-异常'"`
	ErrorMsg      string            `gorm:"type:text;comment:'错误信息'" json:"errorMsg"`
	TaskCount     int               `gorm:"not null;default:0;comment:'任务数量(Type=1时为上传文件数,Type=2时为解析的playbook数,Type=3时固定为1)'" json:"taskCount"`
	TotalDuration int               `gorm:"not null;default:0;comment:'任务执行总耗时(秒,所有子任务耗时总和)'" json:"totalDuration"`
	CreatedAt     time.Time         `gorm:"not null;comment:'创建时间'" json:"createdAt"`
	UpdatedAt     time.Time         `gorm:"not null;comment:'更新时间'" json:"updatedAt"`
	Works         []TaskAnsibleWork `gorm:"foreignKey:TaskID;comment:'子任务列表'" json:"works,omitempty"`
}

func (TaskAnsible) TableName() string {
	return "task_ansible"
}
