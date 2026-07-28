# Repository Docs

This directory contains cross-system notes spanning more than one runtime.  System-specific references stay under `adc/docs/`, `arb/docs/`, or `arbd/docs/`; eval references stay under `evals/` and pool-construction references under `model-pool/`.  The proof note records the proof and certificate status across ARB, ADC, and AARD.

## Documents

| Document | Use |
| --- | --- |
| [Proof Work Status](proof-notes.md) | Proof surface, certificate status, remaining proof direction, and proof limits. |
| [VMCP Design](vmcp.md) | Design for a verified MCP gate: architecture, roles, state, trusted base, and development plan. |
| [Dev Host Requirements](attest-host.md) | The generic `dev` host that the `attest` exec AMI launcher assumes: baseline, paths, EC2 defaults, AWS permissions, network, and verification commands.  The three per-system attested runbooks build on it. |
