# Event Emission

ADC does not have a single global event bus.  It has three distinct output streams: persisted structured run events, runner and ACP status lines on `stderr`, and `xproxy` request and response logs on `stderr`.

The distinction matters.  An external process can already follow the structured run events with little effort, but it cannot observe every runtime message through one hook.

## Current streams

| Stream | Where it goes | Shape | Notes |
|---|---|---|---|
| Structured run events | `events.ndjson`, SQLite `events` table | JSON objects | Best source for durable machine consumption during a run |
| ACP assistant text and runner status | `stderr` | plain text | Includes `agent call`, `agent correction`, and live assistant text chunks |
| `xproxy` transport logs | `stderr` | plain text | Includes request, response, stream, and upstream error lines |

## Structured run events

Structured run events are emitted through two wrapper functions in the runner.

Action events go through `persistActionEvent` in `runtime/runner/io.go`.  These are the legal and support actions that the runner executes: `file_answer`, `offer_case_file_as_exhibit`, `submit_technical_report`, `read_case_text_file`, and similar actions.

ACP agent-tool events go through `persistAgentEvent` in `runtime/runner/io.go`.  These are notifications such as `agent_tool_call` and `agent_tool_update` that arrive from the ACP session while an external attorney agent is running.

Those two wrappers share the same lower-level sinks:

| Sink | Role |
|---|---|
| `appendEventLine` in `runtime/runner/io.go` | Appends one JSON line to `events.ndjson` |
| `Store.AppendEvent` in `runtime/store/store.go` | Inserts one row into the SQLite `events` table |

So there is no single wrapper for persisted events, but there is one NDJSON writer and one SQLite inserter.

`events.ndjson` is reset at run start.  Each appended line contains either:

- `action`: for a normal action event
- `agent_event`: for an ACP tool event

The event order in the file is the append order seen by the runner.  `appendEventLine` opens, writes, and closes the file for each event, so an external process can follow the file with ordinary line-oriented tooling.

The SQLite `events` table contains the same event families, with `action_type` holding either the action name or the ACP event type.

## Where structured events originate

For ordinary actions, `executeAction` in `runtime/runner/runner.go` is the main path.  It executes the action, updates state, and then calls `persistActionEvent`.

For ACP attorney turns, the path is split:

1. ACP support methods such as `get_case`, `list_case_files`, and `read_case_text_file` call `executeAction`, which then emits ordinary structured action events.
2. The ACP `session/update` notification handler in `runtime/runner/acp_role.go` converts `tool_call` and `tool_call_update` notifications into transcript entries and then calls `persistAgentEvent`.

That means an external listener following `events.ndjson` sees both:

- the role-visible support actions invoked through `_adc/*`
- the internal tool activity of the ACP attorney agent, including `bash`

This is now enough to reconstruct important attorney-side behavior during a run.

## What is not in `events.ndjson`

Several useful runtime messages are not persisted as structured events.

ACP assistant text chunks are appended only to the in-memory transcript during the run.  They are also printed to `stderr`.  They do not go to `events.ndjson` as they happen.

Runner status lines are also `stderr` only.  These include messages such as:

- `agent call`
- `agent correction`
- `autopilot stop`

`xproxy` logs are separate again.  `xproxy` uses its own `logf` helper in `runtime/xproxy/util.go`, which writes directly to `stderr`.  Those request and response lines do not pass through the runner persistence layer.

The final `run.json`, `digest.md`, and `transcript.md` are artifacts written after the run, not streamed event feeds.

## What an external listener can do now

Today there are two practical options.

First: follow `events.ndjson`.  This is the cleanest structured stream.  It provides durable, incremental JSON events for both ordinary actions and ACP tool activity.

Second: capture `stderr` from the `adc case` process.  This is necessary if the listener also needs:

- `xproxy` request and response lines
- live assistant text chunks
- runner status lines and correction messages

If the listener watches only `events.ndjson`, it will miss those `stderr`-only messages.

The SQLite `events` table is a reasonable secondary source, but it is less convenient for a push-style listener.  For a live external process, the NDJSON file is the simpler current surface.

## Direction for a unified listener

If ADC later moves toward a single event stream, the clean path is to introduce one internal event sink abstraction and route every stream through it.

That sink would receive a normalized event object with fields such as:

| Field | Meaning |
|---|---|
| `source` | runner, acp, xproxy, juror, or system |
| `kind` | action, agent_tool_call, agent_tool_update, assistant_text, status, transport |
| `turn` | turn index when available |
| `step` | step index when available |
| `role` | actor role when available |
| `payload` | structured data |
| `text` | plain-text fallback when no structured payload exists |
| `created_at` | event timestamp |

Under that design, the current persistence functions would become one sink implementation.  A second implementation could publish to a Unix socket, pipe, or local TCP endpoint for an external listener.

The important point is scope.  If the goal is only to let another process follow legal and ACP tool events, `events.ndjson` already provides a good base.  If the goal is to follow everything the operator sees during a run, the current code does not have one choke point.  That requires a small architectural consolidation, not a one-line hook.
