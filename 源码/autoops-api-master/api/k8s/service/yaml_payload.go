package service

import (
	"fmt"
	"strings"

	"sigs.k8s.io/yaml"
)

func yamlPayloadBytes(payload map[string]interface{}) ([]byte, error) {
	if payload == nil {
		return nil, fmt.Errorf("YAML内容不能为空")
	}

	for _, key := range []string{"yaml", "yamlContent"} {
		raw, ok := payload[key]
		if !ok {
			continue
		}

		if content, ok := raw.(string); ok {
			if strings.TrimSpace(content) == "" {
				return nil, fmt.Errorf("YAML内容不能为空")
			}
			return []byte(content), nil
		}

		yamlBytes, err := yaml.Marshal(raw)
		if err != nil {
			return nil, err
		}
		return yamlBytes, nil
	}

	yamlBytes, err := yaml.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return yamlBytes, nil
}
