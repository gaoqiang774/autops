package model

type DatabaseCommandEvidence struct {
	Name    string `json:"name"`
	Command string `json:"command,omitempty"`
	Output  string `json:"output"`
	Error   string `json:"error,omitempty"`
}

type MySQLAnalyzeRequest struct {
	ProviderID            uint     `json:"providerId"`
	UseLLM                bool     `json:"useLlm"`
	Prompt                string   `json:"prompt,omitempty"`
	HostID                uint     `json:"hostId" binding:"required"`
	AuthID                uint     `json:"authId,omitempty"`
	IncidentTime          string   `json:"incidentTime,omitempty"`
	WindowMinutes         int      `json:"windowMinutes,omitempty"`
	MySQLServiceName      string   `json:"mysqlServiceName,omitempty"`
	MySQLSocket           string   `json:"mysqlSocket,omitempty"`
	MySQLPort             int      `json:"mysqlPort,omitempty"`
	MySQLDefaultsFile     string   `json:"mysqlDefaultsFile,omitempty"`
	MySQLLoginPath        string   `json:"mysqlLoginPath,omitempty"`
	ErrorLogPath          string   `json:"errorLogPath,omitempty"`
	SlowLogPath           string   `json:"slowLogPath,omitempty"`
	SystemLogPaths        []string `json:"systemLogPaths,omitempty"`
	LogTailLines          int      `json:"logTailLines,omitempty"`
	CommandTimeoutSeconds int      `json:"commandTimeoutSeconds,omitempty"`
	IncludeRawEvidence    bool     `json:"includeRawEvidence,omitempty"`
}

type DatabaseDiagnosisEvidence struct {
	Type      string      `json:"type"`
	Title     string      `json:"title"`
	Reference string      `json:"reference,omitempty"`
	Content   string      `json:"content"`
	Raw       interface{} `json:"raw,omitempty"`
}

type DatabaseDiagnosisHypothesis struct {
	Cause              string   `json:"cause"`
	Confidence         string   `json:"confidence"`
	SupportingEvidence []string `json:"supportingEvidence"`
}

type MySQLSlowLogSummary struct {
	TotalEntries             int         `json:"totalEntries"`
	MaxQueryTimeSeconds      float64     `json:"maxQueryTimeSeconds"`
	MaxRowsExamined          int64       `json:"maxRowsExamined"`
	LargeScanCount           int         `json:"largeScanCount"`
	LowRowsNoLockSlowCount   int         `json:"lowRowsNoLockSlowCount"`
	SlowConnectionProbeCount int         `json:"slowConnectionProbeCount"`
	SlowCommitCount          int         `json:"slowCommitCount"`
	TopSchemas               []CountItem `json:"topSchemas,omitempty"`
	TopSQLPatterns           []CountItem `json:"topSqlPatterns,omitempty"`
	TopUsers                 []CountItem `json:"topUsers,omitempty"`
	TopHosts                 []CountItem `json:"topHosts,omitempty"`
}

type MySQLHotspots struct {
	TopHosts       []CountItem `json:"topHosts,omitempty"`
	TopUsers       []CountItem `json:"topUsers,omitempty"`
	TopSchemas     []CountItem `json:"topSchemas,omitempty"`
	TopSQLPatterns []CountItem `json:"topSqlPatterns,omitempty"`
}

type MySQLAnalyzeResult struct {
	Status            string                        `json:"status"`
	Severity          string                        `json:"severity"`
	Summary           string                        `json:"summary"`
	Conclusion        string                        `json:"conclusion"`
	DirectCause       string                        `json:"directCause,omitempty"`
	ImpactedResources []string                      `json:"impactedResources,omitempty"`
	Evidence          []DatabaseDiagnosisEvidence   `json:"evidence"`
	Hypotheses        []DatabaseDiagnosisHypothesis `json:"hypotheses"`
	Hotspots          MySQLHotspots                 `json:"hotspots"`
	SlowLogSummary    MySQLSlowLogSummary           `json:"slowLogSummary"`
	NextChecks        []string                      `json:"nextChecks"`
	Remediation       []string                      `json:"remediation"`
	SafeCommands      []DatabaseCommandEvidence     `json:"safeCommands"`
	MissingData       []string                      `json:"missingData"`
	CollectedCommands []DatabaseCommandEvidence     `json:"collectedCommands,omitempty"`
	LLMAnalysis       string                        `json:"llmAnalysis,omitempty"`
	UsedLLM           bool                          `json:"usedLlm"`
	ProviderID        uint                          `json:"providerId,omitempty"`
	ProviderName      string                        `json:"providerName,omitempty"`
	ModelCode         string                        `json:"modelCode,omitempty"`
	Readonly          bool                          `json:"readonly"`
	GeneratedAt       string                        `json:"generatedAt"`
}
