package proceeding

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveExplicitCaseFilesExpandsGlob(t *testing.T) {
	dir := t.TempDir()
	first := filepath.Join(dir, "a.txt")
	second := filepath.Join(dir, "b.txt")
	if err := os.WriteFile(first, []byte("a"), 0o644); err != nil {
		t.Fatalf("write first: %v", err)
	}
	if err := os.WriteFile(second, []byte("b"), 0o644); err != nil {
		t.Fatalf("write second: %v", err)
	}

	got, err := resolveExplicitCaseFiles([]string{filepath.Join(dir, "*.txt")})
	if err != nil {
		t.Fatalf("resolveExplicitCaseFiles returned error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("resolveExplicitCaseFiles returned %d files, want 2", len(got))
	}
	wantFirst, _ := filepath.Abs(first)
	wantSecond, _ := filepath.Abs(second)
	if got[0] != wantFirst || got[1] != wantSecond {
		t.Fatalf("resolveExplicitCaseFiles = %#v, want [%q %q]", got, wantFirst, wantSecond)
	}
}

func TestResolveExplicitCaseFilesRejectsProhibitedExtension(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sign.sh")
	if err := os.WriteFile(path, []byte("echo hi\n"), 0o644); err != nil {
		t.Fatalf("write sign.sh: %v", err)
	}

	_, err := resolveExplicitCaseFiles([]string{path})
	if err == nil || !strings.Contains(err.Error(), "prohibited extension") {
		t.Fatalf("resolveExplicitCaseFiles error = %v, want prohibited extension error", err)
	}
}

func TestResolveExplicitCaseFilesRejectsUnmatchedGlob(t *testing.T) {
	dir := t.TempDir()
	_, err := resolveExplicitCaseFiles([]string{filepath.Join(dir, "*.txt")})
	if err == nil || !strings.Contains(err.Error(), "matched no files") {
		t.Fatalf("resolveExplicitCaseFiles error = %v, want unmatched glob error", err)
	}
}

func TestResolveAttorneyInstructionsPathUsesExplicitFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "instructions.md")
	if err := os.WriteFile(path, []byte("be careful\n"), 0o644); err != nil {
		t.Fatalf("write instructions: %v", err)
	}

	got, err := resolveAttorneyInstructionsPath(path)
	if err != nil {
		t.Fatalf("resolveAttorneyInstructionsPath returned error: %v", err)
	}
	want, err := filepath.Abs(path)
	if err != nil {
		t.Fatalf("filepath.Abs: %v", err)
	}
	if got != want {
		t.Fatalf("resolveAttorneyInstructionsPath = %q, want %q", got, want)
	}
}

func TestResolveAttorneyInstructionsPathRejectsMissingFile(t *testing.T) {
	dir := t.TempDir()
	_, err := resolveAttorneyInstructionsPath(filepath.Join(dir, "missing.md"))
	if err == nil || !strings.Contains(err.Error(), "stat attorney instructions") {
		t.Fatalf("resolveAttorneyInstructionsPath error = %v, want missing-file error", err)
	}
}

func TestResolvePromptDirUsesExplicitDirectory(t *testing.T) {
	dir := t.TempDir()
	got, err := resolvePromptDir(dir)
	if err != nil {
		t.Fatalf("resolvePromptDir returned error: %v", err)
	}
	want, err := filepath.Abs(dir)
	if err != nil {
		t.Fatalf("filepath.Abs: %v", err)
	}
	if got != want {
		t.Fatalf("resolvePromptDir = %q, want %q", got, want)
	}
}

func TestResolvePromptDirRejectsFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "prompt.md")
	if err := os.WriteFile(path, []byte("prompt"), 0o644); err != nil {
		t.Fatalf("write prompt: %v", err)
	}
	_, err := resolvePromptDir(path)
	if err == nil || !strings.Contains(err.Error(), "must be a directory") {
		t.Fatalf("resolvePromptDir error = %v, want directory error", err)
	}
}

func TestResolvePromptFileUsesExplicitFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "prompt.md")
	if err := os.WriteFile(path, []byte("prompt"), 0o644); err != nil {
		t.Fatalf("write prompt: %v", err)
	}
	got, err := resolvePromptFile("test prompt", path)
	if err != nil {
		t.Fatalf("resolvePromptFile returned error: %v", err)
	}
	want, err := filepath.Abs(path)
	if err != nil {
		t.Fatalf("filepath.Abs: %v", err)
	}
	if got != want {
		t.Fatalf("resolvePromptFile = %q, want %q", got, want)
	}
}

func TestResolvePromptFileRejectsDirectory(t *testing.T) {
	dir := t.TempDir()
	_, err := resolvePromptFile("test prompt", dir)
	if err == nil || !strings.Contains(err.Error(), "must be a file") {
		t.Fatalf("resolvePromptFile error = %v, want file error", err)
	}
}
