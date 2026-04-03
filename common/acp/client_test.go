package acp

import "testing"

func TestCloseSuppressesIntentionalKill(t *testing.T) {
	t.Parallel()

	client, err := NewClient(Config{
		Command: "sleep",
		Args:    []string{"60"},
	})
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}
	if err := client.Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}
}
