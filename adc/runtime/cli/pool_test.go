package cli

import (
	"bytes"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSamplePoolRecordsDeterministic(t *testing.T) {
	rows := []poolRow{
		{Model: "m1", PersonaFile: "p1", Gene: 4, Cluster: 0},
		{Model: "m2", PersonaFile: "p2", Gene: 4, Cluster: 0},
		{Model: "m1", PersonaFile: "p1", Gene: 7, Cluster: 1},
		{Model: "m2", PersonaFile: "p2", Gene: 7, Cluster: 2},
		{Model: "m3", PersonaFile: "p3", Gene: 4, Cluster: 1},
		{Model: "m3", PersonaFile: "p3", Gene: 7, Cluster: 1},
		{Model: "m4", PersonaFile: "p4", Gene: 4, Cluster: 2},
		{Model: "m4", PersonaFile: "p4", Gene: 7, Cluster: 0},
	}
	var log bytes.Buffer
	rng := rand.New(rand.NewSource(1))
	selected, err := samplePoolRecords(rows, 3, rng, &bytes.Buffer{}, &log)
	if err != nil {
		t.Fatalf("samplePoolRecords error = %v", err)
	}
	if len(selected) != 3 {
		t.Fatalf("selected len = %d, want 3", len(selected))
	}
	if log.Len() == 0 {
		t.Fatalf("expected selection log output")
	}
	if !strings.Contains(log.String(), "Selection ") {
		t.Fatalf("expected selection-prefixed log output, got %q", log.String())
	}
	if !strings.Contains(log.String(), "max passes") {
		t.Fatalf("expected max-passes log output, got %q", log.String())
	}
}

func TestSamplePoolRecordsWarnsWhenSizeExceedsUniquePairs(t *testing.T) {
	rows := []poolRow{
		{Model: "m1", PersonaFile: "p1", Gene: 1, Cluster: 0},
		{Model: "m2", PersonaFile: "p2", Gene: 1, Cluster: 1},
	}
	var warnings bytes.Buffer
	var log bytes.Buffer
	rng := rand.New(rand.NewSource(1))
	selected, err := samplePoolRecords(rows, 3, rng, &warnings, &log)
	if err != nil {
		t.Fatalf("samplePoolRecords error = %v", err)
	}
	if len(selected) != 3 {
		t.Fatalf("selected len = %d, want 3", len(selected))
	}
	if !strings.Contains(warnings.String(), "warning: --size=3 exceeds available unique model/persona records (2)") {
		t.Fatalf("missing warning in log: %q", warnings.String())
	}
}

func TestRunPoolSuppressesTraceWithoutVerbose(t *testing.T) {
	rows := "m1,p1,1,0\nm2,p2,1,1\nm1,p1,2,0\nm2,p2,2,1\n"
	path, err := filepath.Abs(filepath.Join("..", "..", "etc", "persona-clusters.csv"))
	if err != nil {
		t.Fatalf("resolve persona-clusters path: %v", err)
	}
	original, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		t.Fatalf("ReadFile(%q) error = %v", path, err)
	}
	restore := func() {
		if original == nil {
			_ = os.Remove(path)
			return
		}
		_ = os.WriteFile(path, original, 0o644)
	}
	defer restore()
	if err := os.WriteFile(path, []byte(rows), 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if err := RunPool([]string{"--size", "1"}, &stdout, &stderr); err != nil {
		t.Fatalf("RunPool error = %v", err)
	}
	if strings.Contains(stderr.String(), "Selection ") {
		t.Fatalf("unexpected trace output without -v: %q", stderr.String())
	}
	if stdout.Len() == 0 {
		t.Fatal("expected sampled output")
	}
}

func TestHelpPool(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if err := Run([]string{"help", "pool"}, &stdout, &stderr); err != nil {
		t.Fatalf("Run(help pool) error = %v", err)
	}
	if !strings.Contains(stderr.String(), "Usage: adc pool") {
		t.Fatalf("pool help missing usage: %q", stderr.String())
	}
}
