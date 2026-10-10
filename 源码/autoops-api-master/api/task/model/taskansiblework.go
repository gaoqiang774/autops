package model

import "time"

// TaskAnsibleWork Ansible子任务表
type TaskAnsibleWork struct {
	ID            uint        `gorm:"primaryKey;comment:'主键ID'" json:"id"`
	TaskID        uint        `gorm:"not null;index:idx_task_id;index:idx_task_work_composite;comment:'父任务ID'" json:"taskID"`
	EntryFileName string      `gorm:"size:255;not null;comment:'入口文件名'" json:"entryFileName"`
	EntryFilePath string      `gorm:"size:255;not null;comment:'入口文件路径'" json:"entryFilePath"`
	LogPath       string      `gorm:"size:255;comment:'日志路径'" json:"logPath"`
	Status        int         `json:"status" gorm:"not null;default:1;index:idx_task_work_composite;comment:'子任务状态:1-等待中,2-运行中,3-成功,4-异常'"`
	StartTime     *time.Time  `gorm:"comment:'开始时间'" json:"startTime,omitempty"`
	EndTime       *time.Time  `gorm:"comment:'结束时间'" json:"endTime,omitempty"`
	Duration      int         `gorm:"comment:'执行耗时(秒)'" json:"duration"`
	ExitCode      int         `gorm:"comment:'退出代码'" json:"exitCode"`
	ErrorMsg      string      `gorm:"type:text;comment:'错误信息'" json:"errorMsg,omitempty"`
	Task          TaskAnsible `gorm:"foreignKey:TaskID;comment:'父任务'" json:"-"`
}

// TaskAnsibleStatus 任务状态结构体
type TaskAnsibleStatus struct {
	Status    int       `json:"status"`
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time"`
	Log       string    `json:"log"`
}

func (TaskAnsibleWork) TableName() string {
	return "task_ansiblework"
}
