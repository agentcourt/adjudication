# AACER

AACER stands for Agent Access to Court Electronic Records.  AACER is inspired by [PACER (Public Access to Court Electronic Records)](https://pacer.uscourts.gov/).
AACER provides read access to court records through three interfaces: a JSON/HTTP API, a command-line API, and a web site.

AACER is read-only.  Its purpose is to make court records easy to discover, review, and retrieve without exposing write paths into case state.

## Goals and benefits

AACER serves three goals.  First, it provides one consistent way to access court records across machine and human interfaces.  Second, it provides a stable document model for automation, review, and audit.  Third, it preserves strict separation between record access and adjudication.

The benefits are practical.  Integrations can consume records through structured JSON responses.  Operators can use command-line tools for scripted workflows.  End users can browse records through a web interface with the same core data model.

## Interface overview

AACER supports three access patterns.  The JSON/HTTP API supports programmatic listing and retrieval.  The command-line API supports operational scripting and inspection.  The web site supports direct, human-oriented document browsing.  All three interfaces expose the same basic workflow: identify a case, list documents, then retrieve the selected document.

For command-line details, see the [AACER CLI Guide](aacer-cli.md).

## Example docket

This table is a reduced and simplified presentation of the docket from the checked-in [Example 1](../examples/ex1/README.md).  It omits many entries and shortens long descriptions for clarity.  The full record remains in the example artifacts.

| No. | Docket entry | Short description |
|---:|---|---|
| 1 | Complaint filed | Peter alleges that Sam breached the engagement by failing to do required reading before drafting work in a paid commercial engagement. |
| 2 | Complaint attachments filed | `instructions.txt`, `session-summary.txt`, `confession.txt`, signature materials, and four damages-support files enter the record with the complaint. |
| 8 | Answer filed | Sam answers and contests liability and damages. |
| 9 | Initial Disclosures | Plaintiff serves initial disclosures. |
| 10 | Technical report - plaintiff | `TR-P1`: digital signature verification of the confession. |
| 11 | Technical report - defendant | `TR-D1`: technical review of the claimed `$108,000` damages figure. |
| 18 | Rule 37 Motion | Plaintiff moves on a discovery dispute. |
| 19 | Rule 37 Order | Motion denied.  Court finds no unresolved discovery failure warranting compulsion or sanctions. |
| 20 | Pretrial Order | Claim preserved for trial.  Documentary exhibits preserved. |
| 21 | Voir dire | The court issues questionnaires, screens lawyer questions, rules on for-cause challenges, records peremptories, and empanels six jurors. |
| 22 | Opening statement - plaintiff | Plaintiff opens on the contract theory and the documentary record. |
| 23 | Opening statement - defendant | Defense narrows the dispute and attacks damages and reliance. |
| 26 | Plaintiff exhibits admitted | `PX-1` through `PX-9` are admitted, covering the assignment, session summary, confession, signature materials, and damages support. |
| 39 | Closing argument - plaintiff | Plaintiff ties contract formation, breach, reliance, and damages to the admitted record. |
| 40 | Closing argument - defendant | Defense argues that the lie did not cause the claimed loss. |
| 43 | Jury instructions settled | Final neutral charge on the contract claim and the burden of proof. |
| 44 | Jury instructions delivered | Court delivers the charge. |
| 46 | Jury vote record | Each sworn juror casts an individual vote with an explanation tied to the admitted exhibits. |
| 55 | Jury poll | Unanimous poll.  All six jurors assent. |
| 56 | Judgment entered | Judgment on the jury verdict. |
