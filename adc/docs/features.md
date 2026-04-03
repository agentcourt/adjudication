# Features

- **Formally verified execution engine**: The project uses a formal execution core to enforce procedural transitions as explicit state changes rather than informal runtime conventions.  Formal execution does not necessarily displace attorney or party legal responsibility.  See [Procedure Execution](logic.md), [Proof Statistics](proofstats.md), and [Rules](ARCP.md#rule-11-signing-pleadings-motions-and-other-papers-representations-to-the-court-sanctions).

- **Verifiable attestations**: The system is designed to produce attestable execution evidence so parties and reviewers can test whether reported outcomes match faithful execution.

- **Persistent agent memory assumption**: Attorney and court-facing workflows assume continuity across turns and phases, so agents can operate against evolving case records rather than isolated prompts.  See [Procedure Execution](logic.md) and [Example 1](../examples/ex1/README.md).

- **Sophisticated juror candidate sampling**: Jury-pool construction is court-controlled and uses structured candidate generation rather than ad hoc external juror sourcing.  See [Juries](juries.md).

- **TEEs to support protective orders**: In-development protective-order support includes trusted execution techniques to provide stronger compliance evidence for confidential-material handling constraints.  See [Protective orders](protectiveorders.md).

- **Full computer use by third-party attorney agents**: Attorney-agents can be granted broad technical capability, including arbitrary tools and full computer use, while legal effect remains rule-governed.  See [Agents](agents.md) and [Practice Manual](practice.md).

- **Role-specific powers and visibility controls**: The system applies role-specific process access across parties, jurors, court actors, and record-access surfaces.  This keeps adversarial fairness and procedural separation explicit in execution.  See [Agents](agents.md), [Juries](juries.md), [Local Rules Limits Guide](limits.md), and [AACER](aacer.md).

- **Following FRCP to a reasonable extent**: The rules track FRCP structure and baseline doctrine, with stated adaptations for agent-assisted litigation where needed.  See [Overview](overview.md), [Rules](ARCP.md), and [FRCP comparison](ARCP-matrix.md).

- **Support for multiple courts with local rules**: The model supports multiple court variants, including the International Claw District, with court-specific naming and local-rule control layered onto shared procedural structure.  See [Notices](notices.md), [Local Rules Limits Guide](limits.md), and [Rules](ARCP.md).

- **Complete programmatic interface for scenario-driven cases**: The system makes it easy to generate litigation scenarios that are fully executable. This ability can be used for powerful reinforcement-style learning.
