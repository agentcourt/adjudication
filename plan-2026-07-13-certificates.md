# Certificate Plan: 2026-07-13

## Direction

A replay certificate should bind a terminal output packet to the Lean state-transition sequence that produced it.  The certificate records the initialization request, each public action accepted by the engine, the claimed final state, and the compact JSON hash of that final state.  Verification replays the certificate through the system's Lean engine and compares the replayed final state with the packet's final state file.

Services should expose certificate artifacts through the existing artifact APIs.  They should list and fetch `certificate.json` when the packet contains it, using the same artifact semantics as `run.json`, `state.json`, and logs.  Services should not run certificate verification as part of case creation, listing, result polling, or artifact reads.

ARB now supplies the reference implementation.  The runtime writes `certificate.json`, `aar verify-certificate` checks it against `state.json`, and the proof package exposes accepted terminal certificates as closed facts or failed facts.  ADC and AARD should follow that shape, with schemas and proof targets that match their different engines.

## ARB Baseline

ARB has the complete first certificate layer.  The runtime records only accepted engine actions, so rejected role attempts, stale opportunities, and invalid tool calls do not enter the certificate.  The verifier checks schema version, procedure name, required fields, claimed-state hash, packet-state hash, Lean replay acceptance, and replayed-state hash.

The proof boundary now covers both terminal outcomes.  `checkReplayCertificate_terminal_facts` returns either `ClosedCertificateFacts` or `FailedCertificateFacts` for an accepted terminal certificate.  Closed facts include replay, reachability, terminal accounting, outcome soundness, due-process filing facts, and decision-summary replay; failed facts include replay, reachability, the initialized length bound, the failed status, the recorded failed-opportunity object, and decision-summary replay.

## ADC Plan

ADC needs a separate certificate schema because its initialization request and terminal states differ from ARB.  Its initialization request includes the court state, case summary or claim packet fields, parties, filing date, jurisdiction facts, and attachment seeds.  Its accepted actions span pleadings, motions, discovery, trial, jury acts, judgment, and failure reports, so the certificate should store the same `action_type`, `actor_role`, and payload boundary used by the runtime's Lean `Step` calls.

ADC writes `state.json` and `certificate.json` beside `run.json` in terminal output directories.  `adc verify-certificate` compares the claimed final state against `state.json`, replays the recorded initialization and accepted engine transitions, and reports the transition count and final-state hash.  The artifact route lists and fetches the certificate without adding service-side verification.

The first Lean target should mirror ARB's replay foundation: exact replay, reachability, and terminal-state accounting for accepted certificates.  The next proof target should cover verdict and judgment soundness, including the configured jury threshold and the existing effective-concurrence rule after juror failure.  A failed certificate package should account for lawyer failure and juror failure separately, because those failures have different legal effects in ADC.

## AARD Plan

AARD is structurally closest to ARB, so its runtime certificate should reuse the ARB shape with AARD names and schema version.  The initialization request should contain the initial state, degree question, and council members.  Accepted actions should include merits filings, submitted evidence, council answers, council-member failure, and party failure in the same order accepted by the Lean engine.

AARD should write `certificate.json` beside `run.json` and `state.json` in terminal output directories.  The verifier should replay the certificate through `.bin/aardengine`, compare the certificate hash to `state.json`, and report the accepted action count and final-state hash.  The service artifact routes should list and fetch the certificate through the existing Clerk and direct case artifact APIs.

The first Lean target should be exact replay with reachability and terminal accounting.  The second target should be answer-map preservation: an accepted certificate ending closed should expose the recorded council answer set that the runtime reports.  Aggregate-rule theorems should wait until the runtime defines an aggregate degree result; proving an aggregate before the output model exists would create proof work without a system claim to support.

## Work Order

| Order | System | Work |
| --- | --- | --- |
| 1 | ADC | Done: use `state.json` as the final-state artifact boundary and identify the certificate initialization schema. |
| 2 | ADC | Done: add runtime certificate writing and an explicit `adc verify-certificate` command. |
| 3 | ADC | Done: expose the certificate artifact through service artifact lists and fetch routes. |
| 4 | ADC | Done: prove accepted-certificate exact replay, replay-start reachability, closed-terminal accounting, and a concrete closed certificate. |
| 5 | ADC | Done: add accepted-certificate verdict and judgment accounting predicates, with replayed juror-vote and enter-judgment examples. |
| 6 | ADC | Done: settle failure accounting boundary.  Lawyer and nonjuror failures do not produce replay certificates today; juror failures are replayed `process_juror_timeout` transitions. |
| 7 | AARD | Done: add runtime certificate writing and an explicit `aard verify-certificate` command using `state.json`. |
| 8 | AARD | Done: expose the certificate artifact through service artifact lists and fetch routes. |
| 9 | AARD | Done: prove exact replay, reachability, terminal accounting, and replayed answer-pair exposure. |

## Non-Goals

Certificate work should not change case execution semantics.  It should record the engine-visible transition sequence already accepted by the runtime and check that sequence later.  It should not add service-side verification, change attestation verification, or treat lawyer and juror reasoning quality as a Lean property.

Certificate work should not create a shared abstract schema before ADC and AARD have concrete implementations.  ARB, ADC, and AARD share a pattern, but their initialization requests and terminal claims differ.  The useful commonality is the operator boundary: terminal packet artifact, explicit verifier command, service artifact exposure, and proof package at the accepted-certificate boundary.
