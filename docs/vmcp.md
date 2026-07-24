# VMCP Design

Status: design with a first implementation, 2026-07-24.  The implementation lives in [the vmcp project](../vmcp/README.md) as a standalone Lean package.

VMCP ("verified MCP") is a persistent MCP server implemented in Lean.  It sits between agents and real MCP tool servers, holds the state of a governed process, and accepts or rejects each tool call by rule.  Accepted calls forward to the real server.  Each run writes an event log whose terminal form is a replay certificate, and theorems about the Lean components state what an accepted certificate implies.  The design generalizes the ARB engine pattern: a pure verified core drives the process, untrusted code performs I/O, and every run is checkable afterward by replay.

VMCP enforces the procedure it is given.  It blocks every action outside its specification regardless of why the agent attempted it, including actions induced by prompt injection, and state-dependent argument predicates reach cases static filters cannot: an argument bounded by a remaining budget, or an irreversible call whose arguments must equal values recorded by an earlier approval action from a different actor.  Behavior that satisfies the full specification passes; enforcing unspecified intent is outside any such tool's function.  VMCP does not judge output quality, and it cannot observe what an upstream server does: responses enter the record as observations, as hashes and sizes, with content bytes stored outside the state.

## Architecture

One process contains four components.  Three are pure transducers: each maps a state and an input event to a new state and a list of commands.  The fourth is the I/O shell, which executes commands and injects external events, and it is the only unproven code.

| Component | Pure | Holds | Proves |
| --- | --- | --- | --- |
| Engine | yes | Process state: phase, filings, budgets, votes, policy. | ARB-style step and outcome theorems, certificate soundness. |
| Protocol layer | yes | Session bindings, pending requests, advertised tool lists. | Request and reply discipline, role-stamp integrity, advertisement correctness, no-bypass. |
| State manager | yes | Log record codec, fold, snapshot check, recovery function. | Codec round trip, recovery correctness under the crash model, chain acceptance. |
| Shell | no | Transport, files, child processes, clock, token checks. | Nothing; kept small enough to audit by hand. |

The pure layers never perform effects.  A protocol step returns commands such as reply to this session, advertise this tool list, forward this call upstream, or append this record, and the shell executes them.  External facts return as observation events: upstream responses, timeout expiries, and operator interventions all enter as recorded inputs, so the clock and the network never enter the pure layers.  ARB already treats agent text this way, so certificates stay exact.

The theorem that justifies the design is no-bypass: a forward command appears in a protocol step's output only when the engine accepted the corresponding action.  It ties the layers together and holds for the compiled artifact rather than for a code review.

## Roles and Sessions

Each participant is a separate MCP connection.  At session initialization the shell authenticates the connection, and the protocol layer binds it to a principal: a role, and a member id where identity within a role matters, so a session is juror C3 rather than a juror.  The binding is protocol state and therefore in proof scope.

Every action passed to the engine is stamped from the session's binding.  Client-supplied identity fields are ignored, so one juror's session cannot vote as another member.  Tool visibility derives from engine state: the engine emits the next obligation with its allowed tools, and the protocol layer advertises each tool only on sessions whose binding matches.  When the engine unseats a member, the next advertisement computation removes that session's tools with no separate cleanup.  The intended theorems: no action reaches the engine with an identity other than its session's binding, and no session is advertised a tool outside its binding's current allowance.

## State

The append-only event log on disk is the authoritative state.  Memory holds the current engine and protocol states as a cache of the log's fold.  The shell appends a record before executing that step's commands, so at a crash an event is either in the log and reproducible by replay, or it never took effect.  The current implementation flushes the log handle without an fsync, which protects against process crashes; power-failure durability needs a sync and is an open item.  Restart folds the log through the pure step functions and recovers the exact engine state; a `state.json` snapshot after each step is an optimization, and determinism makes any snapshot checkable against the fold.  Protocol state is not recovered: connections die with the process, sessions reconnect and re-authenticate, and their tool lists are recomputed from the recovered engine state.

Records carry framing with length and checksum, so torn-tail detection is a pure predicate, and a hash chain over records makes tampering detectable by a pure verifier.  Recovery correctness is conditional on a stated crash model: single writer, append-only, corruption confined to the final record.  That model is an assumption about the shell and the filesystem.  The accurate claim is recovery verified under the stated disk model; a claim of proved crash safety would be wrong.

The certificate is the log's terminal form: initialize request, accepted actions with recorded observations, and final state.  Verification is the same fold.  The default deployment is one process per case, matching the one-run-one-directory artifact layout; a supervisor owns spawning.

## Transport

Version one uses stdio in both directions: the gate is an MCP server to agents over stdio and an MCP client to upstream servers it spawns over pipes.  Lean has no mature HTTP stack, and stdio keeps the shell tiny.  Streamable HTTP transport is deferred.  The pure-transducer shape serializes all sessions into one total event order, which is what replay verification wants, and the cost is acceptable at tool-call rates.

## Trusted Base

The unproven remainder: the shell and its append-before-effect discipline, token checking, the Lean compiler and toolchain, the crash model, hash collision resistance, and JSON round-trip fidelity until the round-trip lemmas exist.  The round-trip requirement appears in three places, in the engine boundary, the protocol messages, and the log records, so the codec and lemma machinery is shared foundation work rather than a per-component afterthought.

## Development Plan

The first implementation took a different order than planned: the persistent process, protocol layer, and engine were built together against a simplified arbitration, and the codec foundation was deferred.  The plan below reflects that state.  Open decisions that block nothing now: the first real workflow, remote upstream servers, and the principal authentication mechanism.

1. [x] Codec foundation for the certificate chain.  Actions, case states, and their components use hand-written codecs with round-trip lemmas (`action_roundtrip`, `caseState_roundtrip`).  Open: round-trip lemmas for the JSON-RPC protocol messages; the config file keeps derived decoding as an input rather than a claim.
2. [x] Gate engine.  A simplified arbitration: openings, deliberation with sequential votes, member failure, closure by strict majority, with outcome soundness proven for all three resolutions and the closure link (`outcomeSound` for reachable and certificate-claimed states).  Budgets beyond the statement limit and approval binding remain open.
3. [ ] State manager.  Implemented: append-before-reply log discipline, replay recovery at startup, per-step state snapshot.  Open: record framing, checksums, hash chain, and the recovery theorem under the stated crash model.
4. [x] Protocol layer.  The MCP subset initialize, tools/list, tools/call, and list-changed notifications, with inbound notifications ignored without a response, session bindings and stamping, and the `parseCall_actor`, `toolsFor_sound`, `toolsFor_complete`, `gateStep_no_bypass`, and `gateStepCall_change_logged` theorems.  Open: request and reply discipline, and log completeness lifted from the call-handling boundary to `gateStep`.
5. [x] Shell and assembly.  Envelope stdio transport, log discipline, recovery, and a `verify` command that replays the log against the final state, exercised end to end by `drive/demo.sh` and the failure-path script `drive/paths.sh`, with a compile-time `#guard` suite in `Tests.lean`.  Upstream spawning does not exist because the demo procedure has intrinsic tools only.
6. [ ] Evaluation.  Run one real workflow through the gate, measure per-call overhead, and write findings into this document with a decision on what follows.
