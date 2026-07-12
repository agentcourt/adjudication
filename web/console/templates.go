package console

const pageTemplates = `
{{define "layout-start"}}<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  {{if .AutoRefresh}}<meta http-equiv="refresh" content="10">{{end}}
  <title>{{.Title}}</title>
  <style>
    :root { color-scheme: light; --line:#b8b8b8; --ink:#111; --muted:#555; --back:#f5f5f2; --panel:#fff; }
    * { box-sizing: border-box; }
    body { margin:0; background:var(--back); color:var(--ink); font:14px/1.35 ui-monospace, SFMono-Regular, Menlo, Consolas, "Liberation Mono", monospace; }
    header { border-bottom:1px solid var(--line); background:#e8e8e4; padding:10px 14px; display:flex; gap:18px; align-items:center; flex-wrap:wrap; }
    header a { color:var(--ink); text-decoration:none; }
    main { padding:14px; max-width:1280px; }
    h1 { font-size:18px; margin:0 0 12px; }
    h2 { font-size:15px; margin:18px 0 8px; }
    table { border-collapse:collapse; width:100%; background:var(--panel); margin:8px 0 14px; }
    th, td { border:1px solid var(--line); padding:6px 8px; text-align:left; vertical-align:top; overflow-wrap:anywhere; }
    th { background:#ededeb; font-weight:700; }
    a { color:#111; text-decoration:underline; text-underline-offset:2px; }
    form { margin:8px 0 14px; }
    fieldset { border:1px solid var(--line); padding:10px; margin:0 0 12px; background:var(--panel); }
    legend { padding:0 6px; }
    label { display:block; margin:8px 0 4px; color:var(--muted); }
    input, textarea, select, button { font:inherit; border:1px solid #777; background:#fff; color:#111; padding:5px 7px; }
    textarea { width:100%; min-height:260px; resize:vertical; }
    button { background:#ddd; cursor:pointer; }
    .bar { display:flex; gap:8px; flex-wrap:wrap; align-items:center; }
    .pill { border:1px solid var(--line); padding:2px 6px; background:#fff; text-decoration:none; }
    .error { border:1px solid #8a1f1f; background:#fff3f3; padding:8px; margin:8px 0; white-space:pre-wrap; }
    .notice { border:1px solid #777; background:#fff; padding:8px; margin:8px 0; white-space:pre-wrap; }
    pre { border:1px solid var(--line); background:#fff; padding:10px; overflow:auto; max-height:520px; }
    .muted { color:var(--muted); }
    .grid { display:grid; grid-template-columns:repeat(auto-fit, minmax(260px, 1fr)); gap:12px; }
    .actions form { display:inline; margin-right:8px; }
    .record-facts { display:grid; grid-template-columns:max-content minmax(0, 1fr); gap:3px 10px; margin:0 0 8px; }
    .record-facts dt { color:var(--muted); }
    .record-facts dd { margin:0; overflow-wrap:anywhere; }
    .record-details summary { cursor:pointer; color:var(--muted); }
    .record-details pre { max-height:340px; margin:6px 0 0; }
  </style>
</head>
<body>
<header>
  <a href="/">Adjudication</a>
  {{range .Systems}}
    {{$sys := .}}
    {{range .Scopes}}<a class="pill" href="/system/{{$sys.ID}}/{{.ID}}/cases">{{$sys.Label}} {{.Label}}</a>{{end}}
    <a class="pill" href="/system/{{$sys.ID}}/request">{{$sys.Label}} request</a>
  {{end}}
</header>
<main>
{{if .Error}}<div class="error">{{.Error}}</div>{{end}}
{{if .Notice}}<div class="notice">{{.Notice}}</div>{{end}}
{{end}}

{{define "layout-end"}}</main></body></html>{{end}}

{{define "index"}}{{template "layout-start" .}}
<h1>Adjudication Service Console</h1>
<div class="grid">
{{range .Systems}}
  <section>
    <h2>{{.Label}}</h2>
    <table>
      <tr><th>Base URL</th><td>{{.BaseURL}}</td></tr>
      <tr><th>Bearer token</th><td>{{if .BearerToken}}configured{{else}}none{{end}}</td></tr>
    </table>
    <div class="bar">
      {{$sys := .}}
      {{range .Scopes}}
        <a class="pill" href="/system/{{$sys.ID}}/{{.ID}}/cases">{{.Label}} cases</a>
        <a class="pill" href="/system/{{$sys.ID}}/{{.ID}}/cases/new">create {{.Label}}</a>
      {{end}}
      <a class="pill" href="/system/{{.ID}}/request">service request</a>
    </div>
  </section>
{{end}}
</div>
{{template "layout-end" .}}{{end}}

{{define "cases"}}{{template "layout-start" .}}
<h1>{{.System.Label}} {{.Scope.Label}} Cases</h1>
<div class="bar">
  <a href="/system/{{.System.ID}}/{{.Scope.ID}}/cases/new">create</a>
  <a href="/system/{{.System.ID}}/{{.Scope.ID}}/cases">all</a>
  <a href="/system/{{.System.ID}}/{{.Scope.ID}}/cases?status=running">running</a>
  <a href="/system/{{.System.ID}}/{{.Scope.ID}}/cases?status=completed">completed</a>
  <a href="/system/{{.System.ID}}/{{.Scope.ID}}/cases?status=failed">failed</a>
</div>
{{if .Cases}}
<table>
  <tr><th>Case</th><th>Run</th><th>Status</th><th>Facts</th><th>Created</th><th>Finished</th><th>Example</th><th>Error</th></tr>
  {{range .Cases}}
  <tr>
    <td><a href="/system/{{$.System.ID}}/{{$.Scope.ID}}/cases/{{field . "case_id" | pathEscape}}">{{field . "case_id"}}</a></td>
    <td>{{field . "run_id"}}</td>
    <td>{{field . "status"}}</td>
    <td>{{caseFacts .}}</td>
    <td>{{field . "created_at"}}</td>
    <td>{{field . "finished_at"}}</td>
    <td>{{field . "example"}}</td>
    <td>{{field . "error"}}</td>
  </tr>
  {{end}}
</table>
{{else}}
<p class="muted">No cases returned.</p>
{{end}}
	{{if .Response}}<h2>Service Response</h2><pre>{{response .Response}}</pre>{{end}}
{{template "layout-end" .}}{{end}}

{{define "new"}}{{template "layout-start" .}}
<h1>Create {{.System.Label}} {{.Scope.Label}} Case</h1>
<form method="post" action="/system/{{.System.ID}}/{{.Scope.ID}}/cases">
  <fieldset>
    <legend>Request JSON</legend>
    <p class="muted">The payload is sent unchanged to {{.Scope.BasePath}} on {{.System.BaseURL}}.</p>
    <textarea name="payload" spellcheck="false">{{if .Payload}}{{.Payload}}{{else}}{{.CreateTemplate}}{{end}}</textarea>
  </fieldset>
  <button type="submit">Create case</button>
</form>
	{{if .Response}}<h2>Service Response</h2><pre>{{response .Response}}</pre>{{end}}
{{template "layout-end" .}}{{end}}

{{define "case"}}{{template "layout-start" .}}
<h1>{{.System.Label}} {{.Scope.Label}} Case {{.CaseID}}</h1>
{{if .CaseAvailable}}
<div class="actions">
  {{if .CanManage}}
  <form method="post" action="/system/{{.System.ID}}/{{.Scope.ID}}/cases/{{.CaseID | pathEscape}}/manage"><button type="submit">{{.Scope.ManageAction}}</button></form>
  {{end}}
  <a class="pill" href="/system/{{.System.ID}}/{{.Scope.ID}}/cases/{{.CaseID | pathEscape}}/result">result</a>
  <a class="pill" href="/system/{{.System.ID}}/{{.Scope.ID}}/cases/{{.CaseID | pathEscape}}/artifacts">artifacts</a>
  {{if .HasEvents}}
  <a class="pill" href="/system/{{.System.ID}}/{{.Scope.ID}}/cases/{{.CaseID | pathEscape}}/events">events</a>
  {{end}}
  <a class="pill" href="/system/{{.System.ID}}/{{.Scope.ID}}/cases/{{.CaseID | pathEscape}}/evidence">evidence</a>
  {{if .HasAttestationEvents}}
  <a class="pill" href="/system/{{.System.ID}}/{{.Scope.ID}}/cases/{{.CaseID | pathEscape}}/attestation/events">attestation events</a>
  {{end}}
</div>
{{end}}
{{if .Record}}
<h2>Record</h2>
<table>{{range keys .Record}}<tr><th>{{.}}</th><td>{{recordValue $.System $.Scope $.CaseID . (index $.Record .)}}</td></tr>{{end}}</table>
{{end}}
{{if .Artifacts}}
<h2>Artifacts</h2>
<table><tr><th>Name</th><th>Size</th><th>View</th></tr>{{range .Artifacts}}<tr><td><a href="/system/{{$.System.ID}}/{{$.Scope.ID}}/cases/{{$.CaseID | pathEscape}}/artifacts/{{.Name | pathEscape}}">{{.Name}}</a></td><td>{{.Size}}</td><td>{{if isLog .Name}}<a href="/system/{{$.System.ID}}/{{$.Scope.ID}}/cases/{{$.CaseID | pathEscape}}/log?name={{.Name | query}}">log</a>{{end}}</td></tr>{{end}}</table>
{{end}}
{{if .RecentEvents}}
<h2>Recent Events</h2>
<table>
  <tr><th>Time</th><th>Phase</th><th>Type</th><th>Actor</th><th>Message</th></tr>
  {{range .RecentEvents}}
  <tr>
    <td>{{.Timestamp}}</td>
    <td>{{.Phase}}</td>
    <td>{{.Type}}</td>
    <td>{{.Actor}}</td>
    <td>{{.Message}}</td>
  </tr>
  {{end}}
</table>
{{end}}
{{if .EventIssues}}
<h2>Failure Events</h2>
<table>
  <tr><th>Time</th><th>Phase</th><th>Type</th><th>Member</th><th>Process</th><th>Reason</th><th>Message</th><th>Log</th></tr>
  {{range .EventIssues}}
  <tr>
    <td>{{.Timestamp}}</td>
    <td>{{.Phase}}</td>
    <td>{{.Type}}</td>
    <td>{{.Member}}</td>
    <td>{{.Process}}</td>
    <td>{{.Reason}}</td>
    <td>{{.Message}}</td>
    <td>{{.LogPath}}</td>
  </tr>
  {{end}}
</table>
{{else if .EventNotice}}<p class="muted">{{.EventNotice}}</p>{{end}}
	{{if .Result}}<h2>Result</h2><pre>{{boundedJSON .Result}}</pre>{{end}}
	{{if .Response}}<h2>Case Response</h2><pre>{{response .Response}}</pre>{{end}}
{{template "layout-end" .}}{{end}}

{{define "artifacts"}}{{template "layout-start" .}}
<h1>{{.System.Label}} Artifacts {{.CaseID}}</h1>
{{if .Artifacts}}
<table><tr><th>Name</th><th>Size</th><th>View</th></tr>{{range .Artifacts}}<tr><td><a href="/system/{{$.System.ID}}/{{$.Scope.ID}}/cases/{{$.CaseID | pathEscape}}/artifacts/{{.Name | pathEscape}}">{{.Name}}</a></td><td>{{.Size}}</td><td>{{if isLog .Name}}<a href="/system/{{$.System.ID}}/{{$.Scope.ID}}/cases/{{$.CaseID | pathEscape}}/log?name={{.Name | query}}">log</a>{{end}}</td></tr>{{end}}</table>
{{else}}<p class="muted">No artifacts returned.</p>{{end}}
	{{if .Response}}<h2>Service Response</h2><pre>{{response .Response}}</pre>{{end}}
{{template "layout-end" .}}{{end}}

{{define "log"}}{{template "layout-start" .}}
<h1>{{.System.Label}} Log {{.CaseID}}</h1>
<div class="bar">
  <a class="pill" href="/system/{{.System.ID}}/{{.Scope.ID}}/cases/{{.CaseID | pathEscape}}">case</a>
  {{if .LogName}}<a class="pill" href="/system/{{.System.ID}}/{{.Scope.ID}}/cases/{{.CaseID | pathEscape}}/artifacts/{{.LogName | pathEscape}}">raw log</a>{{end}}
</div>
<form method="get" action="/system/{{.System.ID}}/{{.Scope.ID}}/cases/{{.CaseID | pathEscape}}/log">
  <fieldset>
    <legend>Log</legend>
    <label for="log-name">Artifact</label>
    <input id="log-name" name="name" value="{{.LogName}}" style="width:100%">
    <label for="log-mode">Mode</label>
    <select id="log-mode" name="mode">
      <option value="tail" {{if eq .LogMode "tail"}}selected{{end}}>tail</option>
      <option value="head" {{if eq .LogMode "head"}}selected{{end}}>head</option>
    </select>
    <label for="log-bytes">Bytes</label>
    <input id="log-bytes" name="bytes" value="{{.LogBytes}}">
    <button type="submit">Reload</button>
  </fieldset>
</form>
{{if .LogText}}<pre>{{.LogText}}</pre>{{else if .LogNotice}}<p class="muted">{{.LogNotice}}</p>{{end}}
{{template "layout-end" .}}{{end}}

{{define "events"}}{{template "layout-start" .}}
<h1>{{.System.Label}} Events {{.CaseID}}</h1>
<div class="bar">
  <a class="pill" href="/system/{{.System.ID}}/{{.Scope.ID}}/cases/{{.CaseID | pathEscape}}">case</a>
  <a class="pill" href="/system/{{.System.ID}}/{{.Scope.ID}}/cases/{{.CaseID | pathEscape}}/artifacts/events.ndjson">raw events.ndjson</a>
</div>
<form method="get" action="/system/{{.System.ID}}/{{.Scope.ID}}/cases/{{.CaseID | pathEscape}}/events">
  <fieldset>
    <legend>Tail</legend>
    <label for="event-limit">Events</label>
    <input id="event-limit" name="limit" value="{{.EventLimit}}">
    <label for="event-bytes">Bytes</label>
    <input id="event-bytes" name="bytes" value="{{.EventBytes}}">
    <button type="submit">Reload</button>
  </fieldset>
</form>
{{if .Events}}
<table>
  <tr><th>Offset</th><th>Time</th><th>Phase</th><th>Type</th><th>Actor</th><th>Message</th><th>Record</th></tr>
  {{range .Events}}
  <tr>
    <td>{{.Offset}}</td>
    <td>{{.Timestamp}}</td>
    <td>{{.Phase}}</td>
    <td>{{.Type}}</td>
    <td>{{.Actor}}</td>
    <td>{{.Message}}</td>
    <td><details class="record-details"><summary>JSON</summary><pre>{{boundedJSON .Raw}}</pre></details></td>
  </tr>
  {{end}}
</table>
{{else if .EventNotice}}<p class="muted">{{.EventNotice}}</p>{{end}}
{{template "layout-end" .}}{{end}}

{{define "evidence"}}{{template "layout-start" .}}
<h1>{{.System.Label}} Evidence {{.CaseID}}</h1>
{{if .Evidence}}
<h2>Evidence Manifest</h2>
<table>
  <tr><th>Evidence</th><th>Title</th><th>MIME</th><th>Size</th><th>Status</th><th>Visibility</th><th>Uses</th></tr>
  {{range .Evidence}}
  <tr>
    <td><a href="/system/{{$.System.ID}}/{{$.Scope.ID}}/cases/{{$.CaseID | pathEscape}}/evidence?id={{.ID | query}}">{{.ID}}</a></td>
    <td>{{.Title}}</td>
    <td>{{.MIMEType}}</td>
    <td>{{.Size}}</td>
    <td>{{.Status}}</td>
    <td>{{.Visibility}}</td>
    <td>{{.Uses}}</td>
  </tr>
  {{end}}
</table>
{{else if .EvidenceNotice}}<p class="muted">{{.EvidenceNotice}}</p>{{end}}
<form method="get" action="/system/{{.System.ID}}/{{.Scope.ID}}/cases/{{.CaseID | pathEscape}}/evidence">
  <label for="evidence-id">Evidence id</label>
  <input id="evidence-id" name="id" value="{{.EvidenceID}}">
  <button type="submit">Fetch</button>
</form>
{{if .EvidenceID}}<p><a href="/system/{{.System.ID}}/{{.Scope.ID}}/cases/{{.CaseID | pathEscape}}/evidence/{{.EvidenceID | pathEscape}}">open raw evidence response</a></p>{{end}}
	{{if .Response}}<h2>Service Response</h2><pre>{{response .Response}}</pre>{{end}}
{{template "layout-end" .}}{{end}}

{{define "response"}}{{template "layout-start" .}}
<h1>{{.Title}}</h1>
	{{if .Response}}<p>HTTP {{.Response.StatusCode}}</p><pre>{{response .Response}}</pre>{{end}}
{{template "layout-end" .}}{{end}}

{{define "request"}}{{template "layout-start" .}}
<h1>{{.System.Label}} Service Request</h1>
<form method="post" action="/system/{{.System.ID}}/request">
  <fieldset>
    <legend>HTTP</legend>
    <label for="method">Method</label>
    <select id="method" name="method">
      <option {{if eq .Method "GET"}}selected{{end}}>GET</option>
      <option {{if eq .Method "POST"}}selected{{end}}>POST</option>
    </select>
    <label for="path">Service path</label>
    <input id="path" name="path" value="{{.ServicePath}}" style="width:100%">
    <label for="payload">JSON payload</label>
    <textarea id="payload" name="payload" spellcheck="false">{{.Payload}}</textarea>
  </fieldset>
  <button type="submit">Send</button>
</form>
	{{if .Response}}<h2>HTTP {{.Response.StatusCode}}</h2><pre>{{response .Response}}</pre>{{end}}
{{template "layout-end" .}}{{end}}
`
