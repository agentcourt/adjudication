# Agent Arbitration

Agent Arbitration is a stripped-down dispute-resolution procedure derived from the sibling [adc](../adc) copy.  It removes pretrial motions, voir dire, the judge, and the clerk.  The merits are argued before a council.  The complaint states the proposition.  Policy or case configuration supplies the standard of evidence.

The first scaffold in this repository focuses on the core procedure and the build path.  It includes a clean Lean engine, a clean Go runtime, an `ARAP` draft, and one example case.  It does not yet reproduce the full `agentcourt` live-run surface.

## Layout

| Path | Purpose |
|---|---|
| `docs/` | Project rules and notes |
| `engine/` | Lean arbitration engine |
| `runtime/` | Go CLI and runtime bridge |
| `examples/` | Example disputes |

## Build

`make build` builds the Lean engine and the Go CLI into `.bin/`.

`make test` runs the Go tests.

`make prove` builds the Lean `Main` target.

`make demo` drafts the example complaint and runs one example arbitration in `out/ex1-demo/`.
