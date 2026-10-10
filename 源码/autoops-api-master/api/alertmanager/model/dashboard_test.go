package model

import (
	"sync"
	"testing"

	"gorm.io/gorm/schema"
)

func TestAlertDashboardEventRowDeclaresJSONColumnTypes(t *testing.T) {
	parsed, err := schema.Parse(&AlertDashboardEventRow{}, &sync.Map{}, schema.NamingStrategy{})
	if err != nil {
		t.Fatalf("parse dashboard event row schema: %v", err)
	}
	for _, fieldName := range []string{"NotifyRuleIDs", "Labels"} {
		field := parsed.LookUpField(fieldName)
		if field == nil || field.DataType != "json" {
			t.Fatalf("field %s data type = %v, want json", fieldName, field)
		}
	}
}
