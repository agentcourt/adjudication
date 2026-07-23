package report

import (
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestScanRoots(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a", "run-1", "run.json"), "{}")
	writeFile(t, filepath.Join(dir, "a", "run-1", "nested", "state.json"), "{}")
	writeFile(t, filepath.Join(dir, "b", "deep", "run-2", "state.json"), "{}")
	writeFile(t, filepath.Join(dir, ".git", "run-3", "run.json"), "{}")
	writeFile(t, filepath.Join(dir, ".hidden", "run-4", "run.json"), "{}")
	writeFile(t, filepath.Join(dir, "pi-C7", "run-5", "run.json"), "{}")
	writeFile(t, filepath.Join(dir, "plain", "notes.txt"), "x")

	runs, problems := ScanRoots([]Root{{Name: "r", Path: dir}})
	if len(problems) != 0 {
		t.Fatalf("problems: %v", problems)
	}
	var rels []string
	for _, r := range runs {
		rels = append(rels, r.Rel)
	}
	want := []string{"a/run-1", "b/deep/run-2"}
	if len(rels) != len(want) {
		t.Fatalf("got %v, want %v", rels, want)
	}
	for i := range want {
		if rels[i] != want[i] {
			t.Fatalf("got %v, want %v", rels, want)
		}
	}
}

func TestScanRootsRootIsRun(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "run.json"), "{}")
	runs, _ := ScanRoots([]Root{{Name: "r", Path: dir}})
	if len(runs) != 1 || runs[0].Rel != "." {
		t.Fatalf("got %+v", runs)
	}
}

func TestScanRootsUnreadable(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("permission checks do not apply to root")
	}
	dir := t.TempDir()
	blocked := filepath.Join(dir, "blocked")
	if err := os.MkdirAll(blocked, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(blocked, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(blocked, 0o755) })

	_, problems := ScanRoots([]Root{{Name: "r", Path: dir}})
	if len(problems) != 1 {
		t.Fatalf("problems: %v", problems)
	}
}
