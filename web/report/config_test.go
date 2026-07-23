package report

import "testing"

func TestLoadConfigRejectsDuplicateRootPath(t *testing.T) {
	dir := t.TempDir()
	_, err := LoadConfig("", []Root{{Name: "a", Path: dir}, {Name: "b", Path: dir}}, "")
	if err == nil {
		t.Fatal("duplicate root path accepted")
	}
}
