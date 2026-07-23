package report

import (
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// markerNames identify a directory as a run output directory.  The
// scanner stops descending once it finds one, so artifacts inside a run
// directory never register as further runs.
var markerNames = []string{
	"run.json",
	"state.json",
	"local-run.json",
	"events.ndjson",
	"certificate.json",
	"transcript.md",
	"digest.md",
	"aar-stdout.log",
	"aar-stderr.log",
}

const maxScanDepth = 16

// piHomePattern matches Pi agent home directories such as pi-C1.  They
// hold container caches and extension trees rather than run outputs.
var piHomePattern = regexp.MustCompile(`^pi-C[0-9]+$`)

// skipDir reports directories the scanner does not enter.
func skipDir(name string) bool {
	return strings.HasPrefix(name, ".") || piHomePattern.MatchString(name)
}

// RunDir is a discovered run output directory.
type RunDir struct {
	Root Root
	Rel  string // slash-separated path relative to Root.Path, "." for the root itself
	Abs  string
}

// ScanProblem records a directory the scanner could not examine.
type ScanProblem struct {
	Root Root
	Path string
	Err  string
}

// ScanRoots walks each root and returns run directories and scan
// problems.  It skips hidden directories, Pi agent homes, and symbolic
// links, and it does not descend into discovered run directories.
func ScanRoots(roots []Root) ([]RunDir, []ScanProblem) {
	var runs []RunDir
	var problems []ScanProblem
	for _, rt := range roots {
		scanDir(rt, rt.Path, ".", 0, &runs, &problems)
	}
	sort.Slice(runs, func(i, j int) bool {
		if runs[i].Root.Name != runs[j].Root.Name {
			return runs[i].Root.Name < runs[j].Root.Name
		}
		return runs[i].Rel < runs[j].Rel
	})
	return runs, problems
}

func scanDir(rt Root, abs, rel string, depth int, runs *[]RunDir, problems *[]ScanProblem) {
	entries, err := os.ReadDir(abs)
	if err != nil {
		*problems = append(*problems, ScanProblem{Root: rt, Path: abs, Err: err.Error()})
		return
	}
	files := map[string]bool{}
	for _, e := range entries {
		if e.Type().IsRegular() {
			files[e.Name()] = true
		}
	}
	for _, m := range markerNames {
		if files[m] {
			*runs = append(*runs, RunDir{Root: rt, Rel: rel, Abs: abs})
			return
		}
	}
	if depth >= maxScanDepth {
		*problems = append(*problems, ScanProblem{Root: rt, Path: abs, Err: "depth limit reached; not scanned"})
		return
	}
	for _, e := range entries {
		if !e.IsDir() || skipDir(e.Name()) {
			continue
		}
		scanDir(rt, filepath.Join(abs, e.Name()), path.Join(rel, e.Name()), depth+1, runs, problems)
	}
}

// isRunDir reports whether the directory holds a run marker file.
func isRunDir(abs string) bool {
	for _, m := range markerNames {
		if info, err := os.Stat(filepath.Join(abs, m)); err == nil && info.Mode().IsRegular() {
			return true
		}
	}
	return false
}
