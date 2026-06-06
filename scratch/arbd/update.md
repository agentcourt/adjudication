# ARBD Update Status

The active migration plan is [`update-plan.md`](update-plan.md).  That plan tracks the move from the older adapter-driven runtime to the current API-first model.  The current implementation has `aard case`, `aard mcp`, `aard service`, and `aard run`, and the obsolete adapter runtime has been removed.

`arbd` keeps degree-arbitration semantics throughout the migration.  Complaints state a question, the policy supplies a judgment standard, lawyers argue for numeric answers or answer ranges, and council members submit one integer answer from `0` through `100`.  The final result is the answer map, not an aggregate and not a binary outcome.

Future update notes should start from the current API model.  The supported lawyer path is the Lawyer API, with MCP as an adapter for OpenClaw.  The supported council-agent path is the Council API, with MCP as the Pi-facing adapter used by `aard run`.
