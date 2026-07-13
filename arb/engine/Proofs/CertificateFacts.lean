import Proofs.DecisionSummary

namespace ArbProofs

structure ClosedCertificateFacts
    (req : InitializeCaseRequest)
    (actions : List CourtAction)
    (claimed : ArbitrationState) : Prop where
  replay_exact :
    replayInitialized req actions = .ok claimed
  reachable :
    Reachable claimed
  length_bound :
    ∃ start,
      initializeCase req = .ok start ∧
        actions.length ≤ 2 * start.policy.max_submitted_evidence_per_side +
          8 + start.policy.max_deliberation_rounds * start.policy.council_size
  terminal_accounted :
    claimed.case.status = "closed" ∧
      claimed.case.phase = "closed" ∧
        (claimed.case.resolution = "demonstrated" ∨
          claimed.case.resolution = "not_demonstrated" ∨
            claimed.case.resolution = "no_majority")
  outcome_sound :
    (claimed.case.resolution = "demonstrated" ∧ demonstratedOutcomeSound claimed) ∨
      (claimed.case.resolution = "not_demonstrated" ∧ notDemonstratedOutcomeSound claimed) ∨
        (claimed.case.resolution = "no_majority" ∧ noMajorityOutcomeSound claimed)
  merits_complete :
    bilateralComplete "openings" claimed.case.openings ∧
      bilateralComplete "arguments" claimed.case.arguments ∧
        plaintiffOptionalSequence "rebuttals" claimed.case.rebuttals ∧
          defendantOptionalSequence "surrebuttals" claimed.case.surrebuttals ∧
            bilateralComplete "closings" claimed.case.closings
  filing_counts :
    filingCount claimed.case.openings "plaintiff" = 1 ∧
      filingCount claimed.case.openings "defendant" = 1 ∧
        filingCount claimed.case.arguments "plaintiff" = 1 ∧
          filingCount claimed.case.arguments "defendant" = 1 ∧
            filingCount claimed.case.closings "plaintiff" = 1 ∧
              filingCount claimed.case.closings "defendant" = 1 ∧
                filingCount claimed.case.rebuttals "plaintiff" ≤ 1 ∧
                  filingCount claimed.case.rebuttals "defendant" = 0 ∧
                    filingCount claimed.case.surrebuttals "plaintiff" = 0 ∧
                      filingCount claimed.case.surrebuttals "defendant" ≤ 1
  decision_summary_replayed :
    ∃ replayed,
      replayInitialized req actions = .ok replayed ∧
        decisionSummary replayed = decisionSummary claimed

theorem checkReplayCertificate_status_closed_facts
    (req : InitializeCaseRequest)
    (actions : List CourtAction)
    (claimed : ArbitrationState)
    (hCheck : checkReplayCertificate req actions claimed = .ok ())
    (hStatus : claimed.case.status = "closed") :
    ClosedCertificateFacts req actions claimed := by
  have hReplay : replayInitialized req actions = .ok claimed :=
    (checkReplayCertificate_ok_iff req actions claimed).1 hCheck
  have hReachable : Reachable claimed :=
    checkReplayCertificate_ok_reachable req actions claimed hCheck
  have hPhase : claimed.case.phase = "closed" :=
    reachable_status_closed_implies_phase_closed claimed hReachable hStatus
  have hResolution :
      claimed.case.resolution = "demonstrated" ∨
        claimed.case.resolution = "not_demonstrated" ∨
          claimed.case.resolution = "no_majority" :=
    reachable_closed_resolution_enum claimed hReachable hPhase
  have hSound :
      (claimed.case.resolution = "demonstrated" ∧ demonstratedOutcomeSound claimed) ∨
        (claimed.case.resolution = "not_demonstrated" ∧ notDemonstratedOutcomeSound claimed) ∨
          (claimed.case.resolution = "no_majority" ∧ noMajorityOutcomeSound claimed) := by
    rcases hResolution with hDemonstrated | hRest
    · exact Or.inl ⟨hDemonstrated,
        checkReplayCertificate_closed_demonstrated_sound
          req actions claimed hCheck hPhase hDemonstrated⟩
    · rcases hRest with hNotDemonstrated | hNoMajority
      · exact Or.inr (Or.inl ⟨hNotDemonstrated,
          checkReplayCertificate_closed_not_demonstrated_sound
            req actions claimed hCheck hPhase hNotDemonstrated⟩)
      · exact Or.inr (Or.inr ⟨hNoMajority,
          checkReplayCertificate_closed_no_majority_sound
            req actions claimed hCheck hPhase hNoMajority⟩)
  exact
    { replay_exact := hReplay
      reachable := hReachable
      length_bound :=
        checkReplayCertificate_ok_length_le_initializedBudget
          req actions claimed hCheck
      terminal_accounted := ⟨hStatus, hPhase, hResolution⟩
      outcome_sound := hSound
      merits_complete :=
        checkReplayCertificate_status_closed_merits_complete
          req actions claimed hCheck hStatus
      filing_counts :=
        checkReplayCertificate_status_closed_filing_counts
          req actions claimed hCheck hStatus
      decision_summary_replayed :=
        checkReplayCertificate_ok_decisionSummary_replayed
          req actions claimed hCheck }

theorem ClosedCertificateFacts.demonstrated_sound
    {req : InitializeCaseRequest}
    {actions : List CourtAction}
    {claimed : ArbitrationState}
    (facts : ClosedCertificateFacts req actions claimed)
    (hResolution : claimed.case.resolution = "demonstrated") :
    demonstratedOutcomeSound claimed := by
  rcases facts.outcome_sound with hDemonstrated | hRest
  · exact hDemonstrated.2
  · rcases hRest with hNotDemonstrated | hNoMajority
    · exact False.elim <| by
        simpa [hResolution] using hNotDemonstrated.1
    · exact False.elim <| by
        simpa [hResolution] using hNoMajority.1

theorem ClosedCertificateFacts.not_demonstrated_sound
    {req : InitializeCaseRequest}
    {actions : List CourtAction}
    {claimed : ArbitrationState}
    (facts : ClosedCertificateFacts req actions claimed)
    (hResolution : claimed.case.resolution = "not_demonstrated") :
    notDemonstratedOutcomeSound claimed := by
  rcases facts.outcome_sound with hDemonstrated | hRest
  · exact False.elim <| by
      simpa [hResolution] using hDemonstrated.1
  · rcases hRest with hNotDemonstrated | hNoMajority
    · exact hNotDemonstrated.2
    · exact False.elim <| by
        simpa [hResolution] using hNoMajority.1

theorem ClosedCertificateFacts.no_majority_sound
    {req : InitializeCaseRequest}
    {actions : List CourtAction}
    {claimed : ArbitrationState}
    (facts : ClosedCertificateFacts req actions claimed)
    (hResolution : claimed.case.resolution = "no_majority") :
    noMajorityOutcomeSound claimed := by
  rcases facts.outcome_sound with hDemonstrated | hRest
  · exact False.elim <| by
      simpa [hResolution] using hDemonstrated.1
  · rcases hRest with hNotDemonstrated | hNoMajority
    · exact False.elim <| by
        simpa [hResolution] using hNotDemonstrated.1
    · exact hNoMajority.2

end ArbProofs
