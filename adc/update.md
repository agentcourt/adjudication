# ADC Update Summary

This note records the current ADC runtime direction after the ARB and ARBD service work was ported.  ADC now uses the same core shape: a case-owned HTTP Role API, an MCP adapter over that API, local OpenClaw lawyers, Pi jurors configured from JSONL request-spec pools, and a clerk service that creates and tracks child cases.  Judge and clerk roles remain internal direct-model roles in the current full-run path.

The active external-agent path is `adc run`.  It starts the case API, starts MCP, starts OpenClaw lawyers according to `--auto-lawyers`, and starts a Pi juror process when that juror first receives an opportunity.  The case process owns Lean state, current opportunity, deadlines, case-file visibility, validation, failure rules, event logging, work-note logging, and final output.

The clerk service now defaults to that full local-agent path.  `POST /clerk/v1/cases` with omitted `mode` or `mode: "run"` starts `adc run`; `mode: "direct"` starts `adc case` or `adc scenario` without local OpenClaw or Pi process startup.  The service stores one `service-case.json` in the case output directory and proxies `/roleapi/v1` calls by `case_id`.

## Current Capabilities

| Area | Current behavior |
| --- | --- |
| Complaint setup | `adc complain`, `adc case`, and complaint-based `adc run` produce a normalized one-claim case packet, private party strategy memos, and `generated-scenario.json`. |
| Direct execution | `adc case` and `adc scenario` can run without local external-agent processes, with selected roles exposed through the Role API when requested. |
| Full local execution | `adc run` starts the case API, MCP, OpenClaw lawyers, and Pi jurors. |
| Remote lawyer operation | `adc run --auto-lawyers plaintiff` or `--auto-lawyers defendant` writes role-specific OpenClaw instructions for the omitted lawyer. |
| Juror model configuration | Pi jurors use request-spec JSONL records and receive the configured endpoint, model, provider data, persona, and MCP server. |
| Clerk service | `adc service` creates, lists, kills, inspects, and proxies child cases. |
| Work notes | External roles can send private work notes through `send_work_notes`, written to `work-notes.ndjson`. |
| Failure handling | Lawyer failure fails the case; juror failure is reported to the case process for replacement or dismissal rules. |

## Documentation

The current operating reference is [`manual.md`](manual.md).  The short entry point is [`README.md`](README.md).  Agent behavior is described in [`docs/agents.md`](docs/agents.md), and event files are described in [`docs/events.md`](docs/events.md).
