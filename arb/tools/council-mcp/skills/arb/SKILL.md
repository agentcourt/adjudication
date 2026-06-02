---
name: arb-council
description: Work an AAR arbitration case as one council member through the AAR Council MCP adapter.
---

# AAR Council

Use this skill when the user asks this OpenClaw agent to act as a council member in an AAR arbitration case.  The AAR court runs elsewhere.  This agent joins it through an MCP server that exposes the court's Council HTTP API.

The assignment requires a case id, a council member id, an MCP URL, and any bearer token required by that MCP server.  If the user gives a full MCP URL that already contains `case_id` and `member_id`, use those values unless they conflict with the stated assignment.  If a required value is missing, ask for it before changing configuration.

## Setup

Configure the MCP server with the URL from the assignment.  The URL has this shape:

```text
http://HOST:PORT/mcp?case_id=CASE_ID&member_id=MEMBER_ID
```

Use `transport: streamable-http`.  If the assignment includes a bearer token, send it as an `Authorization: Bearer ...` header.  Name the MCP server `aar-CASE_ID-MEMBER_ID` so later turns can identify it.

## Work Loop

Call `wait_for_council_opportunity` first.  If it returns `state: waiting`, call `wait_for_council_opportunity` again with the returned `after_version`.  If it returns `state: ready`, read the returned prompt, turn, limits, and tools before taking any action.  Complete exactly that opportunity.

When ready to decide, call `submit_council_vote` once with `vote` set to `demonstrated` or `not_demonstrated`, and include a concise rationale.  After a successful vote, return to `wait_for_council_opportunity`.  Stop when `wait_for_council_opportunity` returns `state: done`.

Use `get_current_council_opportunity` to inspect the current state without waiting.  Use `get_case`, `list_evidence`, `stat_evidence`, and `read_evidence_range` only through the MCP tools supplied by AAR.  Do not search the web, introduce new facts, create new evidence, or upload evidence.

## Evidence

The court record recognizes evidence by `evidence_id` and hash.  Use `list_evidence` to see visible evidence and `stat_evidence` before reading file bytes.  Use `read_evidence_range` only for the parts needed to decide the proposition, and stay within the returned read limits.

## Errors

Treat a malformed payload response as procedural feedback while attempts remain.  Correct the payload and call the tool again for the same opportunity.  If AAR reports a stale opportunity, stop the current action and call `get_current_council_opportunity` or `wait_for_council_opportunity` again.
