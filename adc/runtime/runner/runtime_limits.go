package runner

const (
	DefaultLLMTimeoutSeconds   = 180
	DefaultACPTimeoutSeconds   = 480
	DefaultMaxResponseBytes    = 128 * 1024
	DefaultInvalidAttemptLimit = 3
)

type RuntimeLimits struct {
	LLMTimeoutSeconds   int `json:"llm_timeout_seconds"`
	ACPTimeoutSeconds   int `json:"acp_timeout_seconds"`
	MaxResponseBytes    int `json:"max_response_bytes"`
	InvalidAttemptLimit int `json:"invalid_attempt_limit"`
}

func (limits RuntimeLimits) Normalized() RuntimeLimits {
	if limits.LLMTimeoutSeconds <= 0 {
		limits.LLMTimeoutSeconds = DefaultLLMTimeoutSeconds
	}
	if limits.ACPTimeoutSeconds <= 0 {
		limits.ACPTimeoutSeconds = DefaultACPTimeoutSeconds
	}
	if limits.MaxResponseBytes <= 0 {
		limits.MaxResponseBytes = DefaultMaxResponseBytes
	}
	if limits.InvalidAttemptLimit <= 0 {
		limits.InvalidAttemptLimit = DefaultInvalidAttemptLimit
	}
	return limits
}
