package manage

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"sort"
	"time"
)

// kindAPI maps a UI kind to its service collection path and stop verb.
func kindAPI(kind string) (base, stopVerb string, ok bool) {
	switch kind {
	case "clerk", "attested":
		return "/clerk/v1/cases", "kill", true
	case "direct":
		return "/api/v1/cases", "cancel", true
	}
	return "", "", false
}

type Server struct {
	cfg    Config
	client *Client
	tmpl   *template.Template
	mux    *http.ServeMux
}

func NewServer(cfg Config) (*Server, error) {
	if err := cfg.Finish(); err != nil {
		return nil, err
	}
	tmpl, err := template.New("").Funcs(template.FuncMap{
		"stateClass": stateClass,
	}).ParseFS(files, "templates/*.html")
	if err != nil {
		return nil, fmt.Errorf("parse templates: %w", err)
	}
	s := &Server{cfg: cfg, client: NewClient(cfg.ARBURL, cfg.ARBToken), tmpl: tmpl, mux: http.NewServeMux()}
	s.mux.HandleFunc("GET /{$}", s.overview)
	s.mux.HandleFunc("GET /clerk", s.listPage("clerk"))
	s.mux.HandleFunc("GET /direct", s.listPage("direct"))
	s.mux.HandleFunc("GET /clerk/{id}", s.casePage("clerk"))
	s.mux.HandleFunc("GET /direct/{id}", s.casePage("direct"))
	s.mux.HandleFunc("POST /clerk/{id}/kill", s.stopCase("clerk"))
	s.mux.HandleFunc("POST /direct/{id}/cancel", s.stopCase("direct"))
	s.mux.HandleFunc("GET /start", s.startForm)
	s.mux.HandleFunc("POST /start", s.startSubmit)
	s.mux.HandleFunc("GET /raw", s.rawForm)
	s.mux.HandleFunc("POST /raw", s.rawSubmit)
	s.mux.Handle("GET /static/", http.FileServerFS(files))
	return s, nil
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost && !sameOriginPost(r) {
		http.Error(w, "cross-origin request rejected", http.StatusForbidden)
		return
	}
	s.mux.ServeHTTP(w, r)
}

// sameOriginPost reports whether a POST comes from this UI's own pages
// or from a non-browser client.  Browsers mark cross-site senders with
// Sec-Fetch-Site or an Origin naming another host; every POST route
// changes state, so those senders are rejected.
func sameOriginPost(r *http.Request) bool {
	switch r.Header.Get("Sec-Fetch-Site") {
	case "", "same-origin", "none":
	default:
		return false
	}
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}
	return u.Host == r.Host
}

func stateClass(value string) string {
	switch value {
	case "completed", "ok":
		return "state-good"
	case "running", "starting", "killing", "canceling":
		return "state-active"
	case "failed", "error":
		return "state-bad"
	case "killed", "canceled", "cancelled":
		return "state-muted"
	}
	return "state-plain"
}

type page struct {
	Title   string
	Heading string
	Refresh int
	ARBURL  string
	Now     string
	Error   string
}

func (s *Server) newPage(title string) page {
	return page{Title: title, Heading: title, ARBURL: s.cfg.ARBURL, Now: time.Now().UTC().Format(time.RFC3339)}
}

func (s *Server) render(w http.ResponseWriter, name string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.tmpl.ExecuteTemplate(w, name, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// caseRow is one list entry built from a service record.
type caseRow struct {
	CaseID     string
	Kind       string
	Status     string
	CreatedAt  string
	StartedAt  string
	FinishedAt string
	OutDir     string
	Error      string
	ManageHref string
	ReportHref string
}

func str(m map[string]any, key string) string {
	v, _ := m[key].(string)
	return v
}

func (s *Server) row(kind string, rec map[string]any) caseRow {
	id := str(rec, "case_id")
	row := caseRow{
		CaseID:     id,
		Kind:       kind,
		Status:     str(rec, "status"),
		CreatedAt:  str(rec, "created_at"),
		StartedAt:  str(rec, "started_at"),
		FinishedAt: str(rec, "finished_at"),
		OutDir:     str(rec, "out_dir"),
		Error:      str(rec, "error"),
		ManageHref: "/" + kind + "/" + url.PathEscape(id),
		ReportHref: s.cfg.ReportLink(str(rec, "out_dir")),
	}
	if exec, ok := rec["execution"].(map[string]any); ok && str(exec, "mode") == "attested" {
		row.Kind = "attested"
	}
	return row
}

// fetchRows lists one collection, newest first.
func (s *Server) fetchRows(kind, status string) ([]caseRow, error) {
	base, _, _ := kindAPI(kind)
	path := base
	if status != "" {
		path += "?status=" + url.QueryEscape(status)
	}
	httpStatus, payload, err := s.client.Call(http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	if msg := EnvelopeError(httpStatus, payload); msg != "" {
		return nil, fmt.Errorf("%s", msg)
	}
	var rows []caseRow
	if cases, ok := payload["cases"].([]any); ok {
		for _, c := range cases {
			if rec, ok := c.(map[string]any); ok {
				rows = append(rows, s.row(kind, rec))
			}
		}
	}
	sort.SliceStable(rows, func(i, j int) bool { return rows[i].CreatedAt > rows[j].CreatedAt })
	return rows, nil
}

type statusCount struct {
	Status string
	Count  int
}

func countByStatus(rows []caseRow) []statusCount {
	counts := map[string]int{}
	for _, r := range rows {
		counts[r.Status]++
	}
	keys := make([]string, 0, len(counts))
	for k := range counts {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]statusCount, 0, len(keys))
	for _, k := range keys {
		out = append(out, statusCount{Status: k, Count: counts[k]})
	}
	return out
}

// rowsBlock feeds the shared rows template with a unique table id.
type rowsBlock struct {
	ID   string
	Rows []caseRow
}

type overviewPage struct {
	page
	Clerk        rowsBlock
	ClerkCounts  []statusCount
	ClerkErr     string
	Direct       rowsBlock
	DirectCounts []statusCount
	DirectErr    string
}

const overviewRows = 15

func (s *Server) overview(w http.ResponseWriter, r *http.Request) {
	p := overviewPage{page: s.newPage("arb management")}
	p.Refresh = 30
	clerk, err := s.fetchRows("clerk", "")
	if err != nil {
		p.ClerkErr = err.Error()
	}
	p.ClerkCounts = countByStatus(clerk)
	if len(clerk) > overviewRows {
		clerk = clerk[:overviewRows]
	}
	p.Clerk = rowsBlock{ID: "clerk", Rows: clerk}
	direct, err := s.fetchRows("direct", "")
	if err != nil {
		p.DirectErr = err.Error()
	}
	p.DirectCounts = countByStatus(direct)
	if len(direct) > overviewRows {
		direct = direct[:overviewRows]
	}
	p.Direct = rowsBlock{ID: "direct", Rows: direct}
	s.render(w, "index", p)
}

type listPageData struct {
	page
	Kind   string
	Status string
	Block  rowsBlock
}

func (s *Server) listPage(kind string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p := listPageData{page: s.newPage(kind + " cases"), Kind: kind, Status: r.URL.Query().Get("status")}
		p.Refresh = 30
		rows, err := s.fetchRows(kind, p.Status)
		if err != nil {
			p.Error = err.Error()
		}
		p.Block = rowsBlock{ID: kind, Rows: rows}
		s.render(w, "cases", p)
	}
}

// recordRow is one key of a service record.  Scalar values render as
// text; composite values render as pretty JSON behind a disclosure.
type recordRow struct {
	Key    string
	Text   string
	Pretty string
}

func recordRows(rec map[string]any) []recordRow {
	keys := make([]string, 0, len(rec))
	for k := range rec {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	rows := make([]recordRow, 0, len(keys))
	for _, k := range keys {
		switch v := rec[k].(type) {
		case map[string]any, []any:
			data, err := json.MarshalIndent(v, "", "  ")
			if err != nil {
				data = []byte(fmt.Sprintf("%v", v))
			}
			rows = append(rows, recordRow{Key: k, Pretty: string(data)})
		case nil:
			rows = append(rows, recordRow{Key: k, Text: "null"})
		default:
			rows = append(rows, recordRow{Key: k, Text: fmt.Sprintf("%v", v)})
		}
	}
	return rows
}

type casePageData struct {
	page
	Kind              string
	CaseID            string
	Status            string
	Rows              []recordRow
	CanStop           bool
	StopVerb          string
	StopHref          string
	ReportHref        string
	Result            string
	AttestationEvents string
	Notice            string
}

// activeStatus lists the statuses under which a case page keeps
// refreshing: the run is changing, including while a stop is under way.
func activeStatus(status string) bool {
	switch status {
	case "starting", "running", "killing", "canceling":
		return true
	}
	return false
}

func (s *Server) casePage(kind string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		base, stopVerb, _ := kindAPI(kind)
		p := casePageData{page: s.newPage(kind + " " + id), Kind: kind, CaseID: id, StopVerb: stopVerb}
		p.Notice = r.URL.Query().Get("notice")
		httpStatus, payload, err := s.client.Call(http.MethodGet, base+"/"+url.PathEscape(id), nil)
		if err != nil {
			p.Error = err.Error()
			s.render(w, "case", p)
			return
		}
		if msg := EnvelopeError(httpStatus, payload); msg != "" {
			p.Error = msg
			s.render(w, "case", p)
			return
		}
		rec, _ := payload["case"].(map[string]any)
		p.Status = str(rec, "status")
		p.Rows = recordRows(rec)
		p.CanStop = p.Status == "running" || p.Status == "starting"
		p.StopHref = "/" + kind + "/" + url.PathEscape(id) + "/" + stopVerb
		p.ReportHref = s.cfg.ReportLink(str(rec, "out_dir"))
		if activeStatus(p.Status) {
			p.Refresh = 10
		}
		if p.Status == "completed" || p.Status == "failed" {
			if st, res, err := s.client.Call(http.MethodGet, base+"/"+url.PathEscape(id)+"/result", nil); err == nil && st == http.StatusOK {
				if data, err := json.MarshalIndent(res, "", "  "); err == nil {
					p.Result = string(data)
				}
			}
		}
		if exec, ok := rec["execution"].(map[string]any); ok && str(exec, "mode") == "attested" {
			if st, ev, err := s.client.Call(http.MethodGet, base+"/"+url.PathEscape(id)+"/attestation/events", nil); err == nil && st == http.StatusOK {
				if data, err := json.MarshalIndent(ev, "", "  "); err == nil {
					p.AttestationEvents = string(data)
				}
			}
		}
		s.render(w, "case", p)
	}
}

func (s *Server) stopCase(kind string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		base, stopVerb, _ := kindAPI(kind)
		httpStatus, payload, err := s.client.Call(http.MethodPost, base+"/"+url.PathEscape(id)+"/"+stopVerb, nil)
		notice := stopVerb + " sent"
		if err != nil {
			notice = err.Error()
		} else if msg := EnvelopeError(httpStatus, payload); msg != "" {
			notice = msg
		}
		http.Redirect(w, r, "/"+kind+"/"+url.PathEscape(id)+"?notice="+url.QueryEscape(notice), http.StatusSeeOther)
	}
}

type startPageData struct {
	page
	Kind     string
	Kinds    []string
	Groups   []FieldGroup
	Values   url.Values
	Problems []string
}

func (s *Server) startForm(w http.ResponseWriter, r *http.Request) {
	kind := r.URL.Query().Get("kind")
	if _, _, ok := kindAPI(kind); !ok {
		kind = "clerk"
	}
	p := startPageData{page: s.newPage("start " + kind + " case"), Kind: kind, Kinds: Kinds, Groups: GroupsFor(kind), Values: url.Values{}}
	s.render(w, "start", p)
}

func (s *Server) startSubmit(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	kind := r.PostFormValue("kind")
	base, _, ok := kindAPI(kind)
	if !ok {
		http.Error(w, "unknown kind", http.StatusBadRequest)
		return
	}
	payload, problems := BuildPayload(kind, r.PostFormValue)
	p := startPageData{page: s.newPage("start " + kind + " case"), Kind: kind, Kinds: Kinds, Groups: GroupsFor(kind), Values: r.PostForm}
	if len(problems) > 0 {
		p.Problems = problems
		s.render(w, "start", p)
		return
	}
	body, err := json.Marshal(payload)
	if err != nil {
		p.Error = err.Error()
		s.render(w, "start", p)
		return
	}
	httpStatus, resp, err := s.client.Call(http.MethodPost, base, body)
	if err != nil {
		p.Error = err.Error()
		s.render(w, "start", p)
		return
	}
	if msg := EnvelopeError(httpStatus, resp); msg != "" {
		p.Error = msg
		s.render(w, "start", p)
		return
	}
	rec, _ := resp["case"].(map[string]any)
	id := str(rec, "case_id")
	if id == "" {
		p.Error = "service accepted the case but returned no case_id"
		s.render(w, "start", p)
		return
	}
	kindPath := kind
	if kind == "attested" {
		kindPath = "clerk"
	}
	http.Redirect(w, r, "/"+kindPath+"/"+url.PathEscape(id), http.StatusSeeOther)
}

type rawPageData struct {
	page
	Target   string
	Payload  string
	Response string
	Status   int
}

const rawTemplatePayload = "{\n}\n"

func (s *Server) rawForm(w http.ResponseWriter, r *http.Request) {
	p := rawPageData{page: s.newPage("raw create request"), Target: "clerk", Payload: rawTemplatePayload}
	s.render(w, "raw", p)
}

func (s *Server) rawSubmit(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	target := r.PostFormValue("target")
	base, _, ok := kindAPI(target)
	if !ok {
		http.Error(w, "unknown target", http.StatusBadRequest)
		return
	}
	payload := r.PostFormValue("payload")
	p := rawPageData{page: s.newPage("raw create request"), Target: target, Payload: payload}
	httpStatus, resp, err := s.client.Call(http.MethodPost, base, []byte(payload))
	if err != nil {
		p.Error = err.Error()
		s.render(w, "raw", p)
		return
	}
	p.Status = httpStatus
	if data, err := json.MarshalIndent(resp, "", "  "); err == nil {
		p.Response = string(data)
	}
	s.render(w, "raw", p)
}
