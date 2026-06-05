# Event Emission

ADC writes several event and log files during a run.  The case process writes structured case events to `events.ndjson` and the SQLite `events` table.  External roles can also send private work notes through the Role API, and `adc run` records local OpenClaw, Pi, and MCP process output under `logs/`.

The streams have different purposes.  `events.ndjson` and `run.db` describe case actions and selected agent events.  `work-notes.ndjson` preserves private planning notes sent by an external role.  Process logs explain how local OpenClaw, Pi, and MCP processes behaved outside the formal case record.

## Current Streams

| Stream | Location | Shape | Purpose |
| --- | --- | --- | --- |
| Structured case events | `events.ndjson` | JSON lines | Durable action and agent-event stream. |
| SQLite events | `run.db` table `events` | Database rows | Queryable event record for reports and PACER-style views. |
| Work notes | `work-notes.ndjson` | JSON lines | Private external-role notes outside the case record. |
| Final result | `run.json` | JSON | Authoritative final machine-readable result. |
| Transcript and digest | `transcript.md`, `digest.md` | Markdown | Human-readable summaries written after the run. |
| Local-agent logs | `logs/` | Plain text and JSON lines | MCP, OpenClaw, and Pi process output from `adc run`. |
| Service child logs | `service-logs/` | Plain text | Child stdout and stderr captured by `adc service`. |

## Structured Events

Action events go through `persistActionEvent` in `runtime/runner/io.go`.  These events include legal acts and support actions, such as `file_answer`, `offer_case_file_as_exhibit`, `submit_technical_report`, `read_case_text_file`, and similar actions.  Each event records the run id, turn index, step index, actor role, action type, payload, response, and timestamp.

Agent events go through `persistAgentEvent` in the same file.  The current Role API path uses agent events for case API errors and model-completion result records.  These events use a negative step index based on their per-turn sequence so they remain distinct from legal action steps.

Both event families write to the same sinks.  `appendEventLine` appends one JSON object to `events.ndjson`.  `Store.AppendEvent` inserts one row into the SQLite `events` table.

## Work Notes

`send_work_notes` writes private notes to `work-notes.ndjson`.  The note contains the case id, run id, role id, optional juror principal id, opportunity id, and full note text.  These notes are not legal acts and do not become part of the court record.

A lawyer should use work notes for plans, evidence-search logs, analysis, dead ends, and turn summaries.  A juror can use the same mechanism when the instructions ask for work notes.  The notes give operators a way to review agent work that would otherwise remain inside a remote or containerized process.

## Local-Agent Logs

`adc run` writes local process logs under `logs/`.  The MCP adapter writes `mcp.stderr`.  Each OpenClaw lawyer and Pi juror gets stdout and stderr logs named for the role or juror.  Pi logs pass through a conservative repeated-content filter that can drop repeated accumulated message-update prefixes while preserving tail content.

These logs are outside the formal case record.  They are useful for diagnosing agent startup, MCP configuration, model failures, excessive output, and cases where a local agent exits before completing an opportunity.  If a local juror exceeds its configured output byte cap, `adc run` reports that failure to the case process through the Role API.

## Service Logs

`adc service` captures each child process's stdout and stderr under `service-logs/` in that case output directory.  It also writes `service-case.json`, which records the case id, run id, mode, child PID while active, status, output directory, private case API base URL, log paths, final summary, and error message when one exists.

The service polls the child case API `/health` endpoint during startup.  If the case API does not become healthy before the configured timeout, the service marks the case failed.  If the service cannot persist a status update to `service-case.json`, it records a failure in memory and stops the child process.

## Reading A Run

For case behavior, start with `run.json`, `digest.md`, and `transcript.md`.  For exact action sequence, read `events.ndjson` or query the SQLite `events` table.  For external-agent planning, read `work-notes.ndjson`; for process failures, read the relevant file under `logs/` or `service-logs/`.
