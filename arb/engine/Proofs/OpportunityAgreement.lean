import Proofs.MaximalRuns

namespace ArbProofs

def actorFacingAction (action : CourtAction) : Prop :=
  action.action_type ≠ "remove_council_member" ∧
    action.action_type ≠ "fail_opportunity"

theorem step_record_opening_statement_source_phase
    (s t : ArbitrationState)
    (action : CourtAction)
    (hType : action.action_type = "record_opening_statement")
    (hStep : stepCore { state := s, action := action } = .ok t) :
    s.case.phase = "openings" := by
  by_cases hPhase : s.case.phase = "openings"
  · exact hPhase
  · have hClosed : s.case.phase != "openings" := by simpa using hPhase
    simp [stepCore, hType, hClosed] at hStep
    cases hStep

theorem recordMeritsSubmission_source_phase
    (s t : ArbitrationState)
    (phase actorRole expectedRole textLabel : String)
    (limit : Nat)
    (allowSupplementalMaterials : Bool)
    (payload : Lean.Json)
    (hSubmit : recordMeritsSubmission
      s phase actorRole expectedRole textLabel limit allowSupplementalMaterials payload = .ok t) :
    s.case.phase = phase := by
  by_cases hPhase : s.case.phase = phase
  · exact hPhase
  · have hClosed : s.case.phase != phase := by simpa using hPhase
    simp [recordMeritsSubmission, hClosed] at hSubmit
    cases hSubmit

theorem step_submit_argument_source_phase
    (s t : ArbitrationState)
    (action : CourtAction)
    (hType : action.action_type = "submit_argument")
    (hStep : stepCore { state := s, action := action } = .ok t) :
    s.case.phase = "arguments" := by
  let role := if s.case.arguments.isEmpty then "plaintiff" else "defendant"
  have hSubmit :
      recordMeritsSubmission
        s
        "arguments"
        action.actor_role
        role
        "argument"
        s.policy.max_argument_chars
        true
        action.payload = .ok t := by
    simpa [stepCore, hType, role] using hStep
  exact recordMeritsSubmission_source_phase s t "arguments" action.actor_role role
    "argument" s.policy.max_argument_chars true action.payload hSubmit

theorem step_submit_rebuttal_source_phase
    (s t : ArbitrationState)
    (action : CourtAction)
    (hType : action.action_type = "submit_rebuttal")
    (hStep : stepCore { state := s, action := action } = .ok t) :
    s.case.phase = "rebuttals" := by
  have hSubmit :
      recordMeritsSubmission
        s
        "rebuttals"
        action.actor_role
        "plaintiff"
        "rebuttal"
        s.policy.max_rebuttal_chars
        true
        action.payload = .ok t := by
    simpa [stepCore, hType] using hStep
  exact recordMeritsSubmission_source_phase s t "rebuttals" action.actor_role "plaintiff"
    "rebuttal" s.policy.max_rebuttal_chars true action.payload hSubmit

theorem step_submit_surrebuttal_source_phase
    (s t : ArbitrationState)
    (action : CourtAction)
    (hType : action.action_type = "submit_surrebuttal")
    (hStep : stepCore { state := s, action := action } = .ok t) :
    s.case.phase = "surrebuttals" := by
  have hSubmit :
      recordMeritsSubmission
        s
        "surrebuttals"
        action.actor_role
        "defendant"
        "surrebuttal"
        s.policy.max_surrebuttal_chars
        true
        action.payload = .ok t := by
    simpa [stepCore, hType] using hStep
  exact recordMeritsSubmission_source_phase s t "surrebuttals" action.actor_role "defendant"
    "surrebuttal" s.policy.max_surrebuttal_chars true action.payload hSubmit

theorem step_deliver_closing_statement_source_phase
    (s t : ArbitrationState)
    (action : CourtAction)
    (hType : action.action_type = "deliver_closing_statement")
    (hStep : stepCore { state := s, action := action } = .ok t) :
    s.case.phase = "closings" := by
  by_cases hPhase : s.case.phase = "closings"
  · exact hPhase
  · have hClosed : s.case.phase != "closings" := by simpa using hPhase
    simp [stepCore, hType, hClosed] at hStep
    cases hStep

theorem submitEvidence_source_phase
    (s t : ArbitrationState)
    (actorRole : String)
    (payload : Lean.Json)
    (hSubmit : submitEvidence s actorRole payload = .ok t) :
    s.case.phase = "arguments" ∨
      s.case.phase = "rebuttals" ∨
        s.case.phase = "surrebuttals" := by
  by_cases hArguments : s.case.phase = "arguments"
  · exact Or.inl hArguments
  · by_cases hRebuttals : s.case.phase = "rebuttals"
    · exact Or.inr (Or.inl hRebuttals)
    · by_cases hSurrebuttals : s.case.phase = "surrebuttals"
      · exact Or.inr (Or.inr hSurrebuttals)
      · simp [submitEvidence] at hSubmit
        cases hSubmit

theorem nextOpportunity_allows_record_opening_statement
    (s : ArbitrationState)
    (opportunity : OpportunitySpec)
    (hStatus : s.case.status = "active")
    (hPhase : s.case.phase = "openings")
    (hNext : (nextOpportunity s).opportunity = some opportunity) :
    "record_opening_statement" ∈ opportunity.allowed_tools := by
  unfold nextOpportunity at hNext
  simp [hStatus] at hNext
  cases hRole : plaintiffThenDefendant s.case.openings "plaintiff" "defendant" with
  | none =>
      simp [nextOpportunityForPhase, hPhase, hRole] at hNext
  | some role =>
      simp [nextOpportunityForPhase, hPhase, hRole] at hNext
      cases hNext
      simp

theorem nextOpportunity_allows_submit_argument
    (s : ArbitrationState)
    (opportunity : OpportunitySpec)
    (hStatus : s.case.status = "active")
    (hPhase : s.case.phase = "arguments")
    (hNext : (nextOpportunity s).opportunity = some opportunity) :
    "submit_argument" ∈ opportunity.allowed_tools := by
  unfold nextOpportunity at hNext
  simp [hStatus] at hNext
  cases hRole : plaintiffThenDefendant s.case.arguments "plaintiff" "defendant" with
  | none =>
      simp [nextOpportunityForPhase, hPhase, hRole] at hNext
  | some role =>
      simp [nextOpportunityForPhase, hPhase, hRole] at hNext
      cases hNext
      simp

theorem nextOpportunity_allows_submit_evidence
    (s : ArbitrationState)
    (opportunity : OpportunitySpec)
    (hStatus : s.case.status = "active")
    (hPhase : s.case.phase = "arguments" ∨
      s.case.phase = "rebuttals" ∨
        s.case.phase = "surrebuttals")
    (hNext : (nextOpportunity s).opportunity = some opportunity) :
    "submit_evidence" ∈ opportunity.allowed_tools := by
  rcases hPhase with hArguments | hRest
  · unfold nextOpportunity at hNext
    simp [hStatus] at hNext
    cases hRole : plaintiffThenDefendant s.case.arguments "plaintiff" "defendant" with
    | none =>
        simp [nextOpportunityForPhase, hArguments, hRole] at hNext
    | some role =>
        simp [nextOpportunityForPhase, hArguments, hRole] at hNext
        cases hNext
        simp
  · rcases hRest with hRebuttals | hSurrebuttals
    · unfold nextOpportunity at hNext
      simp [hStatus] at hNext
      cases hEmpty : s.case.rebuttals.isEmpty with
      | false =>
          simp [nextOpportunityForPhase, hRebuttals, hEmpty] at hNext
      | true =>
          simp [nextOpportunityForPhase, hRebuttals, hEmpty] at hNext
          cases hNext
          simp
    · unfold nextOpportunity at hNext
      simp [hStatus] at hNext
      cases hEmpty : s.case.surrebuttals.isEmpty with
      | false =>
          simp [nextOpportunityForPhase, hSurrebuttals, hEmpty] at hNext
      | true =>
          simp [nextOpportunityForPhase, hSurrebuttals, hEmpty] at hNext
          cases hNext
          simp

theorem nextOpportunity_allows_submit_rebuttal
    (s : ArbitrationState)
    (opportunity : OpportunitySpec)
    (hStatus : s.case.status = "active")
    (hPhase : s.case.phase = "rebuttals")
    (hNext : (nextOpportunity s).opportunity = some opportunity) :
    "submit_rebuttal" ∈ opportunity.allowed_tools := by
  unfold nextOpportunity at hNext
  simp [hStatus] at hNext
  cases hEmpty : s.case.rebuttals.isEmpty with
  | false =>
      simp [nextOpportunityForPhase, hPhase, hEmpty] at hNext
  | true =>
      simp [nextOpportunityForPhase, hPhase, hEmpty] at hNext
      cases hNext
      simp

theorem nextOpportunity_allows_submit_surrebuttal
    (s : ArbitrationState)
    (opportunity : OpportunitySpec)
    (hStatus : s.case.status = "active")
    (hPhase : s.case.phase = "surrebuttals")
    (hNext : (nextOpportunity s).opportunity = some opportunity) :
    "submit_surrebuttal" ∈ opportunity.allowed_tools := by
  unfold nextOpportunity at hNext
  simp [hStatus] at hNext
  cases hEmpty : s.case.surrebuttals.isEmpty with
  | false =>
      simp [nextOpportunityForPhase, hPhase, hEmpty] at hNext
  | true =>
      simp [nextOpportunityForPhase, hPhase, hEmpty] at hNext
      cases hNext
      simp

theorem nextOpportunity_allows_deliver_closing_statement
    (s : ArbitrationState)
    (opportunity : OpportunitySpec)
    (hStatus : s.case.status = "active")
    (hPhase : s.case.phase = "closings")
    (hNext : (nextOpportunity s).opportunity = some opportunity) :
    "deliver_closing_statement" ∈ opportunity.allowed_tools := by
  unfold nextOpportunity at hNext
  simp [hStatus] at hNext
  cases hRole : plaintiffThenDefendant s.case.closings "plaintiff" "defendant" with
  | none =>
      simp [nextOpportunityForPhase, hPhase, hRole] at hNext
  | some role =>
      simp [nextOpportunityForPhase, hPhase, hRole] at hNext
      cases hNext
      simp

theorem nextOpportunity_allows_pass_phase_opportunity
    (s : ArbitrationState)
    (opportunity : OpportunitySpec)
    (hStatus : s.case.status = "active")
    (hPhase : s.case.phase = "rebuttals" ∨ s.case.phase = "surrebuttals")
    (hNext : (nextOpportunity s).opportunity = some opportunity) :
    "pass_phase_opportunity" ∈ opportunity.allowed_tools := by
  rcases hPhase with hRebuttals | hSurrebuttals
  · unfold nextOpportunity at hNext
    simp [hStatus] at hNext
    cases hEmpty : s.case.rebuttals.isEmpty with
    | false =>
        simp [nextOpportunityForPhase, hRebuttals, hEmpty] at hNext
    | true =>
        simp [nextOpportunityForPhase, hRebuttals, hEmpty] at hNext
        cases hNext
        simp
  · unfold nextOpportunity at hNext
    simp [hStatus] at hNext
    cases hEmpty : s.case.surrebuttals.isEmpty with
    | false =>
        simp [nextOpportunityForPhase, hSurrebuttals, hEmpty] at hNext
    | true =>
        simp [nextOpportunityForPhase, hSurrebuttals, hEmpty] at hNext
        cases hNext
        simp

theorem nextOpportunity_allows_submit_council_vote
    (s : ArbitrationState)
    (opportunity : OpportunitySpec)
    (hStatus : s.case.status = "active")
    (hPhase : s.case.phase = "deliberation")
    (hNext : (nextOpportunity s).opportunity = some opportunity) :
    "submit_council_vote" ∈ opportunity.allowed_tools := by
  unfold nextOpportunity at hNext
  simp [hStatus] at hNext
  cases hMember : nextCouncilMember? s.case with
  | none =>
      simp [nextOpportunityForPhase, hPhase, hMember] at hNext
  | some member =>
      simp [nextOpportunityForPhase, hPhase, hMember] at hNext
      cases hNext
      simp

theorem reachable_actor_step_allowed_by_nextOpportunity
    (s t : ArbitrationState)
    (action : CourtAction)
    (hs : Reachable s)
    (hStatus : s.case.status = "active")
    (hActorFacing : actorFacingAction action)
    (hStep : step { state := s, action := action } = .ok t) :
    ∃ opportunity,
      (nextOpportunity s).opportunity = some opportunity ∧
        action.action_type ∈ opportunity.allowed_tools := by
  rcases reachable_active_has_nextOpportunity s hs hStatus with ⟨_hLive, hSome⟩
  rcases hSome with ⟨opportunity, hNext⟩
  have hStepCore := stepCore_ok_of_step_ok s t action hStep
  by_cases hOpening : action.action_type = "record_opening_statement"
  · have hPhase := step_record_opening_statement_source_phase s t action hOpening hStepCore
    exact ⟨opportunity, hNext, by
      simpa [hOpening] using
        nextOpportunity_allows_record_opening_statement s opportunity hStatus hPhase hNext⟩
  · by_cases hArgument : action.action_type = "submit_argument"
    · have hPhase := step_submit_argument_source_phase s t action hArgument hStepCore
      exact ⟨opportunity, hNext, by
        simpa [hArgument] using
          nextOpportunity_allows_submit_argument s opportunity hStatus hPhase hNext⟩
    · by_cases hRebuttal : action.action_type = "submit_rebuttal"
      · have hPhase := step_submit_rebuttal_source_phase s t action hRebuttal hStepCore
        exact ⟨opportunity, hNext, by
          simpa [hRebuttal] using
            nextOpportunity_allows_submit_rebuttal s opportunity hStatus hPhase hNext⟩
      · by_cases hSurrebuttal : action.action_type = "submit_surrebuttal"
        · have hPhase := step_submit_surrebuttal_source_phase s t action hSurrebuttal hStepCore
          exact ⟨opportunity, hNext, by
            simpa [hSurrebuttal] using
              nextOpportunity_allows_submit_surrebuttal s opportunity hStatus hPhase hNext⟩
        · by_cases hEvidence : action.action_type = "submit_evidence"
          · have hSubmit : submitEvidence s action.actor_role action.payload = .ok t := by
              simpa [stepCore, hOpening, hArgument, hRebuttal, hSurrebuttal, hEvidence]
                using hStepCore
            have hPhase := submitEvidence_source_phase s t action.actor_role action.payload hSubmit
            exact ⟨opportunity, hNext, by
              simpa [hEvidence] using
                nextOpportunity_allows_submit_evidence s opportunity hStatus hPhase hNext⟩
          · by_cases hClosing : action.action_type = "deliver_closing_statement"
            · have hPhase := step_deliver_closing_statement_source_phase s t action hClosing hStepCore
              exact ⟨opportunity, hNext, by
                simpa [hClosing] using
                  nextOpportunity_allows_deliver_closing_statement s opportunity hStatus hPhase hNext⟩
            · by_cases hPass : action.action_type = "pass_phase_opportunity"
              · rcases step_pass_phase_opportunity_result s t action hPass hStepCore with hResult | hResult
                · exact ⟨opportunity, hNext, by
                    simpa [hPass] using
                      nextOpportunity_allows_pass_phase_opportunity s opportunity hStatus
                        (Or.inl hResult.1) hNext⟩
                · exact ⟨opportunity, hNext, by
                    simpa [hPass] using
                      nextOpportunity_allows_pass_phase_opportunity s opportunity hStatus
                        (Or.inr hResult.1) hNext⟩
              · by_cases hVote : action.action_type = "submit_council_vote"
                · rcases step_submit_council_vote_result s t action hVote hStepCore with
                    ⟨_memberId, _vote, _rationale, hPhase, _hCont⟩
                  exact ⟨opportunity, hNext, by
                    simpa [hVote] using
                      nextOpportunity_allows_submit_council_vote s opportunity hStatus hPhase hNext⟩
                · by_cases hRemove : action.action_type = "remove_council_member"
                  · exact False.elim (hActorFacing.1 hRemove)
                  · by_cases hFail : action.action_type = "fail_opportunity"
                    · exact False.elim (hActorFacing.2 hFail)
                    · simp [stepCore] at hStepCore

end ArbProofs
