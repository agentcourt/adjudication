# ADC Porting Inventory

This inventory records the `arb` changes that should inform `adc` work after the recent arbitration updates.  The main conclusion is that the ACP execution improvements should move first, because they strengthen external-agent operation without changing adjudication semantics.  Evidence provenance and proof architecture require design work before implementation, because `adc` already has different case-file, exhibit, report, and opportunity models.

## Inventory Basis

The review compared recent `arb` changes with the overlapping `adc` runtime, engine, proof, example, and documentation paths.  The most relevant `arb` additions were ACP endpoint handling, PI wrapper staging, attorney instructions, OpenClaw attorney support, submitted evidence, work-product preservation, invalid-attempt feedback, merits graphs, expanded examples, and a larger Lean proof program.  The comparison focused on whether each change carries over as infrastructure, requires an `adc`-specific design, or belongs only to arbitration.

`adc` already has substantial adjacent machinery.  It has ACP role execution, PI wrapper staging, direct model flags for non-ACP roles, case-file import, file requests, visible case files, exhibits, technical reports, support-tool budgets, local-rule limits, and prompt guidance around available methods.  It also has a mature domain proof set, though those proofs concentrate on local procedure and role validity rather than global run progress.

## Immediate Ports

| Item | Source in `arb` | Current `adc` position | Recommendation |
| --- | --- | --- | --- |
| Remote ACP endpoints | `arb/runtime/runner/acp.go` and `common/acp` endpoint support | `adc/runtime/runner/acp_role.go` accepts a command, args, env, and timeout, but no endpoint. | Add endpoint support to the ACP role configuration and CLI.  The interface should fit all `adc` roles rather than copying attorney-specific flags. |
| ACP model configuration | `arb/runtime/runner/pi_container_home.go` stages PI settings and model catalog entries from typed model configuration. | `adc/runtime/runner/pi_container_home.go` relies on `ADC_FLASH_XPROXY_MODEL` for wrapper runs. | Replace or extend the environment-only model path with explicit role configuration.  Preserve a compatibility path if existing scripts depend on the environment variable. |
| ACP standing instructions | `arb` stages attorney instructions into the PI home for external agents. | `adc` has role prompts and scenario data, but no staged PI instruction file for delegated ACP roles. | Add role instruction staging for plaintiff, defendant, judge, clerk, and juror agents.  The instructions should be role-neutral infrastructure, with role-specific content supplied by configuration or repository files. |
| ACP work-product directory | `arb` creates `/home/user/work-product` and preserves agent outputs. | `adc` does not give ACP roles a stable work-product directory. | Create the directory during ACP session setup, describe it in the role prompt, and copy it into the run output.  This improves run inspection and external-agent debugging without changing the engine. |
| Invalid-submission history | `arb/runtime/runner/invalid_attempts.go` records ordered rejection reasons and returns clearer terminal feedback. | `adc` direct turns report rejection issues, and ACP tool calls return errors, but the run does not preserve a useful sequence of failed submissions. | Add ordered rejection history for direct and ACP roles.  For ACP, decide whether the counter lives inside `_adc/submit_decision` handling or in the host turn loop, because the ACP agent controls the retry loop. |
| ACP capability and limit text | `arb` prompts show local search capability, remote endpoint ownership, and remaining filing limits. | `adc` role prompts expose allowed methods, but do not consistently show remaining case-file, exhibit, report, and opportunity limits. | Add a compact capability and limit section to ACP prompts.  The text should derive from current state and policy rather than restating static documentation. |
| Generated signing artifacts | `arb/examples/ex1/.gitignore` ignores generated signing outputs, and `arb` removed tracked derived files. | `adc/examples/ex1` and `adc/examples/ex2` still track `confession.sig.b64`. | Ignore generated `.sig.b64` files and remove tracked generated signing outputs after confirming the examples regenerate them. |

These ports should be implemented before larger semantic changes.  They share existing infrastructure, impose limited proof burden, and make external-agent behavior easier to diagnose.  They also create the runtime foundation needed by any later OpenClaw-style adapter.

## Design Candidates

### Evidence Provenance

`arb` added a first-class `submit_evidence` path with source URL, source description, retrieval timestamp, MIME type, relevance, SHA-256, stored bytes, and visible-file conversion.  `adc` already has `import_case_file`, `request_case_file`, visible case files, exhibits, and technical reports.  A direct `submit_evidence` copy would duplicate existing `adc` mechanisms and create unclear procedural boundaries.

The better `adc` design is to extend imported case-file metadata with provenance fields.  Candidate fields include source URL, source description, retrieval timestamp, MIME type, relevance statement, SHA-256, byte count, original name, and storage path.  The design question is whether `adc` needs a distinct procedural category for party-submitted source evidence, or whether provenance belongs to every imported case file.

That decision affects the Lean state, JSON schemas, CLI/runtime validators, prompts, docs, and proof obligations.  If provenance applies to all imported case files, the port should update `import_case_file` and its host implementation.  If source evidence needs separate procedural treatment, then `adc` needs a new action with explicit rules for visibility, exhibit conversion, and admissibility.

### OpenClaw Adapter

`arb` added an OpenClaw attorney adapter and a TCP bridge that expose an external attorney as an ACP endpoint.  The adapter is arbitration-specific: it uses `_aar/*` methods, attorney filing schemas, and AAR evidence behavior.  `adc` needs `_adc/*` methods, opportunity decisions, case-file imports, technical reports, exhibits, and multiple court roles.

The OpenClaw adapter should wait until remote endpoint support and provenance semantics exist in `adc`.  After that, the adapter can translate an external agent response into `ApplyDecision` input and optional file/report imports.  It should use the common ACP endpoint path rather than a separate transport.

### Proof Architecture

`arb` expanded its proof program around reachable phase shape, fixed-frame progress, viable outcomes, bounded termination, append-only/provenance properties, no-stuck live states, and outcome soundness.  `adc` has many domain proofs, including opportunity closure, orchestration, local rules, jury handling, Rule 56, default judgment, post-judgment motions, and role guards.  It does not yet have the same global run-shape proof layer.

The useful port is architectural.  Define an `adc` reachable-run predicate, prove frame stability for case identity, court, policy, status, phase, and ordering, prove append-only properties for docket events, case files, exhibits, and technical reports, and prove live autopilot states have an available next action until a terminal condition applies.  Add bounded termination results over `max_turns`, opportunity budgets, phase budgets, and judgment/post-judgment closure rules.

These proofs should be staged.  Start with reusable frame lemmas and append-only lemmas, then define reachability, then prove progress and termination.  Copying `arb` Lean files would obscure the differences between arbitration rounds and civil adjudication opportunities.

### Merits or Litigation Graphs

`arb` added a merits graph workflow for arbitration theories, contentions, and support.  `adc` could benefit from a litigation graph, but its nodes and edges should follow civil procedure: pleadings, claims, defenses, motions, orders, exhibits, technical reports, jury findings, verdicts, and judgments.  A direct port of the arbitration graph would give `adc` the wrong vocabulary and weak semantics.

Treat the `arb` graph as a schema pattern only.  If `adc` adds a graph, define it from adjudication events and proof-relevant state transitions.  The graph should answer concrete questions about the case record, not duplicate narrative summaries.

### Convenience Scripts and Documentation

`arb/arbitrate.sh` wraps common arbitration runs and xproxy setup.  `adc` already has Makefile targets for demos and xproxy runs.  A small `adjudicate.sh` may be useful after the ACP endpoint and model work, but the arbitration script should not be copied.

Documentation should follow implementation.  Once endpoint support exists, update `adc/docs/agents.md` with remote ACP endpoint configuration, PI wrapper model selection, work-product behavior, and role instruction staging.  If provenance changes are implemented, update the case-file and exhibit documentation at the same time.

## Keep in `arb`

Council constitution material, ARAP policy text, arbitration examples, arbitration-specific merits graphs, and attorney filing thresholds should remain in `arb`.  `adc` has court, jury, motion, exhibit, report, and judgment procedures with different state transitions.  Porting those files would create misleading parallels.

The council strict-majority work also does not transfer directly.  `adc` already has jury and bench-verdict proof obligations, including vote-threshold reasoning.  Any further voting work should start from the `adc` jury model, not from arbitration council proofs.

Plain complaint text support in `arb` should not move automatically.  `adc` complaint drafting has structured civil-procedure requirements, including parties, claims, requested relief, and court context.  Accepting arbitrary complaint text as a single claim would weaken that model unless a separate intake design preserves the required structure.

## Proposed Order

1. Add ACP endpoint, model, instruction, and work-product support.
2. Add invalid-submission history and better ACP/direct rejection feedback.
3. Update ACP prompts with dynamic capability and limit text.
4. Clean generated signing artifacts in `adc/examples`.
5. Decide the evidence provenance model.
6. Implement provenance changes in the engine, runtime, schemas, prompts, docs, and tests.
7. Add an OpenClaw-style ADC adapter after endpoint and provenance behavior settle.
8. Build the global proof layer in stages: frame lemmas, append-only lemmas, reachability, progress, and bounded termination.
