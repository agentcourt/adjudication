# Role-view enforcement

Mermaid source: [`role-view-enforcement.mmd`](role-view-enforcement.mmd).

This diagram describes the enforcement loop that keeps a local or delegated role inside its permitted view of the case.  The runner first asks Lean for `role_view(state, role)`.  Lean returns the visible case view for that role.  The runner then assembles the prompt from the role preamble, the role view, the tool cards, and the current opportunity.  Delegated ACP roles get one transient home directory and no host repository, run-directory, or persistent host-state mount.

Inside the loop, the agent may call helper tools before it submits a legal act.  The diagram names the current helper set: `_adc/get_case`, `_adc/get_juror_context`, `_adc/list_case_files`, `_adc/read_case_text_file`, and `_adc/request_case_file`.  Those helper calls do not bypass the role view.  They go back through Lean-backed visibility checks, and Lean returns only what that role may see.

When the agent submits a pass or a legal tool call through `_adc/submit_decision`, the runner asks Lean to apply the decision to the current opportunity.  If Lean rejects the decision, it returns a `StepErr` with an actor-facing correction message.  The runner returns that correction to the same agent, and the same opportunity remains open.  If Lean accepts the decision, the runner applies the accepted action with `step` and receives the updated state.

This diagram is important because it shows the boundary between prompt construction and formal enforcement.  The role view is not advisory.  It constrains both prompt assembly and helper-tool access, and Lean validates the resulting decision before the state can change.
