# ADC Update Plan

## Purpose

ADC should use the same operating model now used by `arb/` and `arbd/`: live external agents communicate through a simple HTTP API, MCP adapts that API for agents that prefer MCP, and a clerk service can create, list, kill, and inspect cases.  ADC is harder because its live proceeding is driven by Lean opportunities.  The existing ADC runner must remain the case owner because it already owns Lean state, case-file visibility, evidence imports, juror membership, deadlines, invalid-attempt limits, and final result generation.

The update should not refactor shared code across `arb/`, `arbd/`, and `adc/`.  Copying the working structure is acceptable.  The ADC implementation must delete the ACP and xproxy role paths rather than preserve compatibility.

## Agreed Decisions

Judge and clerk remain internal direct-model roles for the first version.  Plaintiff, defendant, and jurors move to HTTP/MCP.  The complaint-to-scenario setup stage remains an internal direct-model stage, because it creates the case before any live lawyer or juror opportunity exists.

Each juror gets a Pi agent when that juror first appears.  The agent persists for that juror across questionnaire, voir dire, and deliberation opportunities.  Juror model configuration comes from the JSONL request-spec pool format used by `arb/` and `arbd/`, including provider, quantization, request parameters, and persona.

## Core Runner Changes

The existing Lean opportunity loop remains the central execution path.  When Lean returns a deterministic opportunity, the runner applies it as it does now.  When Lean returns an internal role opportunity, the runner uses the existing direct model path.  When Lean returns an external role opportunity, the runner publishes the opportunity through the role API and waits for a submitted decision or reported failure.

The role API should expose the same live opportunity data the existing prompt path already constructs: role view, phase, kind, objective, actor message, allowed legal tools, legal tool schemas, constraints, remaining time, and attempts left.  The API must include `case_id`, `role_id`, `principal_id` when needed, and `opportunity_id`.  For plaintiff and defendant, `principal_id` is empty.  For jurors, `principal_id` is the Lean juror id.

External legal actions should use a stable `submit_decision` envelope.  Lean already expects that decision form through `ApplyDecision`.  The MCP tool set can stay stable while the current opportunity describes which legal tools are allowed.

## HTTP Role API

The role API should provide:

| Method | Path | Purpose |
| --- | --- | --- |
| `GET` | `/health` | Report that the case API is listening. |
| `GET` or `POST` | `/roleapi/v1/status` | Report case status, current phase, current opportunity if any, and final result if available. |
| `GET` or `POST` | `/roleapi/v1/wait_for_opportunity` | Wait up to a caller-supplied limit, capped at 30 seconds, for an opportunity for the requested role and principal. |
| `GET` or `POST` | `/roleapi/v1/get` | Return the current prompt, role view, tools, limits, and opportunity metadata without long waiting. |
| `POST` | `/roleapi/v1/do` | Execute a stable tool call such as `case_status`, file reads, `send_work_notes`, or `submit_decision`. |
| `POST` | `/roleapi/v1/fail` | Report that an external agent failed. |
| `GET` or `POST` | `/roleapi/v1/result` | Return final result if the case has ended, or pending status if it has not. |

Remote agents cannot depend on mounted case files.  The API should expose file listing, text reads, and byte/base64 reads for non-text files that the role can see.  `import_case_file` should continue to accept uploaded base64 content.

Work notes should be stored outside the case record, for analysis of agent behavior.  The `send_work_notes` tool should accept a string payload of any size reasonable for the HTTP server and write it to a work-notes log with case id, role id, principal id, opportunity id, and timestamp.

## MCP Adapter

ADC should get its own MCP adapter copied from the `arb/`/`arbd/` shape.  It should be thin: each MCP session binds to `/mcp?case_id=...&role_id=...&principal_id=...`, forwards calls to the HTTP role API, and does not encode ADC procedure.

The MCP tool set should be stable for a session.  `wait_for_opportunity` returns within 30 seconds.  If it returns a waiting response, the agent calls it again.  The opportunity response tells the agent what legal tools are allowed at that point and how to submit the decision through `submit_decision`.

## Local Run

ADC needs a local run command that starts the case API, starts MCP, starts OpenClaw lawyers, and starts Pi jurors as juror principals appear.  `adc run` fills that role.  It accepts either a complaint, which it first turns into a generated scenario through the internal setup stage, or an existing scenario JSON.  OpenClaw auth should match `arb/` and `arbd/`: `auth.json` by default, with `OPENAI_API_KEY` as an explicit alternative.

The run should start OpenClaw plaintiff and defendant agents unless the caller selects a manual lawyer mode.  Manual mode writes an instruction file that tells a remote OpenClaw how to join the MCP session for a specific case and role.

Pi juror agents should start on first appearance.  Each Pi agent receives instructions that identify the MCP endpoint, case id, `role_id=juror`, and that juror’s `principal_id`.  If a juror agent exits or reports failure, the case owner applies the existing juror-failure path.

The old direct scenario command is `adc scenario`.  It runs a scenario without starting OpenClaw or Pi agents.

## Clerk Service

The ADC clerk service should copy the simple model from `arb/` and `arbd/`: create a case, list cases, kill a case, inspect status, and proxy role API requests to the active case.  A create request can specify either a complaint path for `adc case`-style setup or an existing scenario path for `adc scenario`-style execution.  The clerk record should live in the run output directory.

## Deletions

ADC should remove ACP and xproxy from the active code path.  The `acp`, `acp-role`, and `xproxy` subcommands should go.  Runtime flags such as `--all-through-xproxy`, `--acp-role`, `--acp-command`, and `--acp-endpoint` should go.  Old xproxy juror-persona CSV handling should be replaced by the JSONL request-spec pool used by `arb/` and `arbd/`.

## Test Plan

First test the runner and HTTP API directly with curl-style requests against a small scenario.  Verify that the case API reports an opportunity, accepts `send_work_notes`, allows file reads through the role view, accepts `submit_decision`, and advances Lean state.  Verify that lawyer failure marks the case failed and that juror failure follows the existing juror timeout/dismissal path.

Next test the MCP adapter against the same case API.  Verify that a plaintiff session and a juror session can wait for opportunities, read case files, submit work notes, and submit decisions.

Then test a full local run with OpenClaw plaintiff and defendant agents using `auth.json`, and Pi jurors using a JSONL request-spec pool.  Review evidence access, evidence imports, arguments, juror behavior, and final results.

Finally test the clerk API by creating a case from a complaint, listing it, inspecting status, and killing a live case.
