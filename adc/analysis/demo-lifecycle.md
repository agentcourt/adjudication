# Demo lifecycle

Mermaid source: [`demo-lifecycle.mmd`](demo-lifecycle.mmd).

This diagram shows the end-to-end path for the standard example run.  The run starts from `situation.md` and its linked files, regenerates the key material and signature with `sign.sh`, drafts `complaint.md` with `adc complain`, and then runs `adc case`.

The middle of the diagram shows the runtime path inside `adc case`.  The runner stages the complaint attachments, generates the plaintiff and defense strategies, builds a scenario with `case_init` and no case-specific turns, and asks Lean to initialize the case state.  From there the run enters the `next_opportunity` loop.

The loop shows the decision path for each opportunity.  The runner logs the role, phase, reason, and allowed tools.  If the role is local, the runner makes a direct model call.  If the role is delegated, the runner starts an ACP session with `pi-acp` and `pi` in Podman, using one transient home directory and no host repository mount.  Either path yields a decision or a pass.  Lean validates that decision against the current opportunity.  If Lean rejects it, the runner returns a plain-language correction and the same opportunity stays open.  If Lean accepts it, the runner executes the accepted act, updates the SQLite run database and the event stream, and either loops to the next opportunity or writes the final artifacts.

This diagram is operational.  It does not try to describe every prompt field, tool call, or formal action family.  It shows the control path of a live demo run.
