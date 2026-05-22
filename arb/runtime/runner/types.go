package runner

import (
	"adjudication/arb/runtime/lean"
	"adjudication/arb/runtime/spec"
)

type Policy struct {
	CouncilSize                        int    `json:"council_size"`
	EvidenceStandard                   string `json:"evidence_standard"`
	RequiredVotesForDecision           int    `json:"required_votes_for_decision"`
	MaxDeliberationRounds              int    `json:"max_deliberation_rounds"`
	MaxOpeningChars                    int    `json:"max_opening_chars"`
	MaxArgumentChars                   int    `json:"max_argument_chars"`
	MaxRebuttalChars                   int    `json:"max_rebuttal_chars"`
	MaxSurrebuttalChars                int    `json:"max_surrebuttal_chars"`
	MaxClosingChars                    int    `json:"max_closing_chars"`
	MaxExhibitsPerFiling               int    `json:"max_exhibits_per_filing"`
	MaxExhibitsPerSide                 int    `json:"max_exhibits_per_side"`
	MaxExhibitBytes                    int    `json:"max_exhibit_bytes"`
	MaxReportsPerFiling                int    `json:"max_reports_per_filing"`
	MaxReportsPerSide                  int    `json:"max_reports_per_side"`
	MaxReportTitleBytes                int    `json:"max_report_title_bytes"`
	MaxReportSummaryBytes              int    `json:"max_report_summary_bytes"`
	MaxSubmittedArtifactPerSide        int    `json:"max_submitted_artifacts_per_side"`
	MaxSubmittedArtifactBytes          int    `json:"max_submitted_artifacts_bytes"`
	MaxDirectSubmittedArtifactBytes    int    `json:"max_direct_submitted_artifacts_bytes"`
	MaxArtifactUploadBytes             int    `json:"max_artifact_upload_bytes"`
	MaxArtifactChunkBytes              int    `json:"max_artifact_chunk_bytes"`
	MaxArtifactReadBytes               int    `json:"max_artifact_read_bytes"`
	MaxArtifactReadsPerOpportunity     int    `json:"max_artifact_reads_per_opportunity"`
	MaxArtifactReadBytesPerOpportunity int    `json:"max_artifact_read_bytes_per_opportunity"`
}

type RuntimeLimits struct {
	CouncilLLMTimeoutSeconds  int   `json:"council_llm_timeout_seconds"`
	AttorneyACPTimeoutSeconds int   `json:"attorney_acp_timeout_seconds"`
	MaxResponseBytes          int   `json:"max_response_bytes"`
	InvalidAttemptLimit       int   `json:"invalid_attempt_limit"`
	CouncilMaxOutputTokens    int64 `json:"council_max_output_tokens"`
}

type AttorneyRoleConfig struct {
	Model       string
	ACPCommand  string
	ACPEndpoint string
	SessionCwd  string
}

type AttorneyRunInfo struct {
	Role          string `json:"role"`
	Model         string `json:"model,omitempty"`
	SearchEnabled *bool  `json:"search_enabled,omitempty"`
	ACPTransport  string `json:"acp_transport"`
	ACPCommand    string `json:"acp_command,omitempty"`
	ACPEndpoint   string `json:"acp_endpoint,omitempty"`
	SessionCwd    string `json:"-"`
}

type Config struct {
	RunID                      string
	ComplaintPath              string
	CaseFilePaths              []string
	OutputDir                  string
	CommonRoot                 string
	CouncilPoolPath            string
	AttorneyModel              string
	AttorneyInstructionsPath   string
	PromptDir                  string
	AttorneyCommonPromptPath   string
	AttorneyArgumentPromptPath string
	AttorneyRebuttalPromptPath string
	PlaintiffAttorney          AttorneyRoleConfig
	DefendantAttorney          AttorneyRoleConfig
	Policy                     Policy
	Runtime                    RuntimeLimits
	XProxyConfigPath           string
	XProxyPort                 int
	ACPCommand                 string
	ACPArgs                    []string
	ACPEnv                     []string
	Engine                     lean.Engine
}

type Result struct {
	RunID             string                  `json:"run_id"`
	StartedAt         string                  `json:"started_at"`
	FinishedAt        string                  `json:"finished_at"`
	Status            string                  `json:"status"`
	Phase             string                  `json:"phase"`
	Resolution        string                  `json:"resolution"`
	Complaint         spec.Complaint          `json:"complaint"`
	EvidenceStandard  string                  `json:"evidence_standard"`
	Attorneys         []AttorneyRunInfo       `json:"attorneys"`
	CaseFiles         []CaseFileMeta          `json:"case_files"`
	SubmittedArtifact []SubmittedArtifactMeta `json:"submitted_artifacts,omitempty"`
	Artifacts         []ArtifactMeta          `json:"artifacts,omitempty"`
	Council           []CouncilSeat           `json:"council"`
	Events            []Event                 `json:"events"`
	FinalState        map[string]any          `json:"final_state"`
	FinalReason       string                  `json:"final_reason"`
}

type CaseFile struct {
	ArtifactID   string
	Name         string
	Path         string
	MimeType     string
	TextReadable bool
	SizeBytes    int
	Text         string
}

type CaseFileMeta struct {
	ArtifactID   string `json:"artifact_id"`
	Name         string `json:"name"`
	MimeType     string `json:"mime_type"`
	TextReadable bool   `json:"text_readable"`
}

type SubmittedArtifactMeta struct {
	Phase              string `json:"phase"`
	Role               string `json:"role"`
	ArtifactID         string `json:"artifact_id"`
	Name               string `json:"name"`
	Title              string `json:"title"`
	SourceURL          string `json:"source_url,omitempty"`
	SourceDescription  string `json:"source_description,omitempty"`
	MimeType           string `json:"mime_type"`
	RetrievalTimestamp string `json:"retrieval_timestamp"`
	Relevance          string `json:"relevance"`
	SHA256             string `json:"sha256"`
	SizeBytes          int    `json:"size_bytes"`
}

type ArtifactUploadSession struct {
	UploadID           string
	Role               string
	Phase              string
	Title              string
	MimeType           string
	ExpectedSizeBytes  int
	ExpectedSHA256     string
	SourceURL          string
	SourceDescription  string
	RetrievalTimestamp string
	Relevance          string
	ParentArtifactID   string
	DerivationMethod   string
	Path               string
	ReceivedBytes      int
}

type ArtifactMeta struct {
	ArtifactID          string `json:"artifact_id"`
	SHA256              string `json:"sha256"`
	SizeBytes           int    `json:"size_bytes"`
	MimeType            string `json:"mime_type"`
	StorageName         string `json:"storage_name"`
	CreatedAt           string `json:"created_at"`
	AdmissibilityStatus string `json:"admissibility_status"`
	RecordVisibility    string `json:"record_visibility"`
	Title               string `json:"title,omitempty"`
	OriginalName        string `json:"original_name,omitempty"`
	SourceURL           string `json:"source_url,omitempty"`
	SourceDescription   string `json:"source_description,omitempty"`
	RetrievalTimestamp  string `json:"retrieval_timestamp,omitempty"`
	SubmittedByRole     string `json:"submitted_by_role,omitempty"`
	SubmittedPhase      string `json:"submitted_phase,omitempty"`
	ParentArtifactID    string `json:"parent_artifact_id,omitempty"`
	ParentSHA256        string `json:"parent_sha256,omitempty"`
	DerivationMethod    string `json:"derivation_method,omitempty"`
	Relevance           string `json:"relevance,omitempty"`
	TextReadable        bool   `json:"text_readable"`
}

type CouncilSeat struct {
	MemberID    string `json:"member_id"`
	Model       string `json:"model"`
	PersonaFile string `json:"persona_file"`
	PersonaText string `json:"-"`
}

type Opportunity struct {
	ID           string
	Role         string
	Phase        string
	MayPass      bool
	Objective    string
	AllowedTools []string
}

type Event struct {
	Timestamp string         `json:"timestamp"`
	Turn      int            `json:"turn"`
	Role      string         `json:"role"`
	Phase     string         `json:"phase"`
	Type      string         `json:"type"`
	Payload   map[string]any `json:"payload,omitempty"`
}

type runContext struct {
	cfg               Config
	complaint         spec.Complaint
	state             map[string]any
	caseFiles         []CaseFile
	fileByID          map[string]CaseFile
	submittedArtifact []SubmittedArtifactMeta
	artifacts         []ArtifactMeta
	artifactByID      map[string]ArtifactMeta
	artifactStoreDir  string
	uploadSessions    map[string]*ArtifactUploadSession
	council           []CouncilSeat
	attorneys         map[string]AttorneyRunInfo
	acpSessions       map[string]*acpPersistentSession
	workProductDirs   map[string]string
	events            []Event
	turn              int
}
