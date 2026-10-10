package model

func DefaultAlertNotifyTemplates() []AlertNotifyTemplate {
	return []AlertNotifyTemplate{
		{
			Ident:        "builtin_feishu_markdown",
			Name:         "飞书通知模板",
			ChannelType:  NotifyChannelFeishu,
			TemplateType: NotifyTemplateMarkdown,
			Description:  "发送飞书机器人的普通文本/Markdown 告警通知模板",
			Content: JSONMap{
				"title":   "{{ if .IsRecovered }}【已恢复】{{ else }}【报警中】{{ end }}{{ .RuleName }}",
				"content": feishuMarkdownTemplate,
				"footer":  "AIOps 运维平台",
			},
			Builtin:   1,
			Enabled:   1,
			CreatedBy: "system",
			UpdatedBy: "system",
		},
		{
			Ident:        "builtin_feishu_card",
			Name:         "飞书卡片通知模板",
			ChannelType:  NotifyChannelFeishuCard,
			TemplateType: NotifyTemplateCard,
			Description:  "发送飞书应用机器人的交互式卡片告警通知模板",
			Content: JSONMap{
				"raw": feishuCardTemplate,
			},
			Builtin:   1,
			Enabled:   1,
			CreatedBy: "system",
			UpdatedBy: "system",
		},
		{
			Ident:        "builtin_dingtalk_markdown",
			Name:         "钉钉通知模板",
			ChannelType:  NotifyChannelDingtalk,
			TemplateType: NotifyTemplateMarkdown,
			Description:  "发送钉钉机器人的 Markdown 告警通知模板",
			Content: JSONMap{
				"title":   "{{ if .IsRecovered }}【已恢复】{{ else }}【报警中】{{ end }}{{ .RuleName }}",
				"content": dingtalkMarkdownTemplate,
				"footer":  "AIOps 运维平台",
			},
			Builtin:   1,
			Enabled:   1,
			CreatedBy: "system",
			UpdatedBy: "system",
		},
		{
			Ident:        "builtin_wecom_markdown",
			Name:         "企业微信通知模板",
			ChannelType:  NotifyChannelWecom,
			TemplateType: NotifyTemplateMarkdown,
			Description:  "发送企业微信机器人的 Markdown 告警通知模板",
			Content: JSONMap{
				"title":   "{{ if .IsRecovered }}【已恢复】{{ else }}【报警中】{{ end }}{{ .RuleName }}",
				"content": wecomMarkdownTemplate,
				"footer":  "AIOps 运维平台",
			},
			Builtin:   1,
			Enabled:   1,
			CreatedBy: "system",
			UpdatedBy: "system",
		},
		{
			Ident:        "builtin_leliao_textcard",
			Name:         "乐聊通知模板",
			ChannelType:  NotifyChannelLeliao,
			TemplateType: NotifyTemplateMarkdown,
			Description:  "发送公司内部乐聊通知提醒的文本卡片模板",
			Content: JSONMap{
				"title":   "{{ if .IsRecovered }}【已恢复】{{ else }}【告警】{{ end }}AIOps告警中心-{{ .RuleName }}",
				"content": leliaoTextTemplate,
				"footer":  "AIOps 运维平台",
			},
			Builtin:   1,
			Enabled:   1,
			CreatedBy: "system",
			UpdatedBy: "system",
		},
	}
}

const feishuMarkdownTemplate = `{{ if .IsRecovered }}【已恢复】{{ else }}【报警中】{{ end }}：{{ .RuleName }}
【级别】: {{ .Severity }} {{ .SeverityIcon }}
{{ if ne .PolicyName .RuleName }}【策略】: {{ .PolicyName }}
{{ end }}【规则】: {{ .RuleName }}
{{ if gt .EventCount 1 }}【资源】: {{ .AffectedResources }}{{ else }}【目标】: {{ .ResourceName }}{{ end }}
{{ if .Condition }}【条件】: {{ .Condition }}{{ end }}
{{ if .ValueSummary }}【当前值】: {{ .ValueSummary }}{{ else if .Threshold }}【阈值】: {{ .Threshold }}{{ end }}
【时间】: {{ if .IsRecovered }}{{ .RecoveredAt }}{{ else }}{{ .FirstTriggeredAt }}{{ end }}
【持续】: {{ .Duration }}
{{ if gt .EventCount 1 }}【聚合】: {{ .EventCount }} 个事件 / {{ .ResourceCount }} 个资源{{ end }}
{{ if .Description }}【描述】: {{ .Description }}{{ end }}
{{ if .LabelLines }}【标签】：
{{ .LabelLines }}{{ end }}
【详情】: {{ .DetailURL }}
【静默】: {{ .SilenceURL }}`

const dingtalkMarkdownTemplate = `### {{ if .IsRecovered }}【已恢复】{{ else }}【报警中】{{ end }}{{ .RuleName }}
- **级别**: {{ .Severity }} {{ .SeverityIcon }}
{{ if ne .PolicyName .RuleName }}- **策略**: {{ .PolicyName }}
{{ end }}- **规则**: {{ .RuleName }}
{{ if gt .EventCount 1 }}- **资源**: {{ .AffectedResources }}
{{ else }}- **目标**: {{ .ResourceName }}
{{ end }}{{ if .Condition }}- **条件**: {{ .Condition }}
{{ end }}{{ if .ValueSummary }}- **当前值**: {{ .ValueSummary }}
{{ else if .Threshold }}- **阈值**: {{ .Threshold }}
{{ end }}- **时间**: {{ if .IsRecovered }}{{ .RecoveredAt }}{{ else }}{{ .FirstTriggeredAt }}{{ end }}
- **持续**: {{ .Duration }}
{{ if gt .EventCount 1 }}- **聚合**: {{ .EventCount }} 个事件 / {{ .ResourceCount }} 个资源
{{ end }}{{ if .Description }}- **描述**: {{ .Description }}
{{ end }}{{ if .LabelLines }}- **标签**:
{{ .LabelLines }}
{{ end }}[事件详情]({{ .DetailURL }}) | [静默1小时]({{ .SilenceURL }})`

const wecomMarkdownTemplate = `{{ if .IsRecovered }}<font color="info">【已恢复】{{ .RuleName }}</font>{{ else }}<font color="warning">【报警中】{{ .RuleName }}</font>{{ end }}
>级别: {{ .Severity }} {{ .SeverityIcon }}
{{ if ne .PolicyName .RuleName }}>策略: {{ .PolicyName }}
{{ end }}>规则: {{ .RuleName }}
{{ if gt .EventCount 1 }}>资源: {{ .AffectedResources }}
{{ else }}>目标: {{ .ResourceName }}
{{ end }}{{ if .Condition }}>条件: {{ .Condition }}
{{ end }}{{ if .ValueSummary }}>当前值: {{ .ValueSummary }}
{{ else if .Threshold }}>阈值: {{ .Threshold }}
{{ end }}>时间: {{ if .IsRecovered }}{{ .RecoveredAt }}{{ else }}{{ .FirstTriggeredAt }}{{ end }}
>持续: {{ .Duration }}
{{ if gt .EventCount 1 }}>聚合: {{ .EventCount }} 个事件 / {{ .ResourceCount }} 个资源
{{ end }}{{ if .Description }}>描述: {{ .Description }}
{{ end }}{{ if .LabelLines }}>标签:
{{ .LabelLines }}
{{ end }}[事件详情]({{ .DetailURL }}) | [静默1小时]({{ .SilenceURL }})`

const feishuCardTemplate = `{
  "schema": "2.0",
  "header": {
    "template": "{{ if .IsRecovered }}green{{ else }}red{{ end }}",
    "title": {
      "content": "{{ if .IsRecovered }}【已恢复】{{ else }}【报警中】{{ end }}- {{ .RuleName }}",
      "tag": "plain_text"
    }
  },
  "body": {
    "elements": [
      {
        "tag": "div",
        "text": {
          "tag": "lark_md",
          "content": "**级别**: {{ .Severity }} {{ .SeverityIcon }}{{ if ne .PolicyName .RuleName }}\n**策略**: {{ .PolicyName }}{{ end }}\n**规则**: {{ .RuleName }}{{ if gt .EventCount 1 }}\n**资源**: {{ .AffectedResources }}{{ else }}\n**目标**: {{ .ResourceName }}{{ end }}{{ if .ConditionCard }}\n**条件**: {{ .ConditionCard }}{{ end }}{{ if .ValueSummary }}\n**当前值**: {{ .ValueSummary }}{{ else if .Threshold }}\n**阈值**: {{ .Threshold }}{{ end }}\n**时间**: {{ if .IsRecovered }}{{ .RecoveredAt }}{{ else }}{{ .FirstTriggeredAt }}{{ end }}\n**持续**: {{ .Duration }}{{ if gt .EventCount 1 }}\n**聚合**: {{ .EventCount }} 个事件 / {{ .ResourceCount }} 个资源{{ end }}{{ if .Description }}\n**描述**: {{ .Description }}{{ end }}{{ if .LabelLinesCard }}\n**标签**:\n{{ .LabelLinesCard }}{{ end }}"
        }
      }{{ if not .IsRecovered }},
      {
        "tag": "hr"
      },
      {
        "tag": "column_set",
        "flex_mode": "none",
        "horizontal_spacing": "16px",
        "columns": [
          {
            "tag": "column",
            "width": "weighted",
            "elements": [
              {
                "tag": "button",
                "element_id": "claim_button_{{ .EventID }}",
                "text": {
                  "tag": "plain_text",
                  "content": "认领"
                },
                "type": "primary_filled",
                "width": "default",
                "behaviors": [
                  {
                    "type": "callback",
                    "value": {
                      "action": "claim",
                      "event_id": "{{ .EventID }}",
                      "event_ids": "{{ .EventIDs }}",
                      "fingerprint": "{{ .Fingerprint }}"
                    }
                  }
                ]
              }
            ]
          },
          {
            "tag": "column",
            "width": "weighted",
            "elements": [
              {
                "tag": "button",
                "element_id": "silence_button_{{ .EventID }}",
                "text": {
                  "tag": "plain_text",
                  "content": "静默一小时"
                },
                "type": "danger_filled",
                "width": "default",
                "behaviors": [
                  {
                    "type": "callback",
                    "value": {
                      "action": "silence",
                      "duration": "1h",
                      "event_id": "{{ .EventID }}",
                      "event_ids": "{{ .EventIDs }}",
                      "fingerprint": "{{ .Fingerprint }}"
                    }
                  }
                ]
              }
            ]
          }
        ]
      }{{ end }}
    ]
  }
}`

const leliaoTextTemplate = `{{ if .IsRecovered }}【已恢复】{{ else }}【报警中】{{ end }}：{{ .RuleName }}
【级别】: {{ .Severity }} {{ .SeverityIcon }}
{{ if ne .PolicyName .RuleName }}【策略】: {{ .PolicyName }}
{{ end }}【规则】: {{ .RuleName }}
{{ if gt .EventCount 1 }}【资源】: {{ .AffectedResources }}{{ else }}【目标】: {{ .ResourceName }}{{ end }}
{{ if .Condition }}【条件】: {{ .Condition }}{{ end }}
{{ if .ValueSummary }}【当前值】: {{ .ValueSummary }}{{ else if .Threshold }}【阈值】: {{ .Threshold }}{{ end }}
【时间】: {{ if .IsRecovered }}{{ .RecoveredAt }}{{ else }}{{ .FirstTriggeredAt }}{{ end }}
【持续】: {{ .Duration }}
{{ if gt .EventCount 1 }}【聚合】: {{ .EventCount }} 个事件 / {{ .ResourceCount }} 个资源{{ end }}
{{ if .Description }}【描述】: {{ .Description }}{{ end }}
{{ if .LabelLines }}【标签】：
{{ .LabelLines }}{{ end }}`
