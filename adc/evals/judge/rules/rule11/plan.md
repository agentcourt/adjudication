# Rule 11 Judge Eval Plan

## Scope

This eval measures the judge's `decide_rule11_motion` behavior.  Each fixture builds a filed-stage ADC state with a complaint, a challenged filing, a Rule 11 safe-harbor notice, an optional correction record, a Rule 11 motion, and opposition text.  The runner obtains the real Lean judge opportunity, asks the model for one tool call, applies the payload through Lean, and scores the ruling, sanction fields, explanation tags, and Lean acceptance.

The first fixture set focuses on restraint and enforcement.  It includes frivolous legal contentions, factual contentions with no evidentiary support, factual denials contradicted by records available before filing, improper-purpose filings, nonfrivolous extension arguments, allegations likely to have support after discovery, weak merits positions, safe-harbor defects, timely correction, and discovery conduct that belongs under Rule 37.  The highest-risk failures are false grants against legitimate advocacy, invalid sanction payloads on denied motions, and disproportionate monetary sanctions.

## Fixture Shape

| Field | Meaning |
|---|---|
| `id` | Stable row identifier. |
| `tier` | Difficulty level, with tier 3 for adversarial or close boundaries. |
| `issue_family` | Summary slice for the Rule 11 issue family. |
| `case_theme` | Short factual setting placed in the case caption and docket. |
| `movant` | Party seeking Rule 11 sanctions. |
| `target_party` | Party whose filing is challenged. |
| `challenged_filing` | Name of the filing or paper under Rule 11 review. |
| `filing_text` | Challenged filing content. |
| `notice_text` | Safe-harbor notice text. |
| `notice_served_at` | Safe-harbor service date. |
| `motion_filed_at` | Rule 11 motion filing date. |
| `correction_text` | Optional withdrawal or correction during the safe-harbor period. |
| `motion_text` | Movant's Rule 11 motion and requested sanction. |
| `opposition_text` | Target party's reason for denial or sanction limits. |
| `expected_granted` | Expected grant or denial. |
| `expected_sanction_type` | `none`, `admonition`, `non_monetary_directive`, `monetary_penalty`, or `fee_shift`. |
| `expected_sanction_amount` | Required amount when the expected sanction is monetary. |
| `expected_reason_tags` | Deterministic explanation tags accepted by the scorer. |
| `severity` | Weight used in weighted accuracy. |
| `context_notes` | Human-readable explanation of the fixture boundary. |

## Scoring

The scorer requires exactly one `decide_rule11_motion` tool call.  It checks `motion_index`, `granted`, `sanction_type`, `sanction_amount`, `sanction_detail`, `reasoning`, deterministic reason tags, and Lean acceptance.  Denied motions are invalid if they include a nonempty `sanction_type`, any nonzero `sanction_amount`, or a nonempty `sanction_detail`, because Lean treats those fields as sanctions despite the denial.

Granted motions require a nonempty sanction type and sanction detail.  `monetary_penalty` and `fee_shift` require a positive amount, while `admonition` and `non_monetary_directive` reject monetary amounts.  The summary reports total accuracy, grant accuracy, weighted accuracy, invalid rate, false-grant rate, false-denial rate, sanction mismatches, and slices by reason tag, issue family, tier, movant, and expected sanction type.

## Prompt Iteration

Production made several correct substantive denials but returned invalid denial payloads.  The repeated denial payload included `sanction_type: "none"` and a no-sanctions explanation in `sanction_detail`, which Lean rejects because denied Rule 11 orders cannot include sanction fields.  Production also chose fee shifting on two granted rows without an amount, producing invalid monetary-sanction payloads.

Candidate v1 fixed the invalid payload cluster by stating the denial payload rule and monetary-sanction amount rule directly.  It scored 15/16 live, with no invalid payloads and no grant-or-denial errors, but it used `non_monetary_directive` on a first limited legal defect that the fixture expected to receive an admonition.  Candidate v2 narrowed the sanction ladder by reserving directives for correction-focused records or motion-specific requests, and it is the best measured Rule 11 prompt on this fixture set.
