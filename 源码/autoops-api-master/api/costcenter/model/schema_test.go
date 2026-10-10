package model

import (
	"sync"
	"testing"

	"gorm.io/gorm/schema"
)

func TestBillCloudAccountSchemaDoesNotStoreCredentials(t *testing.T) {
	parsed, err := schema.Parse(&BillCloudAccount{}, &sync.Map{}, schema.NamingStrategy{})
	if err != nil {
		t.Fatalf("parse bill cloud account schema: %v", err)
	}
	if parsed.Table != "bill_cloud_account" {
		t.Fatalf("table = %q, want bill_cloud_account", parsed.Table)
	}
	for _, field := range []string{"AccessKey", "SecretKey"} {
		if parsed.LookUpField(field) != nil {
			t.Fatalf("cloud credentials must not be stored on billing account: field=%s", field)
		}
	}
	for _, field := range []string{
		"KeyManageID", "AvailableBalance", "BalanceStatus", "LastBalanceTime",
		"BalanceAlertRuleID", "BalanceAlertEnabled", "BalanceAlertThreshold",
		"BalanceAlertThresholdDays", "BalanceAlertNotifyRuleIDs",
	} {
		if parsed.LookUpField(field) == nil {
			t.Fatalf("missing billing account field %s", field)
		}
	}
}

func TestBillDetailUniqueIndexIsScopedByAccountAndMonth(t *testing.T) {
	parsed, err := schema.Parse(&BillDetail{}, &sync.Map{}, schema.NamingStrategy{})
	if err != nil {
		t.Fatalf("parse bill detail schema: %v", err)
	}
	indexes := parsed.ParseIndexes()
	var index *schema.Index
	for i := range indexes {
		if indexes[i].Name == "uk_bill_detail_account_month_record" {
			index = indexes[i]
			break
		}
	}
	if index == nil {
		t.Fatal("missing uk_bill_detail_account_month_record")
	}
	want := []string{"cloud_account_id", "month", "record_id"}
	if len(index.Fields) != len(want) {
		t.Fatalf("index fields = %d, want %d", len(index.Fields), len(want))
	}
	for i, field := range index.Fields {
		if field.DBName != want[i] {
			t.Fatalf("index field[%d] = %q, want %q", i, field.DBName, want[i])
		}
	}
}
