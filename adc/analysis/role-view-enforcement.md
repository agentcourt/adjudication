# Role-view enforcement

Mermaid source: [`role-view-enforcement.mmd`](role-view-enforcement.mmd).

This diagram describes the enforcement loop that keeps a local or external role inside its permitted view of the case.  The runner first asks Lean for `role_view(state, role)`.  Lean returns the visible case view for that role.  The runner then assembles the prompt from the role preamble, the role view, the tool cards, and the current opportunity.  External roles receive the same role-visible information through the Role API, usually through the MCP adapter.

Inside the loop, the agent may call support tools before it submits a legal act.  The current support set includes `get_case`, `get_juror_context`, `list_case_files`, `read_case_text_file`, `request_case_file`, `read_case_file_bytes`, and `explain_decisions`.  Those calls do not bypass the role view.  They go back through Lean-backed visibility checks, and Lean returns only what that role may see.

When the agent submits a pass or a legal tool call through `submit_decision`, the runner asks Lean to apply the decision to the current opportunity.  If Lean rejects the decision, it returns a `StepErr` with an actor-facing correction message.  The runner returns that correction to the same agent, and the same opportunity remains open.  If Lean accepts the decision, the runner applies the accepted action with `step` and receives the updated state.

This diagram is important because it shows the boundary between prompt construction and formal enforcement.  The role view is not advisory.  It constrains both prompt assembly and helper-tool access, and Lean validates the resulting decision before the state can change.
