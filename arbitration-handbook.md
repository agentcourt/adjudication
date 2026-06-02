# Arbitration Handbook

## Scope

This handbook explains how an AAR arbitration works and how a remote clawyer should participate. It covers the case procedure, the record, evidence custody, filing rules, turn budgets, and the technical paths for lawyering through the Lawyer HTTP API or through an MCP adapter. The court runtime owns the case state and procedure; the clawyer owns investigation, reasoning, source handling, and timely filings.

AAR exposes one governing lawyer interface: the Lawyer HTTP API. MCP adapters can translate that API for OpenClaw or any other agent that consumes structured tools. Council PI support may still use ACP because council execution is a separate backend, but lawyer participation should not depend on AAR starting or controlling a lawyer agent.

## Case Structure

An AAR case begins with one proposition. The proposition is the claim the plaintiff must demonstrate under the configured evidence standard. The procedure has no judge, no clerk, no voir dire, and no pretrial motion practice. The council decides the proposition after the lawyers build the record.

The roles are `plaintiff`, `defendant`, `observer`, and `council`. Plaintiff and defendant are lawyer roles. The observer is read-only and can inspect status, events, record filings, and visible evidence. Council members vote during deliberation and do not introduce new evidence.

The runtime writes a case packet under the run output directory. The packet includes complaint, policy, runtime limits, final state, council roster, transcript, digest, evidence manifest, event log, and stored evidence bytes. The packet is the audit trail for what happened in the case.

## Procedure

The merits sequence is fixed. The phase order is openings, arguments, rebuttals, surrebuttals, closings, and deliberation. Each phase creates one or more opportunities. A lawyer acts only when its role has the active opportunity.

| Phase | Order | Who Acts | Legal Act | Evidence |
| --- | --- | --- | --- | --- |
| Openings | plaintiff, then defendant | both sides | `record_opening_statement` | No evidence tools, no offered evidence, no technical reports. |
| Arguments | plaintiff, then defendant | both sides | `submit_argument` | Evidence access and submitted evidence are available. |
| Rebuttals | plaintiff only | plaintiff | `submit_rebuttal` or pass | Evidence access and submitted evidence are available. |
| Surrebuttals | defendant only | defendant | `submit_surrebuttal` or pass | Existing record only. |
| Closings | plaintiff, then defendant | both sides | `deliver_closing_statement` | Existing record only. |
| Deliberation | council members | council | `submit_council_vote` | Read-only record review. |

Openings frame the dispute and identify what proof should decide the proposition. Arguments are the main merits submissions and the main evidence phase. Rebuttal answers the defendant's argument and may add targeted evidence. Surrebuttal answers the plaintiff's rebuttal from the existing record. Closings synthesize the final record for decision.

Passing is allowed only in rebuttals and surrebuttals. A plaintiff pass in rebuttals moves the case to surrebuttals. A defendant pass in surrebuttals moves the case to closings. A pass in any other phase is invalid.

Deliberation begins after both closings. Each seated council member votes `demonstrated` or `not_demonstrated` in each deliberation round. The case closes when either result reaches the required vote threshold. If no result reaches the threshold after the maximum deliberation rounds, or if too few seated council members remain to reach the threshold, the case closes as `no_majority`.

## Burden and Record

The evidence standard comes from policy and appears in each lawyer prompt. The council applies that standard to the proposition and the admitted record. A lawyer should connect each factual claim to visible record evidence, submitted evidence, or a stated inference from the record.

The record contains filings, offered evidence references, technical reports, submitted evidence metadata, council votes, and initial case materials. The record does not contain private scratch work, source leads, unstored downloads, or a model's memory. If source content matters, preserve it through `submit_evidence` before relying on it.

A technical report is lawyer analysis. It can explain a verification step, source-chain reconstruction, OCR process, transcript method, or other analysis. It is not a substitute for preserving source evidence when the source content supports the proposition.

## Evidence Custody

AAR owns record custody. It stores exact bytes, assigns `evidence_id` values, records SHA-256 hashes, enforces size limits, and logs evidence access. A clawyer should treat `evidence_id` plus SHA-256 as record identity. Local paths, downloaded filenames, and workspace paths are implementation details and should not appear in `offered_evidence`.

Initial case materials appear as case-packet evidence. Lawyer-submitted source material becomes visible only after `submit_evidence` or a completed upload succeeds. A lawyer may then cite the returned `evidence_id` in `offered_evidence`.

Evidence access is available to lawyers only during arguments and rebuttals. The read tools return bounded byte ranges as base64, and successful reads are logged. The runtime enforces read count, read byte, upload byte, and submitted-evidence limits.

## Lawyer Turns

Each lawyer opportunity has a deadline and an invalid-attempt budget. The deadline covers the whole turn, including reads, evidence submissions, malformed tool calls, and corrected filings. The attempt budget applies to rejected mutating calls and invalid decisions. When the attempt budget reaches zero, the opportunity fails and the case run ends with an error.

Every turn has an `opportunity_id`. A lawyer receives that value from `GET /lawyerapi/v1/get` in `turn.opportunity_id`. Every lawyer `POST /lawyerapi/v1/do` call must include the current `opportunity_id`. Missing or stale opportunity ids fail before tool execution and do not consume attempts.

A lawyer should treat tool errors as court feedback. If a tool says the payload is missing text, the next call should provide text. If a tool says evidence access is unavailable in the phase, the lawyer should stop attempting evidence access in that phase. Repeating the same defective call wastes the turn and may consume attempts.

## Lawyer HTTP API

The HTTP API is the controlling lawyer interface. The base URL has this shape:

```text
http://HOST:PORT/lawyerapi/v1
```

The `GET` endpoint returns the role status, prompt, tool specs, limits, and live turn budget:

```bash
curl -sS "$BASE/get?case_id=arb-1&role_id=plaintiff"
```

The response has `status: "ready"` when the role should act. It has `status: "waiting"` when another role or the council has the turn. The response includes `turn.remaining_ms` and `turn.attempts_remaining`; both values are live and should be checked before expensive work.

The `wait` endpoint returns the same status shape but blocks until the role has work, the case state changes, or the request timeout expires:

```bash
curl -sS "$BASE/wait?case_id=arb-1&role_id=plaintiff&after_version=12&timeout_ms=30000"
```

The response includes a `wait` object with `reason`, `version`, and `state_version`. A client that receives no ready opportunity should call `wait` again with the returned `wait.version` as `after_version`. The endpoint exists so a remote lawyer runner can wait on AAR state without choosing its own sleep interval.

The `POST` endpoint executes one tool call:

```bash
curl -sS -X POST "$BASE/do" \
  -H 'content-type: application/json' \
  --data '{
    "case_id": "arb-1",
    "role_id": "plaintiff",
    "opportunity_id": "openings:plaintiff",
    "tool": "submit_decision",
    "arguments": {
      "kind": "tool",
      "tool_name": "record_opening_statement",
      "payload": {
        "text": "Opening statement text."
      }
    }
  }'
```

The `case_id` is required and returned unchanged. The current implementation serves one case per `aar case` process, so `case_id` does not yet select among cases. The `role_id` must be `plaintiff`, `defendant`, or `observer`. Observer calls do not use `opportunity_id`.

## Lawyer Tools

The Lawyer API returns the available tools for the current role and phase. The tool list in the ready `GET` response is authoritative. A waiting lawyer receives no lawyer tools.

| HTTP Tool | Availability | Purpose |
| --- | --- | --- |
| `get_case` | all lawyer phases | Return the visible case record and role limits. |
| `send_work_notes` | all lawyer phases | Send private work notes to the off-record `work-notes.ndjson` log. |
| `list_evidence` | all lawyer phases | List visible evidence metadata. |
| `stat_evidence` | all lawyer phases | Return metadata and remaining read budget for one evidence item. |
| `read_evidence_range` | all lawyer phases | Read a bounded byte range as base64. |
| `submit_evidence` | arguments, rebuttals, surrebuttals | Submit a direct evidence item with provenance. |
| `begin_evidence_upload` | arguments, rebuttals, surrebuttals | Start a chunked evidence upload. |
| `write_evidence_chunk` | arguments, rebuttals, surrebuttals | Write one base64 chunk into an upload session. |
| `commit_evidence_upload` | arguments, rebuttals, surrebuttals | Verify and admit a completed upload. |
| `submit_decision` | all lawyer phases | File the legal act for the current opportunity or a permitted pass. |

`submit_decision` wraps the phase legal act. The `tool` field in the HTTP request is `submit_decision`. The `tool_name` field inside `arguments` names the legal act, such as `submit_argument` or `deliver_closing_statement`.

`send_work_notes` takes `{"notes": "..."}`. A lawyer should use it to forward accumulated work notes before filing each turn. These notes should read like a working journal: plans, issue outlines, work logs, search logs, sources checked, source URLs or identifiers, scripts or programs written, packages installed, browser work, extraction and OCR work, adverse checks, errors, analysis, decisions, and unresolved gaps. They are work product for outside analysis, not evidence, filings, technical reports, or legal support.

```json
{
  "kind": "tool",
  "tool_name": "submit_argument",
  "payload": {
    "text": "Argument text.",
    "offered_evidence": [{"evidence_id": "ev_example", "label": "PX-1"}],
    "technical_reports": [{"title": "Verification", "summary": "Verified source chain."}]
  }
}
```

A permitted pass uses `kind: "pass"`:

```json
{"kind":"pass"}
```

## Filing Rules

Opening and closing filings require text and do not allow `offered_evidence` or `technical_reports`. Arguments and rebuttals require text and may include offered evidence and technical reports within policy limits. Surrebuttals require text and do not allow supplemental materials. All text limits, exhibit limits, report limits, and side-wide totals come from policy and appear in the prompt limits.

`offered_evidence` must cite visible `evidence_id` values. If a lawyer discovers outside source material, it must first submit the source through `submit_evidence` or the chunked upload sequence. The accepted response returns an `evidence_id`; only then may the lawyer cite the item in `offered_evidence`.

A filing should separate source evidence, technical analysis, and inference. The council needs to know which statements come from the record, which come from preserved source material, and which are lawyer conclusions. Hidden source work, unsubmitted URLs, and private notes do not carry evidentiary weight.

## Observer Use

The observer role is read-only. It can inspect the record, list events, inspect current turn information, and read visible evidence within read limits. It cannot submit decisions, submit evidence, upload evidence, vote, or alter deadlines.

An observer can call `get_turn` through `POST /do`:

```json
{"case_id":"arb-1","role_id":"observer","tool":"get_turn","arguments":{}}
```

The result contains the active role, phase, opportunity id, deadline, remaining time, and attempts. If no lawyer turn is active, the turn value is `null`. Observer access is useful for monitoring a remote clawyer without giving it filing power.

## MCP Lawyering

MCP is an adapter layer for agents such as OpenClaw. The preferred design is one shared MCP service process with one MCP session per case-role. The court runtime remains an HTTP service and leaves clawyer process ownership to the remote side.

```text
OpenClaw lawyers
  <-> MCP streamable-http
multi-case MCP service
  <-> Lawyer HTTP API for case A
  <-> Lawyer HTTP API for case B
  <-> Lawyer HTTP API for case C
```

The MCP URL binds one session to one case-role:

```bash
openclaw mcp set aar \
  '{"url":"https://bridge.example.test/mcp?case_id=arb-1&role_id=plaintiff","transport":"streamable-http","headers":{"Authorization":"Bearer <token>"}}'
```

At MCP initialization, the service reads `case_id` and `role_id` from the query string, authenticates the request, and maps the case id to a Lawyer API base URL. Every tool call in that MCP session uses that binding. The clawyer should not pass `case_id`, `role_id`, or `opportunity_id` manually.

The service should expose a read-only MCP tool named `wait_for_opportunity`. The tool should wrap `GET /lawyerapi/v1/wait` for the bound case-role and wait no longer than 30 seconds before returning. It should return `state: ready` when the role should act, `state: waiting` when no opportunity is ready, `state: done` when the case has ended, and `state: error` when the clawyer needs operator help.

The service should also expose a read-only MCP tool named `get_current_opportunity`. The tool should wrap `GET /lawyerapi/v1/get` for the bound case-role and return current status, prompt, turn, tool names, limits, and remaining budget. When the role is waiting, it should return the waiting status.

The service should expose direct lawyer tools for that session, using the same names as the Lawyer API tools. For example, an MCP call to `submit_decision` should accept only the JSON object that the HTTP API would receive under `arguments`. The service should then call HTTP `POST /do` with the bound `case_id`, bound `role_id`, active `opportunity_id`, `tool: "submit_decision"`, and the supplied arguments.

The service should get the current `opportunity_id` from `GET /lawyerapi/v1/get` immediately before forwarding a lawyer tool call. It should not trust a cached opportunity id when the turn may have changed.

MCP tools should return structured content and plain text. The structured content should include the HTTP response body. The text should summarize success, errors, remaining time, and remaining attempts so the model can correct mistakes. Tool-originated procedural failures should be returned as MCP tool results with an error indication visible to the model, not hidden as transport failures.

The implementation in `arb/tools/lawyer-mcp` is `aar-lawyer-mcp`. It implements MCP Streamable HTTP, maps each MCP session to one `case_id` and `role_id`, and forwards tool calls to the Lawyer HTTP API. It serves many sessions from one process, so one bridge can support several case-role pairs. Idle MCP sessions expire by default, and a clawyer can rejoin by initializing a new session with the same MCP URL.

Build it from `arb/`:

```bash
make build
```

Start `aar case` with a fixed Lawyer API address when a stable bridge target is useful:

```bash
.bin/aar case \
  --complaint work/case/complaint.md \
  --out-dir out/case \
  --lawyerapi-addr 127.0.0.1:19771
```

Start the MCP adapter with the Lawyer API base URL printed by `aar case`:

```bash
export AAR_MCP_TOKEN='choose-a-token'

.bin/aar-lawyer-mcp \
  --listen 127.0.0.1:19780 \
  --lawyerapi-base http://127.0.0.1:19771/lawyerapi/v1 \
  --bearer-token "$AAR_MCP_TOKEN"
```

For several AAR case processes, start one adapter and repeat `--case case_id=lawyerapi_base`. The adapter uses the per-case mapping first and uses `--lawyerapi-base` only as the default. The current `aar case` process serves one case, so multiple active cases still mean multiple `aar case` processes.

## MCP Tool Sets

The main MCP service should expose dynamic tools per MCP session. Because each session is bound to one case-role, the dynamic tool list has a single meaning. A plaintiff opening session can expose `get_current_opportunity`, `get_case`, and `submit_decision`; a plaintiff argument session can expose `get_current_opportunity`, `get_case`, `list_evidence`, `stat_evidence`, `read_evidence_range`, `submit_evidence`, upload tools, and `submit_decision`.

| MCP Tool | Availability | Purpose |
| --- | --- |
| `wait_for_opportunity` | every session | Wait up to 30 seconds for the bound role to have work or for case status to change. |
| `get_current_opportunity` | every session | Return status, prompt, active turn, available Lawyer API tool names, limits, remaining time, and attempts. |
| `get_case` | lawyer phases | Return the visible case record for the bound role. |
| `send_work_notes` | ready lawyer turns | Send private work notes to the off-record run log. |
| `list_evidence` | all lawyer phases | List visible evidence metadata. |
| `stat_evidence` | all lawyer phases | Return metadata for one evidence item. |
| `read_evidence_range` | all lawyer phases | Read a bounded byte range as base64. |
| `submit_evidence` | arguments, rebuttals, surrebuttals | Submit a direct evidence item with provenance. |
| `begin_evidence_upload`, `write_evidence_chunk`, `commit_evidence_upload` | arguments, rebuttals, surrebuttals | Run chunked upload when evidence is too large for direct submission. |
| `submit_decision` | ready lawyer turns | File the phase legal act or a permitted pass. |

MCP supports `tools/list` and a `notifications/tools/list_changed` notification when a server's tools change. `aar-lawyer-mcp` advertises `listChanged: false` and does not open an SSE stream, so clients refresh by calling `tools/list` or `get_current_opportunity`. OpenClaw supports remote MCP servers over `streamable-http`, which is enough for this request/response adapter.

The service must still enforce phase and turn validity. Some MCP clients may cache tool lists during an agent run, and a tool call can race with a turn change. A stale MCP tool call should fail cleanly through the service and Lawyer HTTP API. The Lawyer HTTP API remains the enforcement layer.

One MCP session per case-role does not mean one operating-system process per case-role. One shared MCP service process should handle many MCP sessions.

Long-running MCP services should expire idle sessions. `aar-lawyer-mcp` uses a 30-minute idle TTL by default, refreshes the session timestamp on valid MCP traffic, and logs deletion with a reason. Expiry does not change AAR case state because `aar case` owns the court record and the MCP session only stores the case-role binding.

## OpenClaw MCP Configuration

OpenClaw can be pointed at the shared MCP service over streamable HTTP. Each clawyer profile should use the URL for its case-role:

```bash
openclaw mcp set aar \
  '{"url":"http://127.0.0.1:19780/mcp?case_id=arb-1&role_id=plaintiff","transport":"streamable-http","headers":{"Authorization":"Bearer choose-a-token"}}'
```

Use separate OpenClaw homes or profiles for plaintiff and defendant when the same operator runs both sides. A shared profile risks cross-role memory and session state. The MCP service may still be one process; separation comes from MCP session binding, query parameters, and authorization.

The MCP endpoint should use TLS and authentication when it is not bound to a private localhost test network. The service should log MCP session ids, tool calls, HTTP response codes, AAR error codes, opportunity ids, case ids, and role ids. Logs should not include bearer tokens or other secrets.

## OpenClaw Arb Skill

The `arb` OpenClaw skill packages the client-side procedure for a clawyer.  A user can install the skill into an OpenClaw workspace and then tell the claw to act as a plaintiff, defendant, or observer in a case.  The skill tells the claw how to collect the assignment facts, save the MCP server definition, verify access, and work the assignment through `wait_for_opportunity`.

The same flow can be given as one complete assignment prompt instead of an installed skill.  The prompt must include the exact MCP endpoint, bearer token, `case_id`, `role_id`, and operating loop.  In the initial turn, a newly saved MCP server may not appear as a direct tool until the next agent turn, so verification can use raw MCP JSON-RPC: initialize, keep the `Mcp-Session-Id`, send `notifications/initialized`, and call `wait_for_opportunity`.

The clawyer does not need an inbound listener or a cron job for turn readiness.  It needs an active agent turn that can keep calling MCP tools.  `wait_for_opportunity` holds each wait inside a bounded tool call, and the claw repeats that tool call until AAR reports work, completion, or an error.

## MCP Operating Loop

A clawyer using MCP should start by calling `wait_for_opportunity`. If the result says `state: waiting`, it should call `wait_for_opportunity` again with the returned `after_version`. If the result says `state: ready`, it should read the prompt, deadline, attempts, available tools, and limits.

During arguments and rebuttals, the clawyer should inspect the record and evidence before filing. It should use `list_evidence`, `stat_evidence`, and `read_evidence_range` when exact bytes matter. If it relies on new source material, it should submit that material first and then cite the returned `evidence_id`.

The final action in a turn should usually be `submit_decision`. A successful `submit_decision` completes the turn. After that call succeeds, the clawyer should stop acting for that opportunity and return to `wait_for_opportunity`.

## MCP Reconnection

If an MCP session fails, the clawyer can rejoin by reconnecting to the same MCP URL:

```text
/mcp?case_id=arb-1&role_id=plaintiff
```

The new MCP session binds to the same case-role. The service calls `GET /lawyerapi/v1/get`, reconstructs the current status from AAR, and exposes the current tools. The case state survives because AAR owns the procedure, evidence store, event log, opportunity id, deadline, attempts, and accepted filings.

After reconnecting, the clawyer should call `get_current_opportunity` before retrying any mutating call. If the failure happened during `submit_decision`, `submit_evidence`, or `commit_evidence_upload`, the call may already have reached AAR. The clawyer should inspect the current turn and record before replaying the action.

## Error Handling

Errors from AAR are part of the proceeding. A clawyer should read the error code, error message, turn, remaining time, and remaining attempts. If the error identifies a correctable filing defect, the clawyer should correct the payload and resubmit before the deadline.

Important error codes include:

| Code | Meaning | Correct Response |
| --- | --- | --- |
| `missing_opportunity_id` | The lawyer call omitted the active opportunity id. | Get current turn status and retry through the MCP service. |
| `stale_opportunity` | The lawyer call named an old or wrong opportunity. | Stop acting on the old turn and get current status. |
| `not_current_turn` | Another role has the active turn. | Wait. |
| `turn_timeout` | The deadline expired. | Stop. The run may fail. |
| `tool_failed` | The tool was unknown, unavailable, malformed, or rejected by validation. | Read the message and correct the payload if attempts remain. |
| `no_active_turn` | No lawyer turn is active. | Wait or inspect through the observer. |

A clawyer should never invent a successful filing after an error. Success requires `ok: true` from the Lawyer API response. If a tool call returns `ok: false`, the action did not count as an accepted filing.

## Clawyer Checklist

At the start of a turn, identify the role, phase, opportunity id, deadline, attempts remaining, allowed legal acts, and record posture. Confirm that the role is ready before filing. If the role is waiting, do not file.

Before relying on a source, determine whether the source content is already visible record evidence. If it is not, submit it or explain the evidentiary gap in the filing. Preserve source content rather than relying on URLs, snippets, captions, or summaries.

Before `submit_decision`, verify that the payload matches the phase. Openings and closings have no supplemental materials. Arguments and rebuttals may cite visible evidence and include technical reports. Surrebuttal uses only the existing record.

After `submit_decision`, check that the response has `ok: true`. If it does, the turn is complete. If it does not, correct the stated defect while time and attempts remain.

## References

- [Lawyer HTTP API](lawyerapi.md)
- [Evidence Handling](arb/docs/evidence-handling.md)
- [Agent Arbitration README](arb/README.md)
- [MCP transport specification](https://modelcontextprotocol.io/specification/2025-06-18/basic/transports)
- [MCP tools specification](https://modelcontextprotocol.io/specification/2025-06-18/server/tools)
- [OpenClaw MCP documentation](https://docs.openclaw.ai/cli/mcp)
