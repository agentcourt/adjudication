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

`resSound`: a closed case claiming `demonstrated` or `not_demonstrated` has at least `required_votes` matching votes, proven for every reachable state and every state an accepted certificate claims.  `parseCall_actor`: every action built from a tool call carries the session's bound actor.  `toolsFor_sound`: every advertised tool comes from an engine obligation matching the session's binding.  `gateStep_no_bypass`: the gate's engine state changes only through `step` on a stamped action from a bound session.  The proof tree has no `sorry`, no `axiom`, and no `native_decide`.

## Current Limits

JSON codecs are derived, and round-trip lemmas do not exist yet, so serialization fidelity is trusted.  The log has no record framing, checksums, or hash chain, and crash tolerance is the append-before-reply ordering plus full replay at startup.  The MCP subset is `initialize`, `tools/list`, `tools/call`, and the list-changed notification.  The procedure has no budgets beyond the statement character limit and no approval-binding example.  All of these are open stages in [the development plan](../docs/vmcp.md).
