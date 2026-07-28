# Open Items

Work identified and not done, with the reason it was left.  Items are grouped by
where the decision sits.  Where a rationale was given, it is recorded; where an
item was deferred without one, that is stated too.  Findings that were
investigated and turned out not to be defects are in the last section, so they do
not get raised again.

Opened 2026-07-28 from a repository review and two live ARB runs on branch
`tidy`.  Supporting detail is in [the review](review.md) and
[the test notes](testnotes.md).

## Deferred by decision

### Refresh the council pool

`common/data/personas/pool.jsonl` was generated 2026-07-09.  Nineteen days later
one of its endpoints had been withdrawn upstream and three of its 25 seats do not
advertise tool use at all, so they cannot complete a council opportunity.  In two
live runs, seven of twelve seated members failed.

Deferred at your instruction, 2026-07-28: "nothing now".  Refreshing means a live
run of the model-pool pipeline against OpenRouter and OpenAI, which costs real
API spend.

### The vote threshold when council members fail

ARB fixes `required_votes_for_decision` at case start and never adjusts it.  When
the seated count drops below the threshold, `engine/Main.lean:610` closes the case
`no_majority` regardless of what the surviving members concluded.  In the Clerk
test case both survivors voted `demonstrated` and the result was still
`no_majority`.

ADC does the opposite.  `deriveVerdictFromJurorVotes?` caps the requirement at the
surviving sworn count through `effectiveMinimumConcurring`, so five agreeing
jurors return a verdict after one of six fails, and only an empty panel hangs.

Both rules are defensible.  ARB's reads as a quorum requirement: the parties
agreed to a five-member body and a three-vote threshold, and agent failure does
not lower the bar.  ADC's reads as proceeding with the competent panel that
remains.  No document states that the divergence is deliberate;
`arb/docs/case-failures.md` describes ARB's mechanics without contrasting them
with ADC's.

Deferred at your instruction, 2026-07-28: "let's not address that jury stuff
now".  Changing ARB's rule would also invalidate the current decision-rule proofs,
which would need reworking.

### Preflight does not test tool use

`checkCouncilSeatAvailable` sends a 16-token "reply ready" completion, and
`createCouncilAvailabilityResponse` passes `nil` for the `tools` argument at
`council_preflight.go:163`.  The provider block sets `require_parameters: true`
with `allow_fallbacks: false`, so OpenRouter routes only to providers supporting
every parameter in the request.  With no `tools` in the preflight request the
constraint is trivially satisfied and a text-only endpoint seats cleanly, then
fails at the first `submit_council_vote` with `404 No endpoints found that support
tool use` — after the entire merits phase has run.

The pool record already carries the answer.  `variant.supported_parameters`
records whether an endpoint advertises `tools`, and across both test runs that
field predicted every tool-use 404 with no false positives:

| Run | Seat | Endpoint | `tools` | Outcome |
|---|---|---|---|---|
| ex03 | C1 | `xai` | no | 404 |
| ex03 | C3 | `nebius/fp4` | no | 404 |
| ex03 | C5 | `digitalocean` | yes | voted |
| clerk | C1 | `digitalocean` | yes | voted |
| clerk | C2 | `xai` | no | 404 |
| clerk | C5 | `siliconflow/fp8` | no | 404 |

The two `nemotron-3-super-120b-a12b` rows are the same model on different
endpoints, one failing and one voting, which is the endpoint-as-unit thesis
confirmed by a live failure.  Three of 25 pool seats are affected.

A local filter on `'tools' in variant.supported_parameters` would cost no API
calls and move the failure from deliberation to seat selection, where a
replacement is free.  It would not catch endpoints whose capability changed since
inventory, which is what a live probe with a tool stub would add at the cost of
one request per candidate seat.

Deprioritized 2026-07-28 on your rationale that council failures are normal and
should be managed rather than heavily prevented, "within reason".  Recorded
because the filter is close to free; the live probe is the part that is arguably
beyond "within reason".

### Council failure modes that capability screening would not catch

Three of the seven observed failures were not endpoint capability problems.  The
endpoints advertised `tools` and the models still could not complete the work.

| Model | Failure |
|---|---|
| `qwen/qwen3-30b-a3b-instruct-2507` | Called tools but could not satisfy the Pi `mcp` wrapper schema, which requires `args` as a JSON string.  The adapter returned `args: must be string` 99 times before the agent gave up. |
| `google/gemma-3-12b-it` | Returned empty content with no tool call. |
| `qwen/qwen3-30b-a3b-instruct-2507` | Timed out after 15m0s rather than erroring. |

The `args: must be string` case is the most actionable: it is an adapter
ergonomics problem, not a model capability limit, and a weaker model loops on it
until the opportunity is exhausted.  Whether the Pi `mcp` wrapper should accept an
object as well as a string is a question for the adapter rather than this
repository.

No decision taken.  These sit behind the pool refresh, since a better pool would
reduce how often they are reached.

## Design questions with no decision yet

### `model-pool/config/`

Two files, `general_purpose_test_models.json` and `model_pool_context200k.json`,
are saved run records — a five-model test set and a recorded row selection with an
excluded-failure list.  They live in a directory the README documents as committed
input, and no tool reads either one.

Kept by your decision, 2026-07-28.  The README and manual now describe them as
retained reference sets rather than configuration, so the tree no longer
contradicts itself.  Deleting them remains available.

### `docs/attest-host.md` will drift

The document belongs to the `attest` repository and was copied here after the ten
links to `attest/dev-host.md` were found to point at a file that existed in
neither repository.  The copy resolves the links and covers every requirement the
adjudication documents claim it does.  Nothing keeps the two in step.

The alternative was a dead link.  Worth revisiting if `attest` gains a canonical
location for this document.

### Clerk status filter accepts any value

`runtime/service/clerk.go:173` filters the case list by exact string match with no
validation against known statuses.  `?status=bogus` returns HTTP 200 with an empty
list, which an operator cannot distinguish from "no cases in that state".

Noted but not changed: rejecting unknown values is a behavior change to an
operator API, and the current behavior is defensible as a filter that matches
nothing.

### `adc/docs/limits.md` proposes controls that do not exist

The document opens by saying it proposes court-configurable limits, and the timing
controls it describes — Rule 12 and Rule 56 opposition and reply windows, discovery
response windows, per-side extension caps — appear nowhere in the engine or
runtime.  The text previously read "Useful timing controls now include", which
stated a proposal as fact; it now reads as a proposal.

The document is accurate as a proposal.  Whether to build any of it is open.

## Recorded in-repo, restated here for one view

[The judge eval plan](evals/adc/judge/plan.md) carries its own remaining work,
each entry with the condition that unblocks it.

| Work | Condition |
|---|---|
| A `deliver_jury_instructions` eval under `rules/rule51/` | The `settle_jury_instructions` scorer holds up on harder fixtures. |
| A Rule 59 post-trial relief suite | Lean exposes a grant-or-deny Rule 59 opportunity.  The path observed during planning is deterministic denial, which gives prompt iteration nothing to move. |
| Suites for default judgment, stays and bonds, protective orders | State builders exist for the postures those decisions need. |
| Harder Rule 60 fixtures | Independent of the above; the current set does not separate production from candidate v1. |

## Untested paths

`aar case --attorney-model` was not exercised.  Both live runs used `aar run` with
OpenClaw lawyers in Docker.  The direct-model attorney path is a separate code
path, and `examples/iran-airspace-may-24-condition-open-record` documents a run
that uses it with `--file` to control initial evidence.

Attested execution was not exercised, at your instruction: the environment is not
set up for it.

ADC and AARD were not run live at all.  Only their Go tests were executed.

## Settled, so they are not re-raised

`Result.Council` repeats the council roster without member status.  Left alone
deliberately: it records the council as constituted, and constitution and outcome
are separate facts.  The outcome is in `final_state` within the same file, and as
of `49f6a3a` also in `digest.md` and the Clerk `result` route.

`/health` on `aar service` returns 404.  This is not a defect.  The manual
attributes `/health` to the Case API and the MCP server, and `runtime/service/`
registers no such route.  My expectation was wrong.

`adc/runtime/casegen/prompts/complaint_draft_system.md:8` reports as a broken
Markdown link.  It is a `[label](path)` format template inside backticks,
instructing a model on link form.  A false positive in the link checker.

`confession.sig.b64` and `samantha_public.pem` report as broken links in the two
ADC example `situation.md` files.  Both are produced by the adjacent `sign.sh` and
excluded by the example `.gitignore`.  `ex1/README.md` runs `sign.sh` first and its
inputs table now marks both as generated.  `situation.md` is fed to the complaint
drafter, so build notes do not belong in it.

`adc/docs/voir-dire-journal.md` cites `scratch/experiment1/` paths for data that
was never committed.  Lines 7 and 8 already disclose this.  The data is no longer
on disk, so it cannot be committed now.  Left as written by your decision.

`lake` is at `~/.elan/bin` and absent from the non-interactive PATH, so `make
build` in `arb/` needs it prepended.  Environment friction rather than a
repository defect.
