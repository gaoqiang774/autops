package service

import "testing"

func TestParseAccountAuthImportRows(t *testing.T) {
	tableData := `账号别名            账号地址                                    用户名  密码
乐有家dev环境登录   https://172.16.3.100/jjslogin/tologin       000001  lyj.123456
乐有家test环境登录  https://itest.leyoujia.com/jjslogin/tologin 000001  lyj.123456`

	rows, failedRows, total := parseAccountAuthImportRows(tableData)
	if total != 2 {
		t.Fatalf("total = %d, want 2", total)
	}
	if len(failedRows) != 0 {
		t.Fatalf("failedRows = %#v, want empty", failedRows)
	}
	if len(rows) != 2 {
		t.Fatalf("len(rows) = %d, want 2", len(rows))
	}
	if rows[0].Alias != "乐有家dev环境登录" {
		t.Fatalf("rows[0].Alias = %q", rows[0].Alias)
	}
	if rows[0].Host != "https://172.16.3.100/jjslogin/tologin" {
		t.Fatalf("rows[0].Host = %q", rows[0].Host)
	}
	if rows[0].Username != "000001" {
		t.Fatalf("rows[0].Username = %q", rows[0].Username)
	}
	if rows[0].Password != "lyj.123456" {
		t.Fatalf("rows[0].Password = %q", rows[0].Password)
	}
}

func TestInferAccountAuthPort(t *testing.T) {
	tests := []struct {
		name string
		host string
		want int
	}{
		{name: "https default", host: "https://itest.leyoujia.com/jjslogin/tologin", want: 443},
		{name: "http default", host: "http://example.com/login", want: 80},
		{name: "explicit url port", host: "https://example.com:8443/login", want: 8443},
		{name: "host port", host: "example.com:9090", want: 9090},
		{name: "no port", host: "example.com", want: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := inferAccountAuthPort(tt.host); got != tt.want {
				t.Fatalf("inferAccountAuthPort(%q) = %d, want %d", tt.host, got, tt.want)
			}
		})
	}
}
