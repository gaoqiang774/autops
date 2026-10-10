package engine

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"

	"github.com/prometheus/common/model"
)

func fingerprint(ruleID uint, metric model.Metric) string {
	keys := make([]string, 0, len(metric))
	for key := range metric {
		if isFingerprintIgnoredLabel(string(key)) {
			continue
		}
		keys = append(keys, string(key))
	}
	sort.Strings(keys)

	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("rule:%d", ruleID))
	for _, key := range keys {
		builder.WriteString("|")
		builder.WriteString(key)
		builder.WriteString("=")
		builder.WriteString(string(metric[model.LabelName(key)]))
	}

	sum := sha1.Sum([]byte(builder.String()))
	return hex.EncodeToString(sum[:])
}

func isFingerprintIgnoredLabel(key string) bool {
	switch strings.ToLower(strings.TrimSpace(key)) {
	case "severity", "level", "priority", "first_value", "value", "rule_name", "fingerprint":
		return true
	default:
		return false
	}
}

func metricToJSONMap(metric model.Metric) map[string]interface{} {
	result := make(map[string]interface{}, len(metric))
	for key, value := range metric {
		result[string(key)] = string(value)
	}
	return result
}
