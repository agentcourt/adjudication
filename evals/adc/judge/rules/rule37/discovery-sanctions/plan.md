# Rule 37 Judge Eval Plan

## Scope

This eval measures the judge's `decide_rule37_motion` behavior.  Each fixture builds a pretrial ADC state in the discovery phase with a discovery request, a response record, meet-and-confer text when relevant, a Rule 37 motion, and opposition text.  The runner obtains the real Lean judge opportunity, asks the model for one tool call, applies the decision through Lean, and scores the ruling and sanction payload.

The first fixture set focuses on the boundary between a concrete discovery failure and an ordinary or justified discovery dispute.  It covers no response, complete response, evasive response, privilege and work-product objections, overbreadth, proportionality, harmless cure, initial-disclosure failure, prior-order violation, RFA nonresponse under Rule 36, premature motion practice, grant without fees, and fee-only requests after production.  The highest-risk failure in this first set is a schema-valid or schema-invalid sanction decision that grants fees on a denied motion or omits the required sanction fields.

## Fixture Shape

| Field | Meaning |
|---|---|
| `id` | Stable row identifier. |
| `tier` | Difficulty level, with tier 3 for close sanction and cure boundaries. |
| `issue_family` | Summary slice for the discovery-dispute family. |
| `case_theme` | Short factual setting placed in the case caption and docket. |
| `movant` | Party seeking Rule 37 relief. |
| `target_party` | Opposing party whose discovery conduct is challenged. |
| `discovery_type` | `interrogatories`, `rfp`, `rfa`, or `initial_disclosures`. |
| `set_index` | Discovery set index used in docket text and prompt context. |
| `request_text` | Discovery request or disclosure obligation. |
| `response_text` | Response, objection, cure, or nonresponse record. |
| `meet_and_confer_text` | Optional meet-and-confer chronology. |
| `motion_text` | Movant's requested Rule 37 relief. |
| `opposition_text` | Target party's reason for grant, denial, or sanction limits. |
| `expected_granted` | Expected grant or denial. |
| `expected_sanction_type` | `none` or `fees`. |
| `expected_sanction_amount` | Required amount when fees are expected. |
| `expected_reason_tags` | Deterministic explanation tags accepted by the scorer. |
| `severity` | Weight used in weighted accuracy. |
| `context_notes` | Human-readable explanation of the fixture boundary. |

## Scoring

The scorer requires exactly one `decide_rule37_motion` tool call.  It checks `motion_index`, `granted`, `sanction_type`, `sanction_amount`, `order_text`, `reasoning`, reason tags, and Lean acceptance.  A denied motion with `sanction_type: fees`, a fee award without a positive amount, or a missing required sanction type is invalid before Lean application because the payload cannot represent a valid Rule 37 order.

The summary reports total accuracy, grant accuracy, weighted accuracy, invalid rate, false-grant rate, false-denial rate, sanction mismatches, and slices by reason tag, issue family, tier, movant, and expected sanction type.  Explanation scoring remains deterministic and uses accepted equivalents for ordinary wording such as complete response, cured defect, proportionality, substantially justified objection, and deemed-admitted RFAs.  The runner supports `--rescore-results` so scorer vocabulary corrections can be applied to completed live outputs without repeating model calls.

## Prompt Iteration

Production made correct grant-or-denial decisions for the grant rows but returned invalid sanction payloads on most denial rows.  The repeated error was `granted: false` paired with `sanction_type: fees`, which Lean rejects and which fails the eval before order reasoning can be considered.  The first live production run also exposed a mislabeled RFA fixture; ARCP Rule 36 treats an unanswered RFA as admitted, so that row now expects denial rather than a compelled response.

Candidate v1 fixed most denial-row sanction handling and improved the measured set from 9/16 to 15/16, but it omitted `sanction_type` on the fee-only denial row.  Candidate v2 makes the payload requirement explicit: every tool call must include `sanction_type`, denied motions must use `none`, and `fees` applies only to granted motions with a fee award.  Candidate v2 is the best measured Rule 37 prompt on this fixture set.
