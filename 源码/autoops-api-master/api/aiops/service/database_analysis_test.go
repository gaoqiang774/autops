package service

import (
	"strings"
	"testing"

	aiopsmodel "dodevops-api/api/aiops/model"
)

func TestBuildRuleBasedMySQLResultFindsGlobalStallAndHotspots(t *testing.T) {
	slowText := `
# Time: 2026-05-14T11:42:05.094355+08:00
# User@Host: amoeba_u1204[amoeba_u1204] @  [192.168.196.12]  Id: 43518867
# Schema: jjsapp  Last_errno: 1287  Killed: 0
# Query_time: 5.694468  Lock_time: 0.000000  Rows_sent: 1  Rows_examined: 0  Rows_affected: 0
use jjsapp;
SET timestamp=1778730125;
select @@session.tx_read_only;
# Time: 2026-05-14T11:42:05.395600+08:00
# User@Host: ptiger_erp_u[ptiger_erp_u] @  [192.168.1.204]  Id: 43515176
# Schema: tiger_erp  Last_errno: 0  Killed: 0
# Query_time: 7.375255  Lock_time: 0.000056  Rows_sent: 0  Rows_examined: 0  Rows_affected: 1
use tiger_erp;
INSERT INTO SYSTEM_WARN_LOGIN (WORKER_ID, USER_AGENT, LOGIN_IP, CREATE_TIME) VALUES ('00412653','Chrome','119.123.185.212','2026-05-14 11:41:58.522');
# Time: 2026-05-14T11:42:05.395560+08:00
# User@Host: amoeba_u1204[amoeba_u1204] @  [192.168.208.10]  Id: 43418212
# Schema: workflowplatform  Last_errno: 0  Killed: 0
# Query_time: 7.439629  Lock_time: 0.000000  Rows_sent: 0  Rows_examined: 0  Rows_affected: 0
use workflowplatform;
commit;
# Time: 2026-05-14T11:42:05.800000+08:00
# User@Host: pzhao_pin_u[pzhao_pin_u] @  [192.168.1.36]  Id: 1
# Schema: zhao_pin  Last_errno: 0  Killed: 0
# Query_time: 1.851939  Lock_time: 0.000062  Rows_sent: 4674  Rows_examined: 1558648  Rows_affected: 0
use zhao_pin;
select distinct(n.process_id) from resume_process_node n left join resume_process_node_operator o on n.id = o.node_id where (n.open_emp_number = '02013511' or n.handle_emp_number = '02013511' or o.node_operator_emp_number = '02013511');
`
	systemText := `
May 14 11:50:01 db-204 pmm-agent: Error pinging mysqld err="context deadline exceeded"
May 14 11:50:09 db-204 systemd: mysqld.service stopping timed out. Terminating.
May 14 12:00:57 db-204 mysqld: Database was not shutdown normally!
May 14 12:01:01 db-204 mysqld: Crash recovery finished.
`

	result := NewMySQLAnalysisService().buildRuleBasedMySQLResult(&mysqlAnalyzeContext{
		req: &aiopsmodel.MySQLAnalyzeRequest{HostID: 1},
		texts: []string{
			slowText,
			systemText,
		},
	})
	if result.Status != "abnormal" {
		t.Fatalf("status = %q", result.Status)
	}
	if result.Severity != "critical" {
		t.Fatalf("severity = %q", result.Severity)
	}
	if result.SlowLogSummary.SlowConnectionProbeCount != 1 {
		t.Fatalf("connection probes = %d", result.SlowLogSummary.SlowConnectionProbeCount)
	}
	if result.SlowLogSummary.LargeScanCount != 1 {
		t.Fatalf("large scans = %d", result.SlowLogSummary.LargeScanCount)
	}
	if result.SlowLogSummary.SlowCommitCount != 1 {
		t.Fatalf("slow commits = %d", result.SlowLogSummary.SlowCommitCount)
	}
	if len(result.Hotspots.TopSchemas) == 0 {
		t.Fatal("expected schema hotspots")
	}
	foundTiger := false
	for _, item := range result.SlowLogSummary.TopSQLPatterns {
		if strings.Contains(item.Key, "SYSTEM_WARN_LOGIN") {
			foundTiger = true
		}
	}
	if !foundTiger {
		t.Fatalf("expected SYSTEM_WARN_LOGIN hotspot, got %#v", result.SlowLogSummary.TopSQLPatterns)
	}
}

func TestAnalyzeMySQLRequiresHostID(t *testing.T) {
	_, err := NewMySQLAnalysisService().AnalyzeMySQL(nil, &aiopsmodel.MySQLAnalyzeRequest{})
	if err == nil || !strings.Contains(err.Error(), "hostId") {
		t.Fatalf("expected hostId validation error, got %v", err)
	}
}

func TestNormalizeMySQLSQLPattern(t *testing.T) {
	cases := map[string]string{
		"select @@session.tx_read_only;": "SELECT @@session.tx_read_only",
		"commit;":                        "COMMIT",
		"INSERT INTO SYSTEM_WARN_LOGIN (id) VALUES (1);":           "INSERT INTO SYSTEM_WARN_LOGIN",
		"UPDATE devops_deploy_task SET ks_status=1 WHERE id=147;":  "UPDATE devops_deploy_task",
		"select * from resume_process_node where id = 123 limit 1": "SELECT FROM resume_process_node",
	}
	for input, expected := range cases {
		if actual := normalizeMySQLSQLPattern([]string{input}); actual != expected {
			t.Fatalf("pattern %q = %q, want %q", input, actual, expected)
		}
	}
}
