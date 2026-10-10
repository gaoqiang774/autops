package model

import "time"

type WindowsFileUploadTaskStatus string

const (
	WindowsFileUploadTaskPending   WindowsFileUploadTaskStatus = "pending"
	WindowsFileUploadTaskUploading WindowsFileUploadTaskStatus = "uploading"
	WindowsFileUploadTaskCompleted WindowsFileUploadTaskStatus = "completed"
	WindowsFileUploadTaskFailed    WindowsFileUploadTaskStatus = "failed"
)

type WindowsFileUploadTask struct {
	TaskID        string                      `json:"taskId"`
	HostID        uint                        `json:"hostId"`
	FileName      string                      `json:"fileName"`
	FileSize      int64                       `json:"fileSize"`
	Mode          string                      `json:"mode"`
	RemotePath    string                      `json:"remotePath"`
	Status        WindowsFileUploadTaskStatus `json:"status"`
	Progress      float64                     `json:"progress"`
	UploadedBytes int64                       `json:"uploadedBytes"`
	TotalBytes    int64                       `json:"totalBytes"`
	Message       string                      `json:"message"`
	Error         string                      `json:"error"`
	CreatedBy     uint                        `json:"createdBy"`
	CreatedByName string                      `json:"createdByName"`
	CreatedAt     time.Time                   `json:"createdAt"`
	StartedAt     *time.Time                  `json:"startedAt,omitempty"`
	CompletedAt   *time.Time                  `json:"completedAt,omitempty"`
}
