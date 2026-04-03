# Agents

This page describes the current external-agent path.  The immediate use is attorney agents.  The same boundary could support other delegated roles later.

The boundary is strict.  [Lean](logic.md) remains the rule-authoritative procedure engine.  The Go runtime stages files, assembles prompts, persists state, exposes helper methods, and calls the formal engine.  An external agent does not mutate case state directly.  It receives the current opportunity, inspects the visible record, and proposes one act.  `adc` validates that act through Lean before any state change occurs.

When a role is delegated through ACP, that role's live opportunity turns go through ACP rather than the local model client.  Complaint planning remains a separate path.

## Architecture

The current path uses the [Agent Client Protocol](https://agentclientprotocol.com/).  `adc` acts as the ACP client.  The delegated role agent runs behind `pi-acp`, and `pi-acp` starts `pi`.  In the checked-in demo path, both `pi-acp` and `pi` run inside the Podman image.

One ACP server configuration may serve more than one delegated role.  `adc` accepts repeated delegated-role selections while keeping one shared ACP command, argument list, environment, and timeout.  The role and opportunity still come from Lean, not from ACP-side configuration.

| Layer | Role |
|---|---|
| Lean | determines whether an act is procedurally valid and what state transition follows |
| Go runtime | stages files, manages prompts, persists state, and calls Lean |
| ACP client in `adc` | talks to an external role agent |
| ACP server (`pi-acp`) | translates ACP requests into `pi` RPC calls |
| External role agent (`pi`) | reads, reasons, uses tools, and proposes one legal act |

This separation matters.  Court procedure does not move into the delegated agent.  The rule-authoritative boundary remains inside `adc`.

## ACP methods

The ACP layer uses custom `_adc/*` methods for case-scoped operations.  These methods are not legal acts in themselves.  They give the delegated role access to the visible record and to the one path that can propose a legal act.

The current method surface includes:

| Method | Purpose |
|---|---|
| `_adc/get_case` | return the current role-visible case view |
| `_adc/get_juror_context` | return one juror candidate's questionnaire and oral voir dire record |
| `_adc/list_case_files` | return visible case files and metadata |
| `_adc/read_case_text_file` | return the full text of a visible `.txt` or `.md` file |
| `_adc/request_case_file` | attach a visible image file to the next model turn |
| `_adc/submit_decision` | submit one proposed legal act or a pass |

`_adc/submit_decision` is the only state-changing path.  It sends a proposed act back into `adc`, which validates it against the current Lean opportunity.  If the act is malformed, untimely, or disallowed, Lean rejects it and the state remains unchanged.

If an agent does the wrong thing, the system responds with a clear explanation and another chance to act within the same turn.  If the agent keeps failing, the turn fails openly.  The runtime does not silently reinterpret the agent's intent.

## `pi-acp` changes

Upstream `pi-acp` did not expose host-defined ACP custom methods to `pi`.  That was the gap this system needed to close.  The delegated lawyer had to be able to call `_adc/get_case`, `_adc/list_case_files`, and `_adc/submit_decision`.  ACP transport alone was not enough.

The local `pi-acp` branch adds one generic feature: it turns configured ACP custom methods into ordinary `pi` tools.  The bridge reads a method list from `PI_ACP_CLIENT_TOOLS`, generates a temporary `pi` extension, and registers one tool per ACP custom method.  When `pi` calls one of those tools, `pi-acp` forwards the call back to the ACP client through `conn.extMethod(...)`.

That change is generic.  It does not mention `adc` in the adapter logic.  `adc` supplies `_adc/*` methods because that is its domain.  Another host application could expose a different method family through the same bridge.

## Containerized `pi-acp` and `pi`

The checked-in ACP experiments do not run `pi` on the host.  They run both `pi-acp` and `pi` inside a Podman container.  `adc` starts that container with `pi-container/acp-podman.sh` and talks to `pi-acp` over stdio.

This layout keeps the attorney agent off the host filesystem while preserving a local ACP path.  The container gets one fresh writable home directory for the turn.  Case access goes through ACP methods, not direct host mounts.

| Component | Placement |
|---|---|
| `adc` | host |
| Lean engine | host |
| `pi-acp` | Podman container |
| `pi` | Podman container |
| ephemeral ACP home | bind-mounted into the container |

The wrapper keeps stdio intact and mounts only that ephemeral home directory.  The container does not receive the repository checkout, the run output directory, or a persistent host `~/.pi` tree.  For ACP attorney turns, case materials arrive through `_adc/*` methods and file-by-`file_id` reads, not from direct host paths.

## Host `xproxy`

Provider keys do not need to enter the container.  The current path uses host-side `xproxy` for that reason.

In this mode:

| Component | Secret material |
|---|---|
| host `xproxy` | real provider keys |
| containerized `pi` | only `PI_XPROXY_API_KEY=xproxy` |

`xproxy` presents an OpenAI-compatible endpoint on the host, translates requests where needed, and forwards them to the configured upstream provider.  The containerized agent sees only the proxy endpoint and a dummy token.  This keeps provider credentials outside the container while preserving normal model access.

## What the lawyer can do

The ACP lawyer does two kinds of work.

First, it uses case-scoped ACP methods to inspect the formal case state: the docket, visible files, juror context, and current opportunity.  Second, it uses ordinary `pi` tools only inside its ephemeral home directory.

That distinction matters.  Legal acts do not happen through the shell.  Shell work supports the lawyer's reasoning inside its own transient workspace.  The legal act still happens only through `_adc/submit_decision`, which `adc` validates through Lean.

The current file-access limits are narrower than the long-term goal:

| Path | Current support |
|---|---|
| `read_case_text_file` | visible `.txt` and `.md` files |
| `request_case_file` | visible image files |
| new party files | `import_case_file` with `original_name` and `content_base64` |
| non-image binary inspection through ACP | not yet supported |

That limit comes from current `pi` and `pi-acp` content and tool-result types, not from Lean or `adc`.

## Why this matters

This path is the current plan for third-party attorney agents.  A third-party lawyer should not need to embed court logic, and the court should not need to trust the lawyer with direct state mutation.

The ACP design gives each side what it should own.  `adc` owns procedure, state, visibility, and validation.  The outside lawyer owns strategy, drafting, technical inspection, and judgment about what to do next.  The lawyer may be local, remote, or third-party.  The court does not need to know how that lawyer was built.  It needs a defined protocol and a rule-authoritative validation path.

## Current limits

Several limits remain open.

First, non-image binary documents are not yet usable through the ACP plus `pi` path.  Text files and images work.  Other file types need either richer `pi` content types or a different transport path.

Second, this architecture still depends on careful prompt design.  The delegated lawyer now sees the exact legal-tool schema of the current opportunity because generic submission methods led to avoidable payload errors.

Third, the helper surface is still narrow.  It is enough for the current attorney workflow, but it does not yet expose every potentially useful case-derived helper.

## References

- [Agent Client Protocol](https://agentclientprotocol.com/)
- [Agent Client Protocol standard repository](https://github.com/agentclientprotocol/agent-client-protocol)
- [pi-acp](https://github.com/svkozak/pi-acp)
- [Lean](https://lean-lang.org/)
- [Podman](https://podman.io/)
