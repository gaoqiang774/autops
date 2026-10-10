package model

type WeComPluginChatRequest struct {
	Question       string                 `json:"question"`
	Query          string                 `json:"query"`
	Content        string                 `json:"content"`
	Text           string                 `json:"text"`
	SessionID      string                 `json:"sessionId"`
	UserID         string                 `json:"userId"`
	UserName       string                 `json:"userName"`
	ConversationID string                 `json:"conversationId"`
	ChatID         string                 `json:"chatId"`
	ChatType       string                 `json:"chatType"`
	ThreadID       string                 `json:"threadId"`
	BotID          string                 `json:"botId"`
	CorpID         string                 `json:"corpId"`
	MessageID      string                 `json:"messageId"`
	ProviderID     uint                   `json:"providerId"`
	Context        map[string]interface{} `json:"context"`
}

type WeComPluginChatResponse struct {
	Answer           string                   `json:"answer"`
	SessionID        string                   `json:"sessionId"`
	RunID            uint64                   `json:"runId"`
	Intent           string                   `json:"intent,omitempty"`
	SuggestedActions []string                 `json:"suggestedActions,omitempty"`
	MissingInfo      []string                 `json:"missingInfo,omitempty"`
	Evidence         []AgentDiagnosisEvidence `json:"evidence,omitempty"`
	AssistantResult  *AssistantChatResult     `json:"assistantResult,omitempty"`
}
