package runner

import "testing"

func TestRuntimeLimitsNormalized(t *testing.T) {
	t.Parallel()

	got := (RuntimeLimits{}).Normalized()
	if got.LLMTimeoutSeconds != DefaultLLMTimeoutSeconds {
		t.Fatalf("LLMTimeoutSeconds = %d, want %d", got.LLMTimeoutSeconds, DefaultLLMTimeoutSeconds)
	}
	if got.ACPTimeoutSeconds != DefaultACPTimeoutSeconds {
		t.Fatalf("ACPTimeoutSeconds = %d, want %d", got.ACPTimeoutSeconds, DefaultACPTimeoutSeconds)
	}
	if got.MaxResponseBytes != DefaultMaxResponseBytes {
		t.Fatalf("MaxResponseBytes = %d, want %d", got.MaxResponseBytes, DefaultMaxResponseBytes)
	}
	if got.InvalidAttemptLimit != DefaultInvalidAttemptLimit {
		t.Fatalf("InvalidAttemptLimit = %d, want %d", got.InvalidAttemptLimit, DefaultInvalidAttemptLimit)
	}
}
