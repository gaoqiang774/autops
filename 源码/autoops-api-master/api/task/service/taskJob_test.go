package service

import (
	"testing"

	"dodevops-api/api/task/model"
)

func TestValidateTask(t *testing.T) {
	tests := []struct {
		name    string
		task    model.Task
		wantErr bool
	}{
		{name: "immediate", task: model.Task{Name: "inspect", Type: model.TaskTypeImmediate, Shell: "1", HostIDs: "2"}},
		{name: "scheduled", task: model.Task{Name: "nightly", Type: model.TaskTypeScheduled, Shell: "1", HostIDs: "2", CronExpr: "0 2 * * *"}},
		{name: "bad type", task: model.Task{Name: "bad", Type: 3, Shell: "1", HostIDs: "2"}, wantErr: true},
		{name: "missing target", task: model.Task{Name: "bad", Type: model.TaskTypeImmediate, Shell: "1"}, wantErr: true},
		{name: "missing cron", task: model.Task{Name: "bad", Type: model.TaskTypeScheduled, Shell: "1", HostIDs: "2"}, wantErr: true},
		{name: "invalid cron", task: model.Task{Name: "bad", Type: model.TaskTypeScheduled, Shell: "1", HostIDs: "2", CronExpr: "not cron"}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateTask(&tt.task)
			if (err != nil) != tt.wantErr {
				t.Fatalf("validateTask() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateTemplate(t *testing.T) {
	valid := model.TaskTemplate{Name: "health-check", Type: model.TemplateTypePython, Content: "print('ok')"}
	if err := validateTemplate(&valid); err != nil {
		t.Fatalf("valid template rejected: %v", err)
	}

	invalid := valid
	invalid.Type = 9
	if err := validateTemplate(&invalid); err == nil {
		t.Fatal("invalid template type was accepted")
	}
}
