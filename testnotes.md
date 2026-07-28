# Test Notes

Live testing of the ARB runtime on branch `tidy`, 2026-07-28.  Binaries rebuilt
from source before the runs: `aarengine` through `lake build`, `aar` through `go
build`.  `lake` lives at `~/.elan/bin` and is absent from the non-interactive
PATH, so `make build` needs it prepended.

## What ran

One full local case, `examples/ex03`, through `aar run` with OpenClaw lawyers in
Docker and Pi council agents in Podman.  One case created through the Clerk API
against a running `aar service`.  Cheap checks first: `aar validate` over eight
committed complaints, all of which parse.  No attested runs.

`examples/iran-airspace-may-24-condition-open-record/` has no `complaint.md`.
That is intentional and its README says so: the complaint is generated from
`situation.md`, and `market-page.txt` is passed as the only case-packet file.  It
also uses `aar case --attorney-model` rather than OpenClaw containers, which is a
second attorney path not yet exercised here.

## Council seat failures

`ex03` resolved `no_majority` after four of five council seats failed.  That
resolution is correct under the engine's rule, but the failures have three
distinct causes and only one of them is understood as intended behavior.

| Seat | Model | Failure |
| --- | --- | --- |
| C1 | `x-ai/grok-4.20-multi-agent` | `404 No endpoints found that support tool use` at the first tool call. |
| C2 | `qwen/qwen3-30b-a3b-instruct-2507` | Called tools, but could not satisfy the Pi `mcp` wrapper schema.  `args: must be string` was returned 99 times before the agent gave up. |
| C3 | `nvidia/nemotron-3-super-120b-a12b` | Same tool-use 404 as C1. |
| C4 | `google/gemma-3-12b-it` | Returned empty content with no tool call. |
| C5 | `nvidia/nemotron-3-super-120b-a12b` | Voted `demonstrated` with a record-grounded rationale. |

A sixth seat failed before the case even started.  `nvidia/nemotron-3-super-120b-a12b`
pinned to `dekallm/fp8` returned 404 during preflight, because OpenRouter now
serves that model only through deepinfra, digitalocean, and nebius.  The runtime
recorded `council_member_replaced` with the full cause and seated the next member
of the shuffled pool.  That part works: `council_preflight.go` takes the next pool
candidate rather than a hardcoded fallback, and the pinning it enforces
(`allow_fallbacks: false`, `only: [provider]`, `require_parameters: true`) is why
the dead endpoint failed loudly instead of silently rerouting to a different
endpoint than the pool selected.

### Preflight does not test the capability the run needs

`checkCouncilSeatAvailable` sends a 16-token "reply ready" completion.
`createCouncilAvailabilityResponse` passes `nil` for the `tools` argument at
`council_preflight.go:163`, so no tool definition reaches the provider and
routing is never resolved under the constraint deliberation will use.  An
endpoint that serves text but not tool calls therefore seats cleanly and fails
nineteen minutes later, after the entire merits phase, when the Pi agent first
calls `submit_council_vote`.

The function already accepts a `tools []map[string]any` parameter.  Populating it
with a minimal tool definition would move C1 and C3 into the preflight
replacement path, where a working pool member takes the seat.  The C2 and C4
failures are different in kind and would not be caught this way: C2 can call
tools but cannot produce the argument encoding the Pi wrapper demands, and C4
cannot call tools at all despite the endpoint supporting them.  Catching those at
seat time would need a probe that completes a real tool call, which is what the
deleted `model-speed.sh` did at pool-construction time by recording
`TOOLS_SUPPORTED=true` only on a successful `submit_council_vote`.

### The pool is stale

`common/data/personas/pool.jsonl` was generated 2026-07-09.  Nineteen days later
one endpoint is gone and three more seats cannot do the work.  The manual already
says current-provider claims require a refreshed inventory and fresh evals; this
run is evidence of how quickly that becomes true.

## The outcome is not explained by the artifacts that report it

`ex03` resolved `no_majority` on a single vote from a five-member council.
Neither of the two summary artifacts says why.

`run.json` lists all five council members with `member_id`, `model`,
`persona_file`, and `request_spec`, and no status field.  The failure record lives
in `state.json` as `council_members[].status` and `failure_message`, and in the
event log as `opportunity_failed` and `council_member_removed`.  The manual
presents `run.json` as the artifact for machine inspection, so a tool reading it
alone sees a full council and an unexplained `no_majority`.

`digest.md` is the concise written account.  It carries every filing, exhibit,
submitted-evidence entry, technical report, and the vote tally, and it lists all
five council members without marking any of them failed.  The strings "fail",
"removed", and "error" do not appear in it.  A reader sees "Tally: 1
demonstrated" against a five-member roster and is left to infer the rest.

## What held up

The certificate does what the documentation claims.  `aar verify-certificate`
replayed 15 accepted actions through the Lean engine and matched the claimed
final-state hash.  Both tamper directions are caught, with distinct errors:
editing `state.json` to flip `no_majority` to `demonstrated` gives "packet final
state mismatch", and dropping the last accepted action from `certificate.json`
gives "replayed final state mismatch".  Both exit 1.

The evidence store is content-addressed correctly.  All six entries hash-match and
size-match their manifest records, stored under a two-character shard of the
SHA-256.

The runtime follows the Lean phase rule exactly.  `engine/Main.lean:398-406`
requires two openings, two arguments, one rebuttal, one surrebuttal, and two
closings before deliberation; the event log shows precisely that, including the
single-filing rebuttal and surrebuttal, which is the engine's design rather than a
dropped filing.  The strict-majority policy check at `Main.lean:366-371` and the
all-seated-must-vote gate at line 605 match what `arb/docs/councils.md` describes.

## Clerk

`aar service` starts on its documented default `127.0.0.1:19770`.  Creating a case
through `POST /clerk/v1/cases` returns the record with `case_id`, `run_id`, `pid`,
`out_dir`, and both log paths, and starts a child `aar run`.

| Route | Behavior |
| --- | --- |
| `GET /clerk/v1/cases` | Lists the case. |
| `GET /clerk/v1/cases?status=running` | Filters correctly. |
| `GET /clerk/v1/cases/ID` | Returns the record. |
| `GET /clerk/v1/cases/ID/result` mid-run | 200 with "still pending or has no final result", rather than an error. |
| `GET /clerk/v1/cases/ID/artifacts` | Lists what exists so far, with sizes. |
| Unknown case id, on record, result, and artifacts | 404 with a structured `unknown_case` error. |

Two notes that are not defects in the code.  `/health` on the service returns 404;
the manual attributes `/health` to the Case API and the MCP server, and
`runtime/service/` registers no such route, so my expectation was wrong.
`?status=bogus` returns 200 with an empty list, because `clerk.go:173` does an
exact string match with no validation against known statuses, which makes an
operator typo indistinguishable from "none in that state".

### The Clerk case

The `ex01` case completed with `exit_code: 0`, `status: completed`, and resolution
`no_majority` on two `demonstrated` votes.  Its certificate verifies: 13 accepted
actions replayed to the claimed hash.  Three of five seats failed again, and one
of them is a fourth failure mode not seen in `ex03`.

| Seat | Model | Outcome |
| --- | --- | --- |
| C1 | `nvidia/nemotron-3-super-120b-a12b` | Voted `demonstrated`. |
| C2 | `x-ai/grok-4.20-multi-agent` | Tool-use 404. |
| C3 | `qwen/qwen3-30b-a3b-instruct-2507` | Timed out after 15m0s rather than erroring. |
| C4 | `qwen/qwen3.5-plus-02-15` | Voted `demonstrated`. |
| C5 | `openai/gpt-oss-120b` | Tool-use 404. |

A sixth seat was replaced at preflight for the same missing-endpoint 404 seen in
`ex03`.  Across the two runs, seven of twelve seated council members failed, and
five of those failures were the tool-use 404 that preflight cannot see.

Two seats voting `demonstrated` still yields `no_majority` because
`required_votes_for_decision` is 3.  The engine is right; the council is the
problem.  The Clerk `result` route returns `council_members: []`, so the same
reporting gap noted above applies to the API as well as to `run.json` and
`digest.md`.

### Artifact routes and path confinement

`GET /clerk/v1/cases/ID/artifacts` lists nine artifacts with sizes.  Fetching
`digest.md` through the API returns its content, and a `Range: bytes=0-40` request
on `events.ndjson` returns 206 with the partial body.  The evidence route serves
committed case-packet files by `evidence_id`.

Three path-traversal attempts were refused.  `../../../etc/passwd` and its
percent-encoded form return 404.  A leading-slash form returns 301, which is Go's
ServeMux collapsing the double slash; following the redirect returns 404 and no
file content.
