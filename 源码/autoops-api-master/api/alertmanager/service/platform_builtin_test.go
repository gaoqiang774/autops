package service

import (
	"testing"

	"dodevops-api/api/alertmanager/model"
)

func TestIsPlatformBuiltinRuleTemplate(t *testing.T) {
	if !isPlatformBuiltinRuleTemplate(&model.AlertBuiltinRuleTemplate{Source: " platform "}) {
		t.Fatal("platform template should be read-only")
	}
	if isPlatformBuiltinRuleTemplate(&model.AlertBuiltinRuleTemplate{Source: "manual"}) {
		t.Fatal("manual template should remain editable")
	}
	if isPlatformBuiltinRuleTemplate(nil) {
		t.Fatal("nil template should not be treated as platform-owned")
	}
}
