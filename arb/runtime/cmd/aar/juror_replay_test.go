package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"adjudication/arb/runtime/proceeding"
)

func TestResolveJurorReplaySelectionFindsMemberSnapshot(t *testing.T) {
	source := writeJurorReplaySource(t)
	writeJurorReplaySnapshot(t, source, "turn-000001-C1", "C1")
	want := writeJurorReplaySnapshot(t, source, "turn-000002-C2", "C2")

	got, err := resolveJurorReplaySelection(source, "", "", "C2")
	if err != nil {
		t.Fatalf("resolve selection: %v", err)
	}
	if got.Basis != proceeding.CouncilReplayBasisSnapshot || got.SnapshotDir != want || got.MemberID != "" {
		t.Fatalf("selection = %#v", got)
	}
}

func TestResolveJurorReplaySelectionRejectsAmbiguousSnapshots(t *testing.T) {
	source := writeJurorReplaySource(t)
	writeJurorReplaySnapshot(t, source, "turn-000001-C1", "C1")
	writeJurorReplaySnapshot(t, source, "turn-000002-C2", "C2")

	_, err := resolveJurorReplaySelection(source, "", "", "")
	if err == nil || !strings.Contains(err.Error(), "pass --snapshot or --member-id") {
		t.Fatalf("error = %v", err)
	}
}

func TestResolveJurorReplaySelectionFallsBackToReconstructedWithoutSnapshots(t *testing.T) {
	source := writeJurorReplaySource(t)

	got, err := resolveJurorReplaySelection(source, "", "", "")
	if err != nil {
		t.Fatalf("resolve selection: %v", err)
	}
	if got.Basis != proceeding.CouncilReplayBasisReconstructed || got.MemberID != "C1" || got.SnapshotDir != "" {
		t.Fatalf("selection = %#v", got)
	}
}

func writeJurorReplaySource(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "run.json"), []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func writeJurorReplaySnapshot(t *testing.T, source string, name string, memberID string) string {
	t.Helper()
	dir := filepath.Join(source, "council-turns", name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "input.json"), []byte(`{"member_id":"`+memberID+`"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}
