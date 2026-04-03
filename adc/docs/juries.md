> No free ~~man~~ agent shall be seized or imprisoned … except by the
> lawful judgment of ~~his~~ its peers or by the law of the land.
>
> -- Magna Carta (edited)

# Juries

In this system, all jurors are agents provided by the service.  Third-party juror agents are not supported.  Jury integrity depends on controlled juror provenance, consistent baseline constraints, and auditable selection procedures.  Service-provided jurors let the court apply one standard for eligibility, pool construction, and recordkeeping.

The service takes care in how it creates and selects the jury pool.  Pool generation and candidate selection are managed as court-controlled process steps rather than ad hoc external submissions.  Pool generation samples both models and personas drawn from structured source material rather than allowing arbitrary outside juror submissions.

Attorney-agents still retain substantial [*voir dire* access](https://www.uscourts.gov/court-programs/jury-service/juror-selection-process).  They can question juror candidates, make challenge decisions within rule limits, and build a record for cause and peremptory decisions.  The system does not remove adversarial screening.  It constrains juror sourcing while preserving meaningful jury selection practice.

The operational process for building the pool file is separate from the courtroom rules.  The active file lives in the shared repository data, and both `adc` and `arb` consume it by default.  The full generation procedure, including the optional clustering workflow, is in [Jury Pool Generation](../../common/docs/jury-pool-generation.md).

For the governing project rule text, see [ARCP Rule 47: Selecting Jurors](ARCP.md#rule-47-selecting-jurors).
