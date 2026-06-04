# AARD Service And Agent Update Plan

## Goal

Bring `arbd` up to the current `arb` operating model while preserving degree-arbitration semantics.  The result should have the same capabilities as `arb`: HTTP lawyer and council APIs, MCP, `auth.json` OpenClaw support, local OpenClaw lawyer runs, Pi council runs from full pool configs, Clerk service APIs, remote lawyer skills, evidence custody, work notes, and final-result inspection.  AARD must still decide degree questions by collecting integer council answers, not binary votes.

## Approach

The low-risk approach is copy-and-adapt rather than shared refactoring.  `arb` and `arbd` have different procedural rules, prompts, Lean state, and final-result semantics.  Keeping separate packages avoids a shared abstraction that would have to encode both systems before either one is stable.

Implementation should proceed from the case runtime outward.  MCP, Clerk, and `aard run` all depend on the case process exposing stable HTTP APIs and terminal-result behavior.  AARD should therefore first gain the `arb/runtime/proceeding` shape, adapted to degree-specific filings and council answers.  After that, `arb/runtime/mcp`, `arb/runtime/service`, and `arb/runtime/localrun` can be copied and adapted with narrower risk.

## Steps

| Step | Work | Verification |
| --- | --- | --- |
| 1 | Create `arbd/runtime/proceeding` by adapting the legacy degree behavior to the current `arb/runtime/proceeding` options and API shape. | `go test ./arbd/runtime/proceeding`; `lake build Proofs` remains green. |
| 2 | Replace the old `aard` CLI dispatcher with direct subcommands: `complain`, `validate`, `case`, `mcp`, `service`, and `run`. | `go test ./arbd/runtime/cmd/aard`; direct `aard complain` and `aard validate` tests. |
| 3 | Add private Case API, Lawyer API, Council API, health, status, wait, result, and failure behavior. | HTTP tests through `aard case` process start and direct API calls. |
| 4 | Add `arbd/runtime/mcp` by adapting `arb/runtime/mcp` to AARD lawyer and council tools. | MCP tests for lawyer, observer, council, bearer auth, session binding, tool authority, and result calls. |
| 5 | Add `arbd/runtime/service` by adapting `arb/runtime/service` and Clerk to start `aard run`. | Clerk create, list, inspect, kill, and result tests with fake and real `aard` child processes. |
| 6 | Add `arbd/runtime/localrun` by adapting `arb/runtime/localrun` for degree arbitration. | Unit tests for command construction, `auth.json`, remote lawyer skill, council config, and process failure handling. |
| 7 | Add `arbd/agent-instructions` templates for OpenClaw lawyers, remote lawyers, and Pi council members. | Template tests check degree wording, score/answer requirements, evidence guidance, and stop rules. |
| 8 | Retire obsolete runtime paths after the new surfaces pass: old command package, old adapter-only case path, and duplicate OpenClaw adapter paths that no longer serve the supported flow. | `rg` checks for obsolete command names and stale docs; build has one supported command path. |
| 9 | Run end-to-end checks in increasing cost order. | `go test ./arbd/...`, `lake build Proofs`, process/API tests, MCP tests, Clerk tests, then live `aard run`. |

## Degree-Specific Rules To Preserve

AARD complaints use `# Question`, not `# Proposition`.  Lawyer filings advocate numeric answers or narrow ranges rather than demonstrated/not-demonstrated outcomes.  Council members submit one integer answer from `0` through `100` plus a rationale.  The final result is the answer map keyed by `member_id`.  The service and MCP layers should expose those results without translating them into binary resolutions.

The party roles remain `plaintiff` and `defendant` in the runtime because the Lean engine, examples, and current prompts use those role ids.  Documentation may describe claimant and respondent, but APIs should use the same role ids that the engine expects.

Evidence should keep the current AARD `evidence_id` direction.  New source material must enter through evidence-submission tools before a lawyer cites it in `offered_evidence`.  Technical reports remain attorney analysis and should not carry source bytes.

## Acceptance Criteria

`go test ./arbd/...` passes.  `lake build Proofs` passes under `arbd/engine`.  `aard case` can run as a standalone case process with HTTP Lawyer and Council APIs.  `aard mcp` can serve lawyer, observer, and council sessions.  `aard service` exposes Clerk create, list, inspect, kill, and result APIs.  `aard run` can start OpenClaw lawyers with either `auth.json` or `OPENAI_API_KEY`, start Pi council agents from `pool.jsonl`, and write a complete run packet.  A generated remote lawyer skill lets an independently running OpenClaw join one role through MCP.
