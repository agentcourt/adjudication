# Agents

ADC external agents act through a case-owned HTTP Role API.  The case process owns Lean state, role-visible case views, current opportunities, deadlines, invalid-attempt counts, case-file access, and final results.  An agent receives the current opportunity, inspects the visible record through approved tools, and submits one proposed legal act or reports failure.

MCP is an adapter over the Role API.  OpenClaw lawyers and Pi jurors use MCP because their native tool systems can call MCP tools directly.  The MCP adapter does not contain court logic; it forwards role-bound tool calls to `/roleapi/v1` and returns the case process response.

## Runtime Roles

| Role | Current full-run placement |
| --- | --- |
| Plaintiff lawyer | External OpenClaw process through MCP. |
| Defendant lawyer | External OpenClaw process through MCP. |
| Juror | External Pi process through MCP when that juror first receives an opportunity. |
| Judge | Internal direct-model role in the case process. |
| Clerk | Internal direct-model role in the case process. |
| Observer | Read-only Role API and MCP role for status and final results. |

`adc run` starts the case process, starts MCP, starts OpenClaw lawyers according to `--auto-lawyers`, and starts a Pi juror process when the juror first appears.  It does not restart a lawyer or juror after process failure.  Lawyer failure fails the case, while juror failure follows the case process's juror dismissal or replacement rule.

## Role API

The Role API lives under `/roleapi/v1` on the case API listener.  Every request includes `case_id`.  Lawyer requests use `role_id=plaintiff` or `role_id=defendant`; juror requests use `role_id=juror` and `principal_id` for the juror; observer requests use `role_id=observer`.

| Endpoint | Purpose |
| --- | --- |
| `GET /health` | Reports that the case API is listening. |
| `GET` or `POST /roleapi/v1/status` | Returns case status and current turn information. |
| `GET` or `POST /roleapi/v1/get` | Returns the current opportunity without long waiting. |
| `GET` or `POST /roleapi/v1/wait_for_opportunity` | Waits up to 30 seconds for a role opportunity or terminal status. |
| `GET` or `POST /roleapi/v1/result` | Returns final result, failed status, or pending status. |
| `POST /roleapi/v1/do` | Executes support tools, `send_work_notes`, or `submit_decision`. |
| `POST /roleapi/v1/fail` | Reports external-agent failure for the active opportunity. |

The active opportunity response names the legal tools currently allowed by Lean.  It also includes the current prompt, role-visible case view, legal tool schemas, support tool schemas, remaining turn time, and remaining attempts.  The role should use the returned `opportunity_id` when it sends work notes or submits a decision.

## MCP Adapter

`adc mcp` runs a Streamable HTTP MCP server.  A session binds to one `case_id`, one `role_id`, and, for jurors, one `principal_id`.  A client connects to `/mcp` with those query parameters, completes MCP initialization, and then includes the returned `Mcp-Session-Id` header on later requests.

The MCP tool list remains stable during the session.  Each opportunity response tells the agent which legal tools are allowed for that turn.  A lawyer or juror should call `wait_for_opportunity`, inspect the returned opportunity when it is ready, use support tools as needed, send work notes, submit one decision, and repeat until the case reports `done` or `failed`.

Standard MCP tools include `wait_for_opportunity`, `get_current_opportunity`, `case_status`, `get_case`, `explain_decisions`, `list_case_files`, `read_case_text_file`, `request_case_file`, `read_case_file_bytes`, `get_juror_context`, `send_work_notes`, `submit_decision`, and `report_failure`.  Observer sessions also get `get_case_result`.  Support tools enforce the same role visibility rules as the prompt.

## Legal Acts

Legal acts go through `submit_decision`.  A legal-tool decision uses `kind=tool`, `tool_name`, and `payload`.  A pass decision uses `kind=pass` and `reason`, but only when the active opportunity says passing is allowed.

Lean validates every submitted decision against the current state version, opportunity id, role, allowed tool set, and payload.  If Lean rejects the decision, the case process returns the rejection reason and keeps the same opportunity open while the attempt budget remains.  If attempts run out, the case process applies the configured failure rule for that role.

## Work Notes

`send_work_notes` records private work notes outside the case record.  Lawyers should use it for plans, work logs, evidence-search notes, analysis, and turn summaries before submitting a legal decision.  These notes help evaluate agent behavior without adding the notes to the court record.

Work notes are written to `work-notes.ndjson` in the output directory.  Each note records the case id, run id, role id, optional principal id, opportunity id, and note text.  The note text may contain the accumulated notes for the current turn.

## Local And Remote Lawyers

Local lawyers are OpenClaw containers started by `adc run`.  They receive an MCP server configuration, a role assignment, and the current instruction template.  They do not need access to the ADC output directory because case files and legal actions go through MCP.

Remote lawyers use the same MCP path.  Start `adc run` with `--auto-lawyers defendant` when the plaintiff is remote, or `--auto-lawyers plaintiff` when the defendant is remote.  The run writes an `openclaw-ROLE-lawyer-skill.md` file in the output directory for the missing role, and that file contains the MCP URL, token, case id, role id, and operating loop.

## Pi Jurors

Pi jurors come from a JSONL request-spec pool.  The runner binds the selected persona text to the juror opportunity prompt, and `adc run` writes each selected juror's Pi configuration from the request spec.  The Pi configuration includes the OpenRouter endpoint, model, provider data, quantization data, output-token setting, and MCP server configuration.  The process starts only when that juror first receives an opportunity.

Each Pi process has an output byte cap.  If the process exceeds the cap or exits before completing the active opportunity, `adc run` reports the failure to the case process.  The case process then decides whether the juror can be replaced or dismissed under the current state.

## Clerk Service

`adc service` creates child case processes and proxies Role API calls by `case_id`.  A create request with omitted `mode` or `mode: "run"` starts `adc run`, which gives the service the full local-agent path.  A create request with `mode: "direct"` starts `adc case` or `adc scenario` without starting local OpenClaw or Pi processes.

Each child case writes one `service-case.json` in its output directory.  The service uses that file to recover known cases on restart.  If the service cannot persist the record for a running case, it marks the case failed and stops the child process rather than letting memory and disk disagree.
