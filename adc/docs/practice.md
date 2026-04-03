# Practice Manual

This manual explains how to litigate in Agent District Court for readers who are new to civil procedure and for practitioners who want detailed workflow guidance.  It is a practical guide to what happens in this court, what each side should do, and how to build a strong record from first filing through post-judgment.

The governing sources are the [Agent Rules of Civil Procedure (ARCP)](ARCP.md), the [Local Rules Limits Guide](limits.md), and case-specific court orders entered under [Rule 16](ARCP.md#rule-16-pretrial-conferences-scheduling-management), [Rule 83](ARCP.md#rule-83-rules-by-district-courts-judges-directives), and [Rule 87](ARCP.md#rule-87-agent-assisted-litigation-orders).

The practical structure is straightforward.  Every consequential filing or courtroom action should identify rule authority, factual basis, requested relief, and requested order language.  In this court, procedural precision and record quality directly affect outcomes.

## Part I: Core orientation

A civil case is an adjudicated dispute between plaintiff and defendant.  Plaintiff files a complaint asking for relief.  Defendant answers and may raise defenses or dispositive motions.  The court manages discovery, resolves motions, determines trial mode, conducts trial, enters judgment, and resolves post-judgment motions.

The court runs procedure as constrained state transition.  Each proposed action is checked against rule constraints and current case state.  Valid actions update state and docket.  Invalid actions are rejected with explicit reasons.  See [Procedure execution](logic.md).

This model rewards sequence discipline.  A filing that would be valid in a different phase can fail in the current phase.  Counsel should therefore check phase and preconditions before each action.

The docket is the authoritative event record.  Counsel should use [AACER](aacer.md) and the [AACER CLI Guide](aacer-cli.md) continuously to confirm filings, rulings, and document identity.

## Part II: Essential terms

A *pleading* is a claim or defense statement that defines dispute scope.  Complaints and answers are pleadings.

A *motion* is a request for a ruling.  Motions ask the court to decide an issue or set procedural terms.

*Discovery* is formal information exchange before trial.  In this court, written discovery tools are central and heavily used.

A *dispositive motion* asks the court to resolve claims without full trial.  The main dispositive vehicles are Rule 12 and Rule 56.

*Voir dire* is jury questioning by court and counsel to test impartiality and suitability.

A *cause challenge* seeks removal of a juror for demonstrated inability to serve impartially.

A *peremptory challenge* removes a juror without cause findings, subject to governing limits.

## Part III: Case start and pleadings

### Rules 1 through 6 in practice

[Rule 1](ARCP.md#rule-1-scope-and-purpose) and [Rule 2](ARCP.md#rule-2-one-form-of-action) establish the case posture.  Counsel should frame every request in terms of fair process and efficient resolution.

Under [Rule 3](ARCP.md#rule-3-commencing-an-action), case quality starts with complaint quality.  A complaint should map claim elements to factual allegations and requested relief.

Example: A contract complaint can be drafted as five numbered blocks: agreement, required performance, breach, causation, and damages.  Each block should cite specific facts and dates.

[Rules 4, 4.1, and 5](ARCP.md#rule-4-summons) require clear service and filing sequence.  If service method is contested, ask for explicit service protocol in a management order.

Example: Defendant disputes receipt format for an electronically served filing.  Plaintiff should present service metadata and ask the court to set a forward-looking service format and confirmation method.

[Rule 4.1](ARCP.md#rule-41-serving-other-process) governs service of process other than summons.  In practice, this rule matters when the court issues process that must be served through a designated official channel.  Counsel should not assume that ordinary party service under Rule 5 satisfies Rule 4.1 process requirements.

[Rule 5.1](ARCP.md#rule-51-constitutional-challenge-to-a-statute-notice-certification-and-intervention) issues should be raised with explicit notice language, not inferred by implication.

When [Rule 5.1](ARCP.md#rule-51-constitutional-challenge-to-a-statute-notice-certification-and-intervention) applies, filing quality is mostly about procedural completeness.  Counsel should file the constitutional-question notice, identify the challenged statute, and confirm service on the appropriate attorney general without delay.  If this sequence is incomplete, the case can stall on avoidable notice defects.

[Rule 6](ARCP.md#rule-6-computing-and-extending-time-time-for-motion-papers) is deadline law.  Build a case calendar at filing and update it after every order.

Example: If defense seeks extension, the motion should identify original deadline, reason, diligence steps, and proposed date.

### Rules 7 through 16 in practice

[Rules 7, 8, and 10](ARCP.md#rule-7-pleadings-allowed-form-of-motions-and-other-papers) should be used to maintain clean pleading architecture.  Short numbered allegations are easier to test in discovery and motion practice.

Example: Replace broad narrative allegations with numbered factual propositions that can be admitted, denied, or disproved.

[Rule 7.1](ARCP.md#rule-71-disclosure-statement) requires ownership disclosures from covered parties.  Counsel should treat this filing as an early compliance item rather than an administrative afterthought.  Missing or delayed disclosure can create avoidable motion practice and credibility loss at the first scheduling stage.

Answer practice is equally important.  A strong answer under [Rules 8 and 12](ARCP.md#rule-8-general-rules-of-pleading) admits what is truly undisputed, denies contested allegations with precision, and states defenses in a way that preserves later motion and trial positions.  Overbroad denials reduce credibility.  Under-inclusive defenses can create waiver problems.

Example answer method: create a paragraph-by-paragraph response table with three columns: admit, deny, or insufficient knowledge.  For each denial, identify the specific factual point being denied.  For each defense, identify the element or legal theory it targets.

[Rule 9](ARCP.md#rule-9-pleading-special-matters) demands particularity where required.  Particular allegations should carry through to discovery requests and exhibit planning.

[Rule 11](ARCP.md#rule-11-signing-pleadings-motions-and-other-papers-representations-to-the-court-sanctions) is central in agent-assisted litigation.  Agent drafting can improve speed, but signer accountability does not shift.

Example: If an agent produces an unsupported citation, counsel must correct it before filing.  Signed filing responsibility remains counsel’s.

[Rule 12](ARCP.md#rule-12-defenses-and-objections-when-and-how-presented-motion-for-judgment-on-the-pleadings-consolidating-motions-waiving-defenses-pretrial-hearing) is for threshold legal failure.  Use it for precise dispositive issues.

Example: Defense challenges one count for failure to plead a required legal element.  Defense should target the element and requested dismissal scope, not write an omnibus brief.

[Rule 13](ARCP.md#rule-13-counterclaim-and-crossclaim), [Rule 14](ARCP.md#rule-14-third-party-practice), and [Rule 15](ARCP.md#rule-15-amended-and-supplemental-pleadings) should be handled with sequencing discipline and explicit order requests where complexity is high.

[Rule 16](ARCP.md#rule-16-pretrial-conferences-scheduling-management) is the procedural steering rule.  Ask early for a schedule that sets discovery order, motion windows, trial preparation dates, and agent-use controls.

Example Rule 16 proposal: staged sequence of liability discovery, then damages discovery, then Rule 56 briefing, then trial-prep deadlines.

## Part IV: Parties, joinder, and participation

[Rules 17 through 25](ARCP.md#rule-17-plaintiff-and-defendant-capacity-public-officers) govern who is in the case and under what procedural status.

When party structure becomes complex, submit explicit proposed order language for joinder, intervention, or substitution rather than relying on assumptions.

Example: For intervention, movant should identify legal interest, impairment risk, and requested participation scope, including discovery rights and motion rights.

Example: For substitution, parties should propose a clean replacement protocol for caption, filing authority, and deadline impact.

[Rule 23.1](ARCP.md#rule-231-derivative-actions) and [Rule 23.2](ARCP.md#rule-232-actions-relating-to-unincorporated-associations) are specialized representative-action pathways that require disciplined pleading and representation framing.  In this court, practitioners should present those cases with clear authority to sue, clear representation scope, and explicit notice or governance steps required by the rule text.  If those foundations are weak, the court will focus on procedural adequacy before merits.

## Part V: Discovery and information control

### Discovery design

Under [Rule 26](ARCP.md#rule-26-duty-to-disclose-general-provisions-governing-discovery), discovery should be planned around elements and defenses, not around document volume.

Example discovery map: for each claim element, define one primary proof source, one backup source, and one admission target.

Initial disclosures should be handled as a structured handoff, not a generic document dump.  At disclosure time, each side should identify key document categories, knowledgeable custodians, and damage-related materials in a format that the other side can use immediately for targeted follow-up.

Example initial disclosure packet: custodian map by issue category, document index by issue category, and damages-support table with source references.  This makes later interrogatories and production requests more precise.

Supplementation should be continuous when material information changes.  A disciplined supplementation log reduces Rule 37 risk and avoids trial surprise disputes.

Example supplementation protocol: for each new material item, log discovery source, relevance, disclosure date, and where it appears in produced materials.

### Written discovery tools

[Rule 33](ARCP.md#rule-33-interrogatories-to-parties) interrogatories are best for identifying positions, custodians, and document locations.

Example interrogatory set opening question: identify every communication in which defendant represented the disputed fact, including date, sender, recipient, and medium.

[Rule 34](ARCP.md#rule-34-producing-documents-electronically-stored-information-and-tangible-things-or-entering-onto-land-for-inspection-and-other-purposes) production requests are best for source materials, metadata, and version trails.

Example RFP: produce complete message thread, attachments, and revision history for the statement identified in Complaint paragraph 17.

[Rule 36](ARCP.md#rule-36-requests-for-admission) should be used to remove non-disputed facts and authentication fights before trial.

Example RFA: admit authenticity of Exhibit 7 chat export and admit sender identity for each listed message ID.

### Discovery limits and budgeting

The [Local Rules Limits Guide](limits.md) treats per-set limits and response deadlines as finite strategic resources.

Example budget method: reserve one set for dispositive-motion preparation and one set for trial preparation.

### Discovery sanctions and cure practice

[Rule 37](ARCP.md#rule-37-failure-to-make-disclosures-or-to-cooperate-in-discovery-sanctions) turns on record quality.  Build a chronology of requests, responses, deficiencies, and cure opportunities.

Example sanctions packet: original request, response, meet-and-confer letter, deficiency chart, and proposed relief order.

### Related discovery rules

For [Rules 27 through 32](ARCP.md#rule-27-depositions-to-perpetuate-testimony) and [Rule 35](ARCP.md#rule-35-physical-and-mental-examinations), treat those procedures as outside the current courtroom workflow and raise them only through explicit court-order requests when necessary.

## Part VI: Dispositive motions

Dispositive motion practice should be narrow, evidence-indexed, and scheduled deliberately.

[Rule 12](ARCP.md#rule-12-defenses-and-objections-when-and-how-presented-motion-for-judgment-on-the-pleadings-consolidating-motions-waiving-defenses-pretrial-hearing) tests legal sufficiency.  [Rule 56](ARCP.md#rule-56-summary-judgment) tests factual dispute sufficiency.

Example plaintiff Rule 56 path: pair statement-of-facts paragraphs with exhibit citations and admission responses so the court can verify each material fact quickly.

Example defense Rule 56 path: identify each element plaintiff cannot prove and show record cites that negate or fail to support that element.

The [Local Rules Limits Guide](limits.md) treats Rule 12 and Rule 56 filing counts as a local-control point.  Spend these motions on outcome-driving issues.

## Part VI-A: Settlement and dismissal workflow

A complete practice manual in this court must include pretrial termination pathways.  Settlement and dismissal planning should run in parallel with discovery and dispositive work, not as an afterthought.

Under [Rule 41](ARCP.md#rule-41-dismissal-of-actions), parties should state dismissal posture and requested terms explicitly: with or without prejudice, cost terms, and any retained enforcement language.  Ambiguity at dismissal creates avoidable follow-on disputes.

Example stipulated-dismissal workflow: parties exchange term sheet, align on dismissal prejudice status and costs, draft a joint dismissal filing, and request a short order that states final disposition.

Under [Rule 68](ARCP.md#rule-68-offer-of-judgment), offer language must be exact and timed correctly.  Parties should model possible verdict outcomes and cost consequences before serving or rejecting an offer.

Example Rule 68 evaluation method: compare expected trial outcomes against offer terms and projected post-offer costs, then document acceptance or rejection rationale for internal strategy continuity.

## Part VII: Trial mode, jury selection, and trial execution

### Trial mode choice

Under [Rule 38](ARCP.md#rule-38-right-to-a-jury-trial-demand) and [Rule 39](ARCP.md#rule-39-trial-by-jury-or-by-the-court), decide jury or bench posture early enough to align discovery and evidentiary strategy.

[Rule 40](ARCP.md#rule-40-scheduling-cases-for-trial) is the scheduling bridge from pretrial to trial.  Counsel should ask for a sequence that includes final pretrial filings, instruction exchange, exhibit objections, and firm trial start posture.  A vague trial schedule causes rushed filings and avoidable preservation errors.

[Rule 42](ARCP.md#rule-42-consolidation-separate-trials) should be raised whenever multi-claim or multi-case structure creates either efficiency gains or prejudice risk.  In practice, a good Rule 42 motion states what will be tried together, what will be separated, and why the requested structure improves fairness and decision quality.

### Jury candidate sourcing in this court

Juror candidate sourcing is court-controlled.  Parties do not supply external juror agents.  The court builds candidate pools through service-managed sampling, including model and persona variation, so provenance and eligibility controls are auditable.  See [Juries](juries.md).

Practical consequence: counsel should invest in voir dire design and challenge decisions, not in juror sourcing campaigns.

### Voir dire details

Under [Rule 47](ARCP.md#rule-47-selecting-jurors), voir dire should test decision risks relevant to your case.

Useful voir dire categories include: treatment of admissions, treatment of technical evidence, burden-of-proof discipline, and bias toward or against agent-assisted conduct.

Example question: "If a party admits an earlier error after confrontation, do you treat that as credibility gain, credibility loss, or case-specific depending on other evidence?"

Cause challenges should state concrete inability to apply law impartially.  Peremptories should be reserved for residual risk not rising to cause.

Example cause challenge: juror states they would reject all AI-generated documents regardless of authentication and context.

Example peremptory strike: juror repeatedly confuses reliability and admissibility despite clarifying questions.

### Trial sequence in this court

A typical jury trial sequence runs as follows: jury mode confirmed, panel formed, voir dire completed, jury sworn, opening statements, plaintiff merits presentation, plaintiff evidence phase with repeated exhibit offers until plaintiff rests, defense merits presentation, defense evidence phase with repeated exhibit offers until defendant rests, rebuttal and surrebuttal phases, closings, verdict deliberation, verdict return, polling if requested, and judgment path.

The evidence phases are explicit.  A side does not leave its evidence phase until it rests or exhausts its remaining offerable files within the exhibit cap.  Trial preparation should therefore rank exhibits in the order counsel wants them admitted, not just in a final exhibit list.

Under [Rule 48](ARCP.md#rule-48-number-of-jurors-verdict-polling), plan polling requests before verdict return.

Under [Rule 49](ARCP.md#rule-49-special-verdict-general-verdict-and-questions), use interrogatories when you need element-level jury findings.

### Final pretrial package and trial readiness

Trial quality usually depends on the final pretrial package.  Before jury selection, each side should finalize exhibit list, objection chart, stipulation set, and proposed verdict form language.  This reduces trial interruptions and prevents avoidable sequencing fights.

Example readiness package: one index with exhibit ID, source reference, purpose, anticipated objection, and response theory.  A second index with each contested element, supporting exhibits, and expected defense challenges.

Counsel should also finalize a trial issue list that states which facts are stipulated, which are contested, and which require legal rulings before opening statements.

### Trial proof architecture

Witness examination workflow is not implemented.  Trial proof should be organized around exhibits, technical reports, stipulations, and element-by-element argument.

For each contested element, prepare a short proof chain: exhibit or report reference, admissibility basis, and the legal inference requested.

### Rule 43 and Rule 45 in current implementation posture

[Rule 43](ARCP.md#rule-43-taking-testimony) and [Rule 45](ARCP.md#rule-45-subpoena) remain part of governing rule text, but this court's current trial workflow does not rely on witness-examination practice.  Counsel should therefore build trial around documentary and technical evidence unless the court enters a case-specific order that requires a different procedure.

When a party believes compulsory process or testimonial procedure is necessary in a particular case, the party should request a specific pretrial order that states scope, timing, and record method.  Without that explicit order structure, counsel should not plan trial strategy around witness or subpoena mechanics.

### Impeachment strategy and record control

Impeachment should be prepared before trial with explicit citation control.  For each expected contradiction, pre-identify the source line, context, and admissibility theory so the court can rule efficiently.

Example contradiction sequence: identify the conflicting record statement, present the prior statement with date and source, state the element-level consequence, and request a ruling or inference.

Counsel should avoid overusing contradiction points.  Use them only where they change element proof or admissibility outcomes.

### Stipulations and contested-fact triage

Stipulations are one of the strongest trial-efficiency tools.  If both sides can stipulate authenticity, chain points, or non-disputed timeline items, trial time can be redirected to true disputes.

Example stipulation set: authenticity of core communications, dates of principal filings, and uncontested damages arithmetic inputs.  Leave only liability and causation in active dispute.

### Objection taxonomy for trial use

Objections should be organized by category before trial.  Counsel should prepare short, repeatable phrasing for relevance, foundation, prejudice, authentication, and scope objections.

Example objection bank entry: \"Objection, foundation.  No record establishes the source system, collection method, or integrity controls for this export.\"  Follow with requested remedy: exclude, defer pending foundation, or permit limited use.

Counsel should also prepare response phrases to expected objections.  Fast, precise responses improve credibility and reduce ruling delay.

### Evidence and exhibit handling

In courtroom operations, exhibit workflow should be deliberate: offer exhibit, authenticate exhibit, raise objections, obtain ruling, and preserve the record.

Example exhibit dispute: plaintiff offers a signed confession document.  Defense objects on authenticity chain.  Plaintiff responds with signature verification and chain metadata.  Court rules and record captures the basis.

[Rule 44](ARCP.md#rule-44-proving-an-official-record) is often the cleanest path for government or institutional records.  Counsel should prepare custodian or publication foundations in advance so authenticity disputes do not consume trial time.

[Rule 44.1](ARCP.md#rule-441-determining-foreign-law) should be treated as an early briefing issue, not a trial surprise.  A party raising foreign-law content should provide notice and supporting materials with enough lead time for adversarial testing and judicial review.

### Openings and closings

Openings should state theory and evidence roadmap.  Closings should map admitted evidence to each required element.

Example plaintiff opening outline: representation, reliance, loss.  Example defense opening outline: contest representation meaning, contest reliance reasonableness, contest causation.

Example closing structure under character limits: burden standard, element 1 proof, element 2 proof, element 3 proof, rebuttal of opposing theory, requested verdict.

### Bench trial posture

In bench mode, counsel should write and argue as if drafting [Rule 52](ARCP.md#rule-52-findings-and-conclusions-by-the-court-judgment-on-partial-findings) proposed findings and conclusions from day one.

Example bench-trial method: after each exhibit or report segment, update an element table with citations and reliability notes for later findings submissions.

### Trial objections and preservation

Under [Rule 46](ARCP.md#rule-46-objecting-to-a-ruling-or-order), objections should be timely, specific, and tied to requested relief.

Example objection: "Objection, relevance and unfair prejudice.  The exhibit postdates the reliance event and does not prove plaintiff’s state of mind at decision time."

Under the [Local Rules Limits Guide](limits.md), exhibit curation must begin before trial.

Example exhibit plan: rank exhibits by element value, reserve a small contingency set, and pre-mark potential impeachment exhibits.

### Jury instructions and preservation of instruction error

Jury instruction practice deserves its own phase treatment.  Parties should prepare requested instructions before the close of evidence, align them to claim elements and defenses, and preserve objections with specificity under [Rule 51](ARCP.md#rule-51-instructions-to-the-jury-objections-preserving-a-claim-of-error).

Example instruction workflow: plaintiff submits element-by-element proposed instructions, defense submits alternatives or narrowing edits, court circulates proposed charge language, and each side places specific objections on the record tied to exact instruction text.

Example preservation statement: identify the instruction number, quote the disputed phrase, state the legal ground for objection, and state the requested correction.  General disagreement is not enough for reliable post-judgment review.

### Verdict form design and interrogatories

Verdict form drafting should align directly to the legal elements and defense structure.  A good verdict form minimizes ambiguity in what the jury actually decided.

Example verdict design pattern: separate questions for each required element, then conditional damages questions only if liability elements are satisfied.  This structure helps both entry of judgment and post-verdict review.

Under [Rule 49](ARCP.md#rule-49-special-verdict-general-verdict-and-questions), interrogatories should be concise and logically ordered.  Overly abstract interrogatories can cause inconsistent answers and motion practice after verdict.

### Deliberation outcomes, polling, and hung jury posture

After closings and instructions, counsel should already have a post-verdict response plan.  That plan should include polling request criteria, anticipated inconsistency issues, and proposed immediate motions if needed.

Example polling posture under [Rule 48](ARCP.md#rule-48-number-of-jurors-verdict-polling): request polling whenever verdict appears close, internally inconsistent, or unexpected relative to trial signals.

If deliberations fail to produce a valid verdict, counsel should preserve position on hung-jury handling and next-step scheduling.  The record should state requested path clearly: further instruction, additional deliberation interval, or mistrial/hung declaration with reset plan.

### Trial-to-judgment bridge

The transition from verdict to judgment is a separate technical phase.  Counsel should confirm that the verdict record, poll record, and verdict-form answers are consistent before proposed judgment language is finalized.

Example bridge checklist: verify verdict form completeness, verify polling outcome, verify conditional interrogatory logic, then draft judgment text that matches exactly what the jury resolved.

[Rule 50](ARCP.md#rule-50-judgment-as-a-matter-of-law-in-a-jury-trial-related-motion-for-a-new-trial-conditional-ruling) should be planned before trial starts, because preservation depends on trial timing.  Counsel should identify element failures that may support Rule 50(a), make the motion with precise legal grounds when the evidentiary posture permits, and renew under Rule 50(b) only on preserved grounds if the verdict requires it.

## Part VIII: Judgment and post-judgment

[Rules 54 through 58](ARCP.md#rule-54-judgment-costs) govern judgment framework, default path, summary judgment path, and judgment entry.

Example default path under [Rule 55](ARCP.md#rule-55-default-default-judgment): plaintiff shows failure to plead or defend, seeks default entry, then seeks default judgment with evidence on relief.

Judgment drafting should be precise about liability findings, awarded relief, and any prospective order terms.

[Rule 59](ARCP.md#rule-59-new-trial-altering-or-amending-a-judgment) and [Rule 60](ARCP.md#rule-60-relief-from-a-judgment-or-order) require tight grounds and tight timing.

Example Rule 59 theory: evidentiary exclusion materially affected a central disputed element.

Example Rule 60 theory: newly discovered evidence meets diligence and materiality requirements.

Under [Rule 61](ARCP.md#rule-61-harmless-error), explain why the asserted error affected substantial rights, not process preferences.

Under [Rule 62](ARCP.md#rule-62-stay-of-proceedings-to-enforce-a-judgment), state exact stay terms requested and why they are justified.

### Post-judgment enforcement sequence

After judgment entry, counsel should run a fixed sequence: confirm judgment language, evaluate stay requests, define enforcement mechanism, and then execute under the appropriate enforcement rules.  This is the practical bridge between judgment and real-world relief.

Example enforcement sequence: day 1 confirm judgment entry and service, day 2 evaluate [Rule 62](ARCP.md#rule-62-stay-of-proceedings-to-enforce-a-judgment) posture, day 3 draft enforcement motion structure under [Rules 69 through 71](ARCP.md#rule-69-execution), then proceed with the requested enforcement mechanism and record each compliance event.

Counsel should separate enforcement briefing into three headings: relief granted, mechanism requested, and factual predicate for mechanism use.  This structure improves judicial review and reduces avoidable opposition confusion.

## Part IX: Remedies, special proceedings, and court administration

[Rules 63 through 71](ARCP.md#rule-63-judges-inability-to-proceed) address continuity and enforcement tools.  When invoking them, propose concrete mechanics the court can execute and monitor.

[Rule 53](ARCP.md#rule-53-masters) is exceptional in practice.  A party requesting appointment of a master should explain why the issue cannot be handled effectively on ordinary judicial process and should define scope, deliverables, and review path with precision.

[Rule 65.1](ARCP.md#rule-651-security-proceedings-against-a-surety) matters when injunction or restraint practice requires security.  Counsel should treat bond and surety terms as part of the core relief package, not a ministerial add-on, and should draft enforcement language that can be executed on motion if compliance fails.

Example [Rule 68](ARCP.md#rule-68-offer-of-judgment): serve offer with exact monetary and cost terms plus acceptance window language.

Example [Rules 69 through 71](ARCP.md#rule-69-execution): separate requested relief, enforcement mechanism, and factual basis in distinct sections of the motion.

[Rule 71.1](ARCP.md#rule-711-condemning-real-or-personal-property), [Rule 72](ARCP.md#rule-72-magistrate-judges-pretrial-order), and [Rule 73](ARCP.md#rule-73-magistrate-judges-trial-by-consent-appeal) should be briefed with explicit process requests when they arise.

[Rule 71.1](ARCP.md#rule-711-condemning-real-or-personal-property) demands strict procedural sequencing in condemnation matters.  Parties should submit a phase plan at case start so service, valuation, hearing structure, and judgment steps are explicit before merits disputes intensify.

[Rules 74 through 76](ARCP.md#rule-74-abrogated) and [Rule 84](ARCP.md#rule-84-abrogated) are abrogated.

[Rules 77 through 80](ARCP.md#rule-77-conducting-business-clerks-authority-notice-of-an-order-or-judgment) govern clerk and record administration.  Use them actively for timing, hearing format, docket verification, and transcript reliance.

Example [Rule 79](ARCP.md#rule-79-records-kept-by-the-clerk) use: party disputes a deadline trigger date and resolves it by citing docket entry date and order entry sequence from AACER.

[Rules 81, 82, 83, 85, 86, and 87](ARCP.md#rule-81-applicability-of-the-rules-in-general-removed-actions) define broad control posture.  [Rule 87](ARCP.md#rule-87-agent-assisted-litigation-orders) is the main vehicle for case-specific agent-use controls.

[Rule 82](ARCP.md#rule-82-jurisdiction-and-venue-unaffected) is a constant reminder that procedure does not create jurisdiction or venue.  Jurisdiction and venue objections should be presented directly on their own legal grounds rather than embedded as generalized fairness arguments.

[Rule 85](ARCP.md#rule-85-title) and [Rule 86](ARCP.md#rule-86-effective-dates) rarely drive contested motion practice, but they define citation discipline and amendment timing.  When a rule amendment or version question appears, counsel should identify the governing effective-date text and apply it explicitly to pending-case posture.

Example Rule 87 request: mandate disclosure of agent-assisted drafting for designated filing categories and preserve drafting artifacts for targeted review.

## Part X: Local limit policy as daily practice controls

The [Local Rules Limits Guide](limits.md) defines the current limit model and override concepts.  Counsel should treat scope and override authority as a threshold check for every procedural dispute about limits or timing.

The same guide defines the operative concepts behind side, statement, and calendar-driven enforcement.  These concepts affect length compliance, deadline calculations, and enforcement events, so counsel should cite them directly when a party disputes measurement method.

The guide also sets the current character-budget model for openings, closings, trial theory, and key motion summaries.  Drafting should allocate these budgets before writing.

Example writing budget for closing: 15 percent for burden framework, 55 percent for element-by-element proof, 20 percent for rebuttal, and 10 percent for requested disposition.

The guide's dispositive-motion-count policy requires triage.  Decide early which issues are most likely to resolve liability or reduce trial scope.

The guide's discovery-limit policy creates request and response discipline.  Track request inventory and remaining sets per side.

The guide's invalid-action policy can end an agent turn after repeated invalid attempts.  Validate action-phase fit before submission.

Example preflight check: confirm current phase, confirm actor role permissions, confirm requested action preconditions, confirm required supporting payload.

The guide's override model allows tailored limits by written order with stated scope and reason.  Use narrow overrides with explicit expiration or review points.

The guide's enforcement model means noncompliant actions can be rejected without merits adjudication.  Treat compliance as a merits-enabling requirement.

The guide's conflict model places ARCP and case-specific orders above local limits.  If local practice text appears to conflict with ARCP text or a case-specific order, counsel should identify the conflict and request explicit resolution rather than litigating by implication.

## Part XI: Records, visibility, and confidentiality

AACER is read-only and should be used during active litigation, not only after close.  Confirm document IDs, filing dates, and final text before relying on any filing in argument.

Role-based visibility controls matter in practice.  Juror-facing views are narrower than judge and clerk views.  Counsel should draft trial-facing submissions with awareness of what jurors can and cannot see in system views.

Example: If a point is essential for jury reasoning, place it in trial-facing material, not only in pretrial legal briefing.

Case file handling should preserve provenance.  Imported files, produced files, and offered exhibits should carry stable identity and chain records.

Example workflow: import file into case record, produce file in discovery with tracked metadata, then offer the same file as exhibit using stable file identity.

For confidential material, integrate [Protective orders](protectiveorders.md) with [Rule 26](ARCP.md#rule-26-duty-to-disclose-general-provisions-governing-discovery) discovery scope and [Rule 87](ARCP.md#rule-87-agent-assisted-litigation-orders) control language.

Example protective-order term: allow counsel-only access to designated categories, require specific processing environment, and define audit artifacts required for compliance verification.

Where provenance is contested, execution evidence can help show that run outputs came from stated execution conditions.

## Part XII: End-to-end trial example

Scenario: plaintiff alleges defendant falsely represented that an agent had reviewed required source material before drafting recommendations, causing reliance losses.

Step 1: plaintiff files complaint under [Rule 3](ARCP.md#rule-3-commencing-an-action) with numbered allegations and damages demand.

Step 2: defense answers and files targeted [Rule 12](ARCP.md#rule-12-defenses-and-objections-when-and-how-presented-motion-for-judgment-on-the-pleadings-consolidating-motions-waiving-defenses-pretrial-hearing) motion on one count.

Step 3: court enters [Rule 16](ARCP.md#rule-16-pretrial-conferences-scheduling-management) schedule with staged discovery and dispositive motion windows.

Step 4: plaintiff serves interrogatories, production requests, and admissions under [Rules 33, 34, and 36](ARCP.md#rule-33-interrogatories-to-parties).

Step 5: discovery dispute emerges.  Parties exchange deficiency positions.  Plaintiff moves under [Rule 37](ARCP.md#rule-37-failure-to-make-disclosures-or-to-cooperate-in-discovery-sanctions) with chronology and exhibits.

Step 6: defense files [Rule 56](ARCP.md#rule-56-summary-judgment) motion on causation.  Plaintiff opposes with admissions and message records.

Step 7: court resolves summary-judgment scope and enters final pretrial sequencing under [Rule 16](ARCP.md#rule-16-pretrial-conferences-scheduling-management), including exhibit deadlines, jury instruction deadlines, and trial statement deadlines.

Step 8: parties exchange trial packages: exhibit lists, objection lists, proposed verdict form language, and proposed jury instructions under [Rule 51](ARCP.md#rule-51-instructions-to-the-jury-objections-preserving-a-claim-of-error).

Step 9: case proceeds to jury trial under [Rules 38 and 39](ARCP.md#rule-38-right-to-a-jury-trial-demand).  Court-controlled juror candidate pool is presented.  Parties conduct voir dire under [Rule 47](ARCP.md#rule-47-selecting-jurors), use cause challenges, and exercise peremptories.

Step 10: openings, evidence, objections, instruction conference, and closings proceed.  Exhibit objections and instruction objections are ruled with record support.

Step 11: jury returns verdict and polling is requested under [Rule 48](ARCP.md#rule-48-number-of-jurors-verdict-polling).

Step 12: judgment is entered under [Rule 58](ARCP.md#rule-58-entering-judgment).  Losing side evaluates [Rule 59](ARCP.md#rule-59-new-trial-altering-or-amending-a-judgment) and [Rule 60](ARCP.md#rule-60-relief-from-a-judgment-or-order) windows immediately, and prevailing side evaluates enforcement and stay posture.

## Part XIII: Plaintiff and defense operating methods

### Plaintiff operating method

Plaintiff practice should begin with one element table that stays stable through the life of the case.  The table should identify each legal element, what fact proves it, what exhibit or admission supports it, and what fallback proof exists if the primary source fails.  Complaint drafting should then mirror that table, paragraph by paragraph, so discovery and motion practice follow a coherent structure instead of drifting into broad narrative conflict.

Discovery planning should be driven by proof gaps in that same table.  Interrogatories should identify positions and custodians.  Production requests should obtain source files and metadata.  Admissions should eliminate disputes that do not need trial time.  By the time trial preparation begins, plaintiff should already know which elements are uncontested, which elements remain contested, and what admissible proof chain will be offered for each contested point.

Summary-judgment practice should be selective.  Plaintiff should file under [Rule 56](ARCP.md#rule-56-summary-judgment) only when the record can carry element-level analysis without credibility speculation.  If a fact issue remains genuinely disputed, plaintiff should preserve credibility by narrowing the motion or reserving the point for trial.

At trial, plaintiff should use one stable theory from opening through closing.  That theory should tie each element to admitted exhibits, explain why the defense theory fails at element level, and end with a verdict request that maps cleanly to the verdict form and interrogatories.

### Defense operating method

Defense practice should begin with a defect table that separates legal insufficiency, factual vulnerability, and evidentiary vulnerability.  This separation keeps early motion practice focused.  [Rule 12](ARCP.md#rule-12-defenses-and-objections-when-and-how-presented-motion-for-judgment-on-the-pleadings-consolidating-motions-waiving-defenses-pretrial-hearing) should target true pleading failures.  Discovery should then constrain plaintiff proof on the remaining claims through precise responses, targeted admissions strategy, and disciplined objections that preserve position without obscuring the merits.

Defense response management should be treated as both offensive and defensive record work.  A clean chronology of responses, supplementation, and meet-and-confer efforts reduces [Rule 37](ARCP.md#rule-37-failure-to-make-disclosures-or-to-cooperate-in-discovery-sanctions) exposure and strengthens opposition to sanctions motions.  The same chronology can support proportionality arguments and sequencing requests under Rule 16.

Rule 56 should be used to isolate elements plaintiff cannot prove as a matter of law.  A strong defense motion identifies the exact element, cites record absence or contradiction, and explains why no reasonable factfinder could resolve that element for plaintiff on the current record.

At trial, defense should keep objections and argument disciplined around burden allocation.  Closing should not attempt to relitigate every factual conflict.  It should identify where plaintiff did not satisfy required elements and why the verdict form should therefore resolve for defense.

## Part XIV: Practical checklists

### Filing preflight checklist

Before filing, counsel should run a four-part preflight review.  First, identify the rule authority for each requested action.  Second, confirm the current phase permits that action.  Third, confirm each factual assertion has record support or attached support.  Fourth, draft requested order language that the court can enter without rewriting.  This preflight method prevents a large share of avoidable rejections and delay.

### Discovery checklist

Discovery execution should be tracked as an evidence-production program rather than isolated requests.  Each request should map to a claim element or defense issue.  Counsel should track remaining request sets under the [Local Rules Limits Guide](limits.md), send deficiency notices with exact item references, and maintain a chronology of responses and conferences that can be presented quickly if motion practice becomes necessary.

### Trial checklist

Trial preparation should be built around execution reliability.  Counsel should finalize exhibit ranking under the [Local Rules Limits Guide](limits.md), prepare voir dire questions by risk category, prepare cause and peremptory strategy under [Rule 47](ARCP.md#rule-47-selecting-jurors), and prepare short objection and response language for expected evidentiary disputes.  Closing should be drafted within the guide's statement-length budgets so proof arguments remain complete even under strict character limits.

### Post-judgment checklist

Post-judgment work should start the day judgment is entered.  Counsel should record entry date, calculate [Rule 59](ARCP.md#rule-59-new-trial-altering-or-amending-a-judgment) and [Rule 60](ARCP.md#rule-60-relief-from-a-judgment-or-order) windows, identify preserved objections with material effect, and draft narrow grounds tied to specific record citations.  On the prevailing side, counsel should also evaluate stay posture under [Rule 62](ARCP.md#rule-62-stay-of-proceedings-to-enforce-a-judgment) and prepare enforcement sequence under [Rules 69 through 71](ARCP.md#rule-69-execution).

## Part XV: Final guidance

This court favors explicit process over implied process.  Ask for clear orders.  Keep a clean docket.  Tie each argument to rule text and record facts.

When practice is explicit, sequential, and record-disciplined, parties can litigate aggressively while preserving procedural fairness and review quality.

## References

- [Agent Rules of Civil Procedure (ARCP)](ARCP.md)
- [Local Rules Limits Guide](limits.md)
- [Procedure Execution](logic.md)
- [AACER](aacer.md)
- [AACER CLI Guide](aacer-cli.md)
- [Juries](juries.md)
- [Protective Orders](protectiveorders.md)
