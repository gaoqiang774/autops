package controller

import "testing"

func TestParseIdcVmRows(t *testing.T) {
	rows := [][]string{
		{"物理机IP", "虚拟机IP", "用户名", "虚拟机名称", "备注"},
		{"192.168.99.13", "192.168.99.100", "root", "big-data-99-100", "所属物理机: 192.168.99.13"},
		{"", "192.168.99.94", "root", "big-data-99-94", "所属物理机: 192.168.99.13"},
	}

	parsed, err := parseIdcVmRows(rows)
	if err != nil {
		t.Fatalf("parseIdcVmRows returned error: %v", err)
	}
	if len(parsed) != 2 {
		t.Fatalf("parseIdcVmRows len = %d, want 2", len(parsed))
	}
	if parsed[1].PhysicalIP != "192.168.99.13" {
		t.Fatalf("second row physical ip = %q, want %q", parsed[1].PhysicalIP, "192.168.99.13")
	}
}
