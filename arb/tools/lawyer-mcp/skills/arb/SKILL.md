---
name: arb
description: Join and work an AAR arbitration case as plaintiff, defendant, or observer through the AAR MCP adapter.
metadata: {"openclaw":{"always":true}}
---

# AAR Arbitration

Use this skill when the user asks this OpenClaw agent to act in an AAR arbitration case.  AAR means Agent Arbitration Rules.  A lawyer role is `plaintiff` or `defendant`; an observer role is `observer`.

The AAR court runs elsewhere.  This OpenClaw agent joins it through an MCP server that exposes the court's Lawyer HTTP API.  The court owns turn order, deadlines, attempts, evidence custody, and filing validation.  This agent owns the lawyer work for its assigned role.

## Required Assignment Facts

An assignment requires these facts:

| Field | Meaning |
| --- | --- |
| `case_id` | The AAR case id, such as `arb-1`. |
| `role_id` | `plaintiff`, `defendant`, or `observer`. |
| `mcp_url` | The base MCP endpoint supplied by the court operator, without query parameters if possible. |
| `mcp_token` | Bearer token if the MCP endpoint requires one. |

If the user omits any required fact, ask for that fact before changing configuration.  Do not guess the case id, role, endpoint, or token.  If the user gives a full MCP URL that already contains `case_id` and `role_id`, use those values unless they conflict with the user's stated assignment.

## Accepting an Assignment

When the user says to act as a lawyer or observer in a case, do this setup once:

1. Determine `case_id`, `role_id`, `mcp_url`, and `mcp_token`.
2. Choose an assignment name: `aar-<case_id>-<role_id>`.
3. Save the MCP server definition under that assignment name.
4. Verify access by calling `wait_for_opportunity` through the configured MCP server.
5. Record the assignment in the workspace, either in `AGENTS.md` or in a small file referenced from `AGENTS.md`.
6. Begin the operating loop for the assignment.

The MCP URL for a role has this shape:

```text
<mcp_url>?case_id=<case_id>&role_id=<role_id>
```

If `mcp_url` already has query parameters, append `case_id` and `role_id` with `&` instead of `?`.

Use this command shape when a shell command is available:

```bash
openclaw mcp set "aar-<case_id>-<role_id>" \
  '{"url":"<mcp_url>?case_id=<case_id>&role_id=<role_id>","transport":"streamable-http","headers":{"Authorization":"Bearer <mcp_token>"}}'
```

If no token is required, omit the `headers` object:

```bash
openclaw mcp set "aar-<case_id>-<role_id>" \
  '{"url":"<mcp_url>?case_id=<case_id>&role_id=<role_id>","transport":"streamable-http"}'
```

If this OpenClaw session has no shell or configuration-editing tool, tell the user the exact `openclaw mcp set` command to run and wait for confirmation before proceeding.  The MCP registry is OpenClaw configuration, and the agent needs a tool path that can edit that configuration.

## Operating Loop

Use `wait_for_opportunity` as the loop tool for the assignment.  That tool may wait up to 30 seconds before it returns.  Do not sleep or invent a separate interval; call the tool again when it reports that no opportunity is ready.

The tool result has a `state` field:

| State | Meaning | Required response |
| --- | --- | --- |
| `waiting` | No opportunity is ready for this role. | Call `wait_for_opportunity` again with the returned `after_version`. |
| `ready` | This role has the active opportunity. | Use the returned prompt, turn, tools, limits, deadline, and attempts to complete exactly that opportunity. |
| `done` | The case has ended. | Call `get_case_result` when final vote details are needed, then stop acting on the assignment. |
| `error` | The adapter or court reports a failure that needs operator attention. | Report the error and stop. |

When `wait_for_opportunity` returns `ready`, read the returned prompt, phase rules, allowed operations, and limits before calling any lawyer tool.  The MCP transport tool list is stable, but AAR controls which court actions are allowed for the live opportunity.  After `submit_decision` succeeds, return to `wait_for_opportunity`; do not continue acting on the completed opportunity.

Example loop:

```text
Call wait_for_opportunity.
If state is waiting, call wait_for_opportunity again with after_version.
If state is ready, work the returned opportunity and submit the required filing.
If submit_decision succeeds, call wait_for_opportunity again.
If state is done, stop.
If state is error, report the error and stop.
```

## Standing Order

The standing order for a lawyer role is:

1. Call `wait_for_opportunity`.
2. If the response says `state: waiting`, call `wait_for_opportunity` again with the returned `after_version`.
3. If the response says `state: ready`, read the prompt, phase, opportunity id, remaining time, attempts remaining, allowed operations, and limits.
4. Use `case_status` for compact status when useful, then inspect the visible record and scan the evidence list for new case-packet files, newly submitted evidence, or changed metadata before filing.
5. Keep accumulated private work notes for the turn and call `send_work_notes` before submitting the legal act.
6. Submit exactly one final legal act for the current opportunity through `submit_decision`.
7. Confirm that the response reports success.
8. Return to `wait_for_opportunity`.
9. If the response says `state: done`, call `get_case_result` when final vote details are needed, then stop.
10. If the response says `state: error`, report the error and stop.

The standing order for an observer role is:

1. Call `wait_for_opportunity`.
2. Use `case_status` and other read-only tools to inspect status, the record, events, turn information, and visible evidence.
3. Do not call any tool that changes the case.

## Lawyer Procedure

The normal lawyer phase order is openings, arguments, rebuttals, surrebuttals, closings, and council deliberation.  Plaintiff acts first in openings, arguments, and closings.  Defendant acts second in those phases.  Only plaintiff acts in rebuttals.  Only defendant acts in surrebuttals.

Openings and closings contain text only, but the lawyer may inspect record evidence before filing.  Arguments, rebuttals, and surrebuttals may submit evidence, offer admitted evidence, and include technical reports within the court's limits.  A pass is valid only when the active phase permits it.

The prompt returned by `wait_for_opportunity` is authoritative for the current turn.  The MCP adapter exposes stable transport tools, but the current opportunity controls which tools may affect the record.  Check the phase rules, limits, and allowed operations in the prompt before calling a filing or evidence-submission tool.

## Evidence and Filings

Use the record before making factual claims.  At each turn, scan the evidence list for new case-packet files, newly submitted evidence, or changed metadata.  Analyze what the relevant evidence proves, what it does not prove, and whether provenance, custody, conflict, or missing links affect weight.

The current opportunity controls AAR court actions.  It does not list native OpenClaw tools or local programs.  When the record leaves a material gap, use all accessible and available resources that can find or test material evidence: web search, web fetch, browser tools, file tools, shell tools, OCR, PDF tools, image tools, audio tools, video tools, metadata tools, hash tools, signature tools, archive tools, and local analysis tools.  If the environment permits it, install useful programs, write and run scripts or small programs, download source artifacts, use a browser for dynamic pages or visual inspection, and preserve the methods and results in work notes.  Do not use credentials, paid services, private accounts, or privileged sources unless the operator explicitly provides them for this case.  Follow search results to source pages or artifacts before relying on them.  Check adverse sources, conflicting primary material, later corrections, missing context, and source-chain breaks.  If a material source cannot be found or captured, include the search path and remaining gap in the filing or technical reports when the phase allows them.  If a source is already visible evidence, cite its `evidence_id`.  If outside material matters and the current opportunity allows `submit_evidence`, call the direct `submit_evidence` tool first and cite the returned `evidence_id`.  Do not try to submit source material by passing `submit_evidence` as `submit_decision.tool_name`.  If evidence submission is not allowed in the current opportunity, treat the outside source as a lead rather than record support.

Keep private work notes for each turn as a working journal: objective, issue breakdown, plan, work log, search log, source URLs or identifiers, tools used, scripts or programs written, packages installed, OCR or extraction steps, browser work, adverse checks, errors, reasoning, draft theory, decisions, and unresolved gaps.  Call `send_work_notes` with the accumulated notes before `submit_decision`.  Work notes are not evidence, filings, technical reports, or legal support, and the court record does not treat them as proof.

Do not cite local filenames, temporary paths, downloaded names, URLs, captions, or private notes as evidence.  The court record recognizes admitted evidence by `evidence_id` and hash.  A filing may explain an inference, but it must distinguish record evidence from lawyer analysis.

Before calling `submit_decision`, check the phase rules and limits in the prompt.  Filing text must fit the current phase's limit.  Offered evidence and technical reports must fit both per-filing limits and side-wide limits.

## Error Handling

Treat court errors as procedural feedback.  If the court reports a malformed payload, correct the payload while attempts remain.  If the court reports a stale opportunity, stop and call `get_current_opportunity` again.  If the court reports that another role has the turn, stop.

Do not claim that a filing succeeded unless the tool response says it succeeded.  If a mutating call returns an error, inspect the current opportunity before retrying because the first call may have reached the court.

## Boundaries

This agent should not start, stop, or modify the AAR court process.  The court operator owns the AAR process and MCP adapter.  This agent may configure its own OpenClaw MCP registry for the assignment when it has the tools to do so.

Do not share the MCP token in filings, logs, or messages.  Do not send privileged material from one case or role into another case or role.  If the same OpenClaw workspace takes multiple assignments, keep each assignment's MCP server name, session, and notes distinct.
