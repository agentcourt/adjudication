package report

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Artifact readers tolerate absent files because failed or interrupted
// runs write only part of the artifact set.  Present files that fail to
// parse produce notes on the page instead of silent gaps.

type runFile struct {
	CaseID           string          `json:"case_id"`
	RunID            string          `json:"run_id"`
	StartedAt        string          `json:"started_at"`
	FinishedAt       string          `json:"finished_at"`
	Status           string          `json:"status"`
	Phase            string          `json:"phase"`
	Resolution       string          `json:"resolution"`
	FinalReason      string          `json:"final_reason"`
	EvidenceStandard string          `json:"evidence_standard"`
	CouncilBackend   string          `json:"council_backend"`
	Complaint        map[string]any  `json:"complaint"`
	Council          []CouncilMember `json:"council"`
	Attorneys        []Attorney      `json:"attorneys"`
}

type stateFile struct {
	SchemaVersion string `json:"schema_version"`
	ForumName     string `json:"forum_name"`
	Case          struct {
		CaseID       string          `json:"case_id"`
		Caption      string          `json:"caption"`
		Phase        string          `json:"phase"`
		Status       string          `json:"status"`
		Resolution   string          `json:"resolution"`
		Proposition  string          `json:"proposition"`
		Members      []CouncilMember `json:"council_members"`
		CouncilVotes []Vote          `json:"council_votes"`
		Failure      json.RawMessage `json:"failure"`
	} `json:"case"`
}

type localRunFile struct {
	Error   string          `json:"error"`
	Failure json.RawMessage `json:"failure"`
}

type certificateFile struct {
	SchemaVersion string `json:"schema_version"`
	Procedure     string `json:"procedure"`
}

type CouncilMember struct {
	MemberID    string `json:"member_id"`
	Model       string `json:"model"`
	PersonaFile string `json:"persona_file"`
}

type Attorney struct {
	Role      string `json:"role"`
	Interface string `json:"interface"`
}

type Vote struct {
	MemberID  string `json:"member_id"`
	Round     int    `json:"round"`
	Vote      string `json:"vote"`
	Rationale string `json:"rationale"`
}

// Summary is one row on the index page.
type Summary struct {
	RunDir
	CaseID          string
	System          string
	Status          string
	Phase           string
	Resolution      string
	StartedAt       string
	FinishedAt      string
	Duration        string
	DurationSeconds string
	VoteTally       string
	Note            string
}

// Label names the run for display: the case ID when known, otherwise
// the run directory path within its root.
func (s Summary) Label() string {
	if s.CaseID != "" {
		return s.CaseID
	}
	return s.Rel
}

// Summarize reads the run directory's artifacts and builds an index row.
func Summarize(rd RunDir) Summary {
	s := Summary{RunDir: rd}
	var notes []string

	var run runFile
	haveRun := readJSONFile(filepath.Join(rd.Abs, "run.json"), &run, &notes)
	var st stateFile
	haveState := readJSONFile(filepath.Join(rd.Abs, "state.json"), &st, &notes)
	var local localRunFile
	readJSONFile(filepath.Join(rd.Abs, "local-run.json"), &local, &notes)
	var cert certificateFile
	readJSONFile(filepath.Join(rd.Abs, "certificate.json"), &cert, &notes)

	s.CaseID = firstNonEmpty(run.CaseID, st.Case.CaseID)
	s.Status = firstNonEmpty(run.Status, st.Case.Status)
	s.Phase = firstNonEmpty(run.Phase, st.Case.Phase)
	s.Resolution = firstNonEmpty(run.Resolution, st.Case.Resolution)
	s.StartedAt = run.StartedAt
	s.FinishedAt = run.FinishedAt
	s.Duration, s.DurationSeconds = duration(run.StartedAt, run.FinishedAt)
	s.System = system(cert, st)
	s.VoteTally = tally(st.Case.CouncilVotes)
	if !haveRun && !haveState {
		s.Status = "incomplete"
	}
	if local.Error != "" {
		notes = append(notes, local.Error)
	}
	if len(st.Case.Failure) > 0 && string(st.Case.Failure) != "null" {
		notes = append(notes, "state.json records a failure")
	}
	s.Note = strings.Join(notes, "; ")
	return s
}

// readJSONFile decodes path into v and reports whether it did.  A
// missing file is normal; any other problem becomes a note.
func readJSONFile(path string, v any, notes *[]string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			*notes = append(*notes, err.Error())
		}
		return false
	}
	if err := json.Unmarshal(data, v); err != nil {
		*notes = append(*notes, fmt.Sprintf("%s: %v", filepath.Base(path), err))
		return false
	}
	return true
}

func system(cert certificateFile, st stateFile) string {
	if cert.Procedure != "" {
		return cert.Procedure
	}
	if i := strings.Index(st.SchemaVersion, "."); i > 0 {
		return st.SchemaVersion[:i]
	}
	return ""
}

func tally(votes []Vote) string {
	if len(votes) == 0 {
		return ""
	}
	counts := map[string]int{}
	for _, v := range votes {
		counts[v.Vote]++
	}
	keys := make([]string, 0, len(counts))
	for k := range counts {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if counts[keys[i]] != counts[keys[j]] {
			return counts[keys[i]] > counts[keys[j]]
		}
		return keys[i] < keys[j]
	})
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s %d", k, counts[k]))
	}
	return strings.Join(parts, ", ")
}

func duration(started, finished string) (display, seconds string) {
	a, errA := time.Parse(time.RFC3339, started)
	b, errB := time.Parse(time.RFC3339, finished)
	if errA != nil || errB != nil || b.Before(a) {
		return "", ""
	}
	d := b.Sub(a).Round(time.Second)
	return d.String(), fmt.Sprintf("%d", int(d.Seconds()))
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

// Event is one parsed line of events.ndjson.
type Event struct {
	Timestamp string          `json:"timestamp"`
	Turn      int             `json:"turn"`
	Role      string          `json:"role"`
	Phase     string          `json:"phase"`
	Type      string          `json:"type"`
	Payload   json.RawMessage `json:"payload"`
	Preview   string          `json:"-"`
}

const eventPreviewRunes = 240

// readEvents parses events.ndjson.  A malformed line becomes an event
// whose preview carries the parse error, so bad lines stay visible.
func readEvents(path string) ([]Event, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()
	var events []Event
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 1<<20), 64<<20)
	line := 0
	for sc.Scan() {
		line++
		text := strings.TrimSpace(sc.Text())
		if text == "" {
			continue
		}
		var ev Event
		if err := json.Unmarshal([]byte(text), &ev); err != nil {
			events = append(events, Event{Preview: fmt.Sprintf("line %d: %v", line, err)})
			continue
		}
		ev.Preview = truncateRunes(compactJSON(ev.Payload), eventPreviewRunes)
		events = append(events, ev)
	}
	if err := sc.Err(); err != nil {
		return events, err
	}
	return events, nil
}

func compactJSON(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var buf bytes.Buffer
	if err := json.Compact(&buf, raw); err != nil {
		return string(raw)
	}
	return buf.String()
}

func truncateRunes(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n]) + " [truncated]"
}

// FileEntry is one file inside a run directory listing.
type FileEntry struct {
	Rel      string // slash path relative to the run directory
	Size     int64
	Modified string
	ViewHref string
	RawHref  string
}

// listFiles walks the run directory and returns every regular file.
// Unreadable subdirectories become error strings.
func listFiles(runAbs string) ([]FileEntry, []string) {
	var entries []FileEntry
	var problems []string
	err := filepath.WalkDir(runAbs, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			problems = append(problems, err.Error())
			return nil
		}
		if !d.Type().IsRegular() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			problems = append(problems, err.Error())
			return nil
		}
		rel, err := filepath.Rel(runAbs, p)
		if err != nil {
			problems = append(problems, err.Error())
			return nil
		}
		entries = append(entries, FileEntry{
			Rel:      filepath.ToSlash(rel),
			Size:     info.Size(),
			Modified: info.ModTime().UTC().Format(time.RFC3339),
		})
		return nil
	})
	if err != nil {
		problems = append(problems, err.Error())
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Rel < entries[j].Rel })
	return entries, problems
}

// Detail is the full run page model.
type Detail struct {
	Summary
	RunID            string
	FinalReason      string
	Caption          string
	Proposition      string
	EvidenceStandard string
	CouncilBackend   string
	Complaint        [][2]string
	Council          []CouncilMember
	Attorneys        []Attorney
	Votes            []Vote
	Events           []Event
	EventsErr        string
	Files            []FileEntry
	FileProblems     []string
	Failure          string
}

// LoadDetail reads all artifacts the run page renders.
func LoadDetail(rd RunDir) Detail {
	d := Detail{Summary: Summarize(rd)}
	var notes []string

	var run runFile
	readJSONFile(filepath.Join(rd.Abs, "run.json"), &run, &notes)
	var st stateFile
	readJSONFile(filepath.Join(rd.Abs, "state.json"), &st, &notes)

	d.RunID = run.RunID
	d.FinalReason = run.FinalReason
	d.Caption = st.Case.Caption
	d.EvidenceStandard = run.EvidenceStandard
	d.CouncilBackend = run.CouncilBackend
	d.Proposition = st.Case.Proposition
	for _, k := range sortedKeys(run.Complaint) {
		d.Complaint = append(d.Complaint, [2]string{k, fmt.Sprintf("%v", run.Complaint[k])})
	}
	d.Council = run.Council
	if len(d.Council) == 0 {
		d.Council = st.Case.Members
	}
	d.Attorneys = run.Attorneys
	d.Votes = st.Case.CouncilVotes
	if len(st.Case.Failure) > 0 && string(st.Case.Failure) != "null" {
		d.Failure = prettyJSON(st.Case.Failure)
	}

	eventsPath := filepath.Join(rd.Abs, "events.ndjson")
	if info, err := os.Stat(eventsPath); err == nil && info.Size() > viewMaxBytes {
		d.EventsErr = fmt.Sprintf("%d bytes, above the %d-byte parse limit. Open it through the file table below.", info.Size(), viewMaxBytes)
	} else {
		events, err := readEvents(eventsPath)
		d.Events = events
		if err != nil {
			d.EventsErr = err.Error()
		}
	}
	d.Files, d.FileProblems = listFiles(rd.Abs)
	for i := range d.Files {
		full := path.Join(rd.Rel, d.Files[i].Rel)
		d.Files[i].ViewHref = "/view/" + rd.Root.Name + "/" + full
		d.Files[i].RawHref = "/raw/" + rd.Root.Name + "/" + full
	}
	return d
}

// RunHref is the run page URL.
func (s Summary) RunHref() string {
	return "/run/" + s.Root.Name + "/" + s.Rel
}

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func prettyJSON(raw []byte) string {
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return string(raw)
	}
	out, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return string(raw)
	}
	return string(out)
}
