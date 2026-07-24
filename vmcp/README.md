# vmcp

A persistent MCP server implemented in Lean.  It holds the state of a simplified arbitration, accepts or rejects each tool call by rule, advertises to each connection only the tools its role currently holds, and writes an append-only log whose replay is the certificate of the run.  [The VMCP design](../docs/vmcp.md) records the architecture and development plan.  The project is standalone: it builds with `lake build` under Lean `v4.27.0` and imports nothing from the rest of the repository.

## Layout

| Path | Content |
| --- | --- |
| `Vmcp/Engine.lean` | Pure process core: case state, actions, `step`, obligations, replay, certificate check. |
| `Vmcp/Gate.lean` | Pure protocol layer: session bindings, JSON-RPC handling, tool advertisement, commands. |
| `Main.lean` | The I/O shell: envelope transport, log discipline, recovery, `serve` and `verify`. |
| `Proofs/Engine.lean` | Decision soundness (`resSound`), preservation, reachability, certificate soundness. |
| `Proofs/Gate.lean` | Stamping, advertisement soundness, and the no-bypass theorems. |
| `drive/demo.sh` | Drives one full case through the server and verifies the log. |

## Use

```sh
lake build
drive/demo.sh
```

The server reads envelope lines from stdin: `{"session": "s1", "payload": <JSON-RPC message>}`.  The envelope multiplexes several participants over one stdio pipe for ad hoc driving; it is the test transport, and one standard MCP stdio connection per participant can replace it without touching the pure layers.  A session authenticates through `initialize` with a `token` named in the config, which binds it to a role and, for council members, a member id.  Tool calls are stamped from that binding; client-supplied identity is ignored.  `vmcp verify --config C --log L --state S` replays the log through the engine and compares the result with the state file.

## Theorems

`outcomeSound`: every resolution is backed by its ground, proven for every reachable state and every state an accepted certificate claims.  A `demonstrated` or `not_demonstrated` resolution has at least `required_votes` matching votes, a `no_majority` resolution reached neither threshold, and any settled resolution means the case is closed.  `parseCall_actor`: every action built from a tool call carries the session's bound actor.  `toolsFor_sound` and `toolsFor_complete`: the tools advertised to a session are exactly those from engine obligations matching the session's binding.  `gateStep_no_bypass`: the gate's engine state changes only through `step` on a stamped action from a bound session.  `gateStepCall_change_logged`: a call that changes the engine state emits the log record for the stamped action that produced it.  `action_roundtrip` and `caseState_roundtrip`: decoding an encoded action or case state yields the value, over hand-written codecs, so the log and state files carry exactly the values the theorems govern.  The proof tree has no `sorry`, no `axiom`, and no `native_decide`.

## Current Limits

The certificate chain (actions, states, and their components) uses hand-written codecs with round-trip lemmas; the config file still decodes through derived instances, which is acceptable because the config is an input rather than a claim.  The JSON-RPC protocol messages have no round-trip lemmas.  The log has no record framing, checksums, or hash chain, and crash tolerance is the append-before-reply ordering plus full replay at startup; the append flushes the handle without an fsync, so an operating-system crash can lose an acknowledged action even though a process crash cannot.  Two proof gaps remain at the protocol layer: request and reply discipline, and log completeness at the `gateStep` boundary rather than the call-handling boundary.  A second connection presenting an already-bound token is rejected, so one principal holds at most one live session.  The MCP subset is `initialize`, `tools/list`, `tools/call`, and the list-changed notification, with all other notifications ignored without a response.  Tests are the `#guard` suite in `Tests.lean`, which builds with the package, plus `drive/demo.sh` for the happy path and `drive/paths.sh` for the failure paths: unknown token, duplicate binding, member failure into `no_majority`, tamper detection, and restart recovery.  The procedure has no budgets beyond the statement character limit and no approval-binding example.  Open items live in [the development plan](../docs/vmcp.md).
