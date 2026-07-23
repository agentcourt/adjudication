package report

import (
	"fmt"
	"html/template"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

// viewMaxBytes bounds rendered file views.  Larger files stay reachable
// through the raw route, which serves byte ranges.
const viewMaxBytes = 8 << 20

type Server struct {
	cfg   Config
	roots map[string]Root
	tmpl  *template.Template
	mux   *http.ServeMux
}

func NewServer(cfg Config) (*Server, error) {
	tmpl, err := template.New("").Funcs(template.FuncMap{
		"stateClass": stateClass,
	}).ParseFS(files, "templates/*.html")
	if err != nil {
		return nil, fmt.Errorf("parse templates: %w", err)
	}
	s := &Server{cfg: cfg, roots: map[string]Root{}, tmpl: tmpl, mux: http.NewServeMux()}
	for _, rt := range cfg.Roots {
		s.roots[rt.Name] = rt
	}
	s.mux.HandleFunc("GET /{$}", s.index)
	s.mux.HandleFunc("GET /run/{root}/{path...}", s.run)
	s.mux.HandleFunc("GET /view/{root}/{path...}", s.view)
	s.mux.HandleFunc("GET /raw/{root}/{path...}", s.raw)
	s.mux.Handle("GET /static/", http.FileServerFS(files))
	return s, nil
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

// stateClass maps a status or resolution value to a display class.
func stateClass(value string) string {
	switch value {
	case "ok", "demonstrated", "completed":
		return "state-good"
	case "running", "active", "open":
		return "state-active"
	case "failed", "error":
		return "state-bad"
	case "not_demonstrated", "closed":
		return "state-cool"
	case "incomplete", "":
		return "state-muted"
	}
	return "state-plain"
}

// resolve maps a URL path element to an absolute path confined to the
// root.  Symbolic links resolve first, so a link pointing outside the
// root stays unreachable.
func (s *Server) resolve(rootName, rel string) (Root, string, error) {
	rt, ok := s.roots[rootName]
	if !ok {
		return Root{}, "", fmt.Errorf("unknown root %q", rootName)
	}
	if rel == "" {
		rel = "."
	}
	if rel != "." && !filepath.IsLocal(filepath.FromSlash(rel)) {
		return Root{}, "", fmt.Errorf("path %q is not inside the root", rel)
	}
	abs := filepath.Join(rt.Path, filepath.FromSlash(rel))
	realAbs, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return Root{}, "", err
	}
	realRoot, err := filepath.EvalSymlinks(rt.Path)
	if err != nil {
		return Root{}, "", err
	}
	if realAbs != realRoot && !strings.HasPrefix(realAbs, realRoot+string(filepath.Separator)) {
		return Root{}, "", fmt.Errorf("path %q leaves the root", rel)
	}
	return rt, abs, nil
}

type crumb struct {
	Text string
	Href string
}

type page struct {
	Title   string
	Heading string
	Refresh int
	Crumbs  []crumb
	Now     string
}

func newPage(title string) page {
	return page{Title: title, Heading: title, Now: time.Now().UTC().Format(time.RFC3339)}
}

func (s *Server) render(w http.ResponseWriter, name string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.tmpl.ExecuteTemplate(w, name, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

type indexPage struct {
	page
	RootFilters []crumb
	Filter      string
	Runs        []Summary
	Problems    []ScanProblem
}

func (s *Server) index(w http.ResponseWriter, r *http.Request) {
	filter := r.URL.Query().Get("root")
	roots := s.cfg.Roots
	if filter != "" {
		rt, ok := s.roots[filter]
		if !ok {
			http.Error(w, "unknown root", http.StatusNotFound)
			return
		}
		roots = []Root{rt}
	}
	dirs, problems := ScanRoots(roots)
	p := indexPage{page: newPage("adjudication runs"), Filter: filter, Problems: problems}
	p.Refresh = 30
	for _, rt := range s.cfg.Roots {
		p.RootFilters = append(p.RootFilters, crumb{Text: rt.Name, Href: "/?root=" + rt.Name})
	}
	for _, rd := range dirs {
		p.Runs = append(p.Runs, Summarize(rd))
	}
	sort.SliceStable(p.Runs, func(i, j int) bool { return p.Runs[i].StartedAt > p.Runs[j].StartedAt })
	s.render(w, "index", p)
}

type runPage struct {
	page
	Detail
}

func (s *Server) run(w http.ResponseWriter, r *http.Request) {
	rt, abs, err := s.resolve(r.PathValue("root"), r.PathValue("path"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	info, err := os.Stat(abs)
	if err != nil || !info.IsDir() {
		http.NotFound(w, r)
		return
	}
	rel := path.Clean(filepath.ToSlash(r.PathValue("path")))
	d := LoadDetail(RunDir{Root: rt, Rel: rel, Abs: abs})
	p := runPage{page: newPage(d.Label()), Detail: d}
	p.Crumbs = []crumb{{Text: "runs", Href: "/"}, {Text: rt.Name, Href: "/?root=" + rt.Name}, {Text: rel}}
	if d.Status == "running" || d.Status == "active" {
		p.Refresh = 15
	}
	s.render(w, "run", p)
}

// fileView selects how the view page presents a file.
type fileView struct {
	page
	Root     Root
	Rel      string // slash path relative to the root
	RunHref  string // run page for the nearest enclosing run directory
	RawHref  string
	Mode     string // markdown, text, json, ndjson, binary, toolarge, dir
	AsText   bool
	Links    []crumb // view toggles; a link without Href is the current view
	Rendered template.HTML
	Text     string
	Records  []ndjsonRecord
	Size     int64
	Entries  []dirEntry
}

type ndjsonRecord struct {
	Line    int
	Summary string
	Pretty  string
}

type dirEntry struct {
	Name  string
	IsDir bool
	Size  int64
	Href  string
}

func (s *Server) view(w http.ResponseWriter, r *http.Request) {
	rootName, rel := r.PathValue("root"), r.PathValue("path")
	rt, abs, err := s.resolve(rootName, rel)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	rel = path.Clean(filepath.ToSlash(rel))
	info, err := os.Stat(abs)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	v := fileView{page: newPage(rt.Name + "/" + rel), Root: rt, Rel: rel}
	v.Heading = path.Base(abs)
	v.Crumbs = s.fileCrumbs(rt, rel, info.IsDir())
	v.RunHref = s.runHrefFor(rt, rel)
	if info.IsDir() {
		v.Mode = "dir"
		entries, err := os.ReadDir(abs)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		for _, e := range entries {
			de := dirEntry{Name: e.Name(), IsDir: e.IsDir()}
			if fi, err := e.Info(); err == nil && !e.IsDir() {
				de.Size = fi.Size()
			}
			de.Href = "/view/" + rt.Name + "/" + path.Join(rel, e.Name())
			v.Entries = append(v.Entries, de)
		}
		s.render(w, "file", v)
		return
	}
	v.RawHref = "/raw/" + rt.Name + "/" + rel
	v.Size = info.Size()
	if info.Size() > viewMaxBytes {
		v.Mode = "toolarge"
		s.render(w, "file", v)
		return
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	v.AsText = r.URL.Query().Get("as") == "text"
	altLabel := ""
	switch {
	case strings.HasSuffix(rel, ".md"):
		altLabel = "rendered"
	case strings.HasSuffix(rel, ".json"), strings.HasSuffix(rel, ".ndjson"):
		altLabel = "formatted"
	}
	switch {
	case strings.HasSuffix(rel, ".md") && !v.AsText:
		v.Mode = "markdown"
		v.Rendered = RenderMarkdown(data)
	case strings.HasSuffix(rel, ".ndjson") && !v.AsText:
		v.Mode = "ndjson"
		v.Records = ndjsonRecords(data)
	case strings.HasSuffix(rel, ".json") && !v.AsText:
		v.Mode = "json"
		v.Text = prettyJSON(data)
	case utf8.Valid(data):
		v.Mode = "text"
		v.Text = string(data)
	default:
		v.Mode = "binary"
	}
	switch {
	case altLabel != "" && v.AsText:
		v.Links = []crumb{{Text: altLabel, Href: "?"}, {Text: "text"}, {Text: "raw", Href: v.RawHref}}
	case altLabel != "":
		v.Links = []crumb{{Text: altLabel}, {Text: "text", Href: "?as=text"}, {Text: "raw", Href: v.RawHref}}
	default:
		v.Links = []crumb{{Text: "raw", Href: v.RawHref}}
	}
	s.render(w, "file", v)
}

// fileCrumbs builds the breadcrumb for a file or directory view.
func (s *Server) fileCrumbs(rt Root, rel string, isDir bool) []crumb {
	crumbs := []crumb{{Text: "runs", Href: "/"}, {Text: rt.Name, Href: "/view/" + rt.Name + "/."}}
	if rel == "." {
		return crumbs
	}
	parts := strings.Split(rel, "/")
	for i, part := range parts {
		if i == len(parts)-1 && !isDir {
			crumbs = append(crumbs, crumb{Text: part})
			continue
		}
		crumbs = append(crumbs, crumb{Text: part, Href: "/view/" + rt.Name + "/" + strings.Join(parts[:i+1], "/")})
	}
	return crumbs
}

// runHrefFor returns the run page URL for the nearest enclosing run
// directory, or "" when no ancestor is a run directory.
func (s *Server) runHrefFor(rt Root, rel string) string {
	dir := rel
	for {
		if isRunDir(filepath.Join(rt.Path, filepath.FromSlash(dir))) {
			return "/run/" + rt.Name + "/" + dir
		}
		if dir == "." {
			return ""
		}
		dir = path.Dir(dir)
	}
}

const ndjsonSummaryRunes = 160

func ndjsonRecords(data []byte) []ndjsonRecord {
	var records []ndjsonRecord
	for i, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		records = append(records, ndjsonRecord{
			Line:    i + 1,
			Summary: truncateRunes(line, ndjsonSummaryRunes),
			Pretty:  prettyJSON([]byte(line)),
		})
	}
	return records
}

func (s *Server) raw(w http.ResponseWriter, r *http.Request) {
	_, abs, err := s.resolve(r.PathValue("root"), r.PathValue("path"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	info, err := os.Stat(abs)
	if err != nil || !info.Mode().IsRegular() {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if strings.HasSuffix(abs, ".ndjson") || strings.HasSuffix(abs, ".log") {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	}
	http.ServeFile(w, r, abs)
}
