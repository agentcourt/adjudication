import Proofs.CaseFrame

namespace ArbProofs

/-
This file begins the outcome-soundness proof layer.

The earlier global invariants show that reachable states keep the procedure in
order, preserve side-to-side parity, respect cumulative material limits, and
keep the initialized case frame fixed.  Those theorems still leave the most
important public question open.

When the engine closes a case with `demonstrated`, `not_demonstrated`, or
`no_majority`, does that result follow from the recorded deliberation state?

The executable closing logic is concentrated in `continueDeliberation`.  That
is the right place to start.  This file proves that the three closing branches
of `continueDeliberation` are sound with respect to the recorded votes, the
configured threshold, and the round and seating conditions.

This is not yet the final reachable-state theorem promised in
`verification-plan.md`.  It is the first layer that the global theorem will
need.  The result says that whenever deliberation closes a case, the closure is
justified by the deliberation record that the engine carries in state.
-/

def demonstratedOutcomeSound (s : ArbitrationState) : Prop :=
  voteCountFor (currentRoundVotes s.case) "demonstrated" ≥
    s.policy.required_votes_for_decision

def notDemonstratedOutcomeSound (s : ArbitrationState) : Prop :=
  voteCountFor (currentRoundVotes s.case) "not_demonstrated" ≥
    s.policy.required_votes_for_decision

def noMajorityOutcomeSound (s : ArbitrationState) : Prop :=
  voteCountFor (currentRoundVotes s.case) "demonstrated" <
      s.policy.required_votes_for_decision ∧
    voteCountFor (currentRoundVotes s.case) "not_demonstrated" <
      s.policy.required_votes_for_decision ∧
    (seatedCouncilMemberCount s.case < s.policy.required_votes_for_decision ∨
      ((currentRoundVotes s.case).length = seatedCouncilMemberCount s.case ∧
        s.case.deliberation_round ≥ s.policy.max_deliberation_rounds))

/--
`voteCountFor` never exceeds the length of the vote list it counts.
-/
theorem voteCountFor_fold_le_length
    (votes : List CouncilVote)
    (value : String)
    (acc : Nat) :
    votes.foldl (fun acc vote => if trimString vote.vote = value then acc + 1 else acc) acc ≤
      acc + votes.length := by
  induction votes generalizing acc with
  | nil =>
      simp
  | cons vote rest ih =>
      simp only [List.foldl_cons, List.length_cons]
      by_cases hMatch : trimString vote.vote = value
      · simp [hMatch]
        simpa [Nat.add_assoc, Nat.add_left_comm, Nat.add_comm] using ih (acc + 1)
      · simp [hMatch]
        exact Nat.le_trans (ih acc) (Nat.le_succ (acc + rest.length))

theorem voteCountFor_le_length
    (votes : List CouncilVote)
    (value : String) :
    voteCountFor votes value ≤ votes.length := by
  simpa [voteCountFor] using voteCountFor_fold_le_length votes value 0

/--
If `currentResolution?` returns `demonstrated`, the current round already has
enough `demonstrated` votes to satisfy the threshold.

This is the smallest direct link between the executable resolution function and
the public vote count.
-/
theorem currentResolution_demonstrated_implies_sound
    (c : ArbitrationCase)
    (requiredVotes : Nat)
    (hResolution : currentResolution? c requiredVotes = some "demonstrated") :
    voteCountFor (currentRoundVotes c) "demonstrated" ≥ requiredVotes := by
  unfold currentResolution? at hResolution
  by_cases hDem : voteCountFor (currentRoundVotes c) "demonstrated" ≥ requiredVotes
  · exact hDem
  · simp [hDem] at hResolution

/--
If `currentResolution?` returns `not_demonstrated`, the current round already
has enough `not_demonstrated` votes to satisfy the threshold.
-/
theorem currentResolution_not_demonstrated_implies_sound
    (c : ArbitrationCase)
    (requiredVotes : Nat)
    (hResolution : currentResolution? c requiredVotes = some "not_demonstrated") :
    voteCountFor (currentRoundVotes c) "not_demonstrated" ≥ requiredVotes := by
  unfold currentResolution? at hResolution
  by_cases hDem : voteCountFor (currentRoundVotes c) "demonstrated" ≥ requiredVotes
  · simp [hDem] at hResolution
  · by_cases hNot : voteCountFor (currentRoundVotes c) "not_demonstrated" ≥ requiredVotes
    · exact hNot
    · simp [hDem, hNot] at hResolution

/--
If `currentResolution?` returns `none`, neither side has yet reached the vote
threshold in the current round.
-/
theorem currentResolution_none_implies_below_threshold
    (c : ArbitrationCase)
    (requiredVotes : Nat)
    (hResolution : currentResolution? c requiredVotes = none) :
    voteCountFor (currentRoundVotes c) "demonstrated" < requiredVotes ∧
      voteCountFor (currentRoundVotes c) "not_demonstrated" < requiredVotes := by
  unfold currentResolution? at hResolution
  by_cases hDem : voteCountFor (currentRoundVotes c) "demonstrated" ≥ requiredVotes
  · simp [hDem] at hResolution
  · by_cases hNot : voteCountFor (currentRoundVotes c) "not_demonstrated" ≥ requiredVotes
    · simp [hDem, hNot] at hResolution
    · exact ⟨Nat.lt_of_not_ge hDem, Nat.lt_of_not_ge hNot⟩

/--
Successful initialization always opens the live case in the `openings` phase.

This generic fact is useful in the reachable-state wrapper below.  The
initialization branch cannot already be a closed outcome.
-/
theorem initializeCase_phase_openings
    (req : InitializeCaseRequest)
    (s : ArbitrationState)
    (hInit : initializeCase req = .ok s) :
    s.case.phase = "openings" := by
  unfold initializeCase at hInit
  cases hPolicy : validatePolicy req.state.policy with
  | error err =>
      simp [hPolicy] at hInit
      cases hInit
  | ok okv =>
      cases okv
      by_cases hProposition : trimString req.proposition = ""
      · simp [hPolicy, hProposition] at hInit
        cases hInit
      · by_cases hEvidence : trimString req.state.policy.evidence_standard = ""
        · simp [hPolicy, hProposition, hEvidence] at hInit
          cases hInit
        · by_cases hEmpty : req.council_members.isEmpty
          · simp [hPolicy, hProposition, hEvidence, hEmpty] at hInit
            cases hInit
          · by_cases hLength : req.council_members.length != req.state.policy.council_size
            · simp [hPolicy, hProposition, hEvidence, hEmpty, hLength] at hInit
              cases hInit
            · by_cases hDuplicate : hasDuplicateCouncilMemberIds req.council_members
              · simp [hPolicy, hProposition, hEvidence, hEmpty, hLength, hDuplicate] at hInit
                cases hInit
              · simp [hPolicy, hProposition, hEvidence, hEmpty, hLength, hDuplicate, stateWithCase] at hInit
                cases hInit
                simp

/--
If deliberation closes a case as `demonstrated`, the closed state records
enough `demonstrated` votes for the configured threshold.

The theorem assumes that deliberation started from a deliberation-phase case.
That is exactly the condition under which the public council actions call
`continueDeliberation`.
-/
theorem continueDeliberation_closed_demonstrated_sound
    (s t : ArbitrationState)
    (c : ArbitrationCase)
    (hDeliberation : c.phase = "deliberation")
    (hCont : continueDeliberation s c = .ok t)
    (hClosed : t.case.phase = "closed")
    (hResolution : t.case.resolution = "demonstrated") :
    demonstratedOutcomeSound t := by
  unfold continueDeliberation at hCont
  by_cases hRoundComplete : (currentRoundVotes c).length = seatedCouncilMemberCount c
  · cases hCurrent : currentResolution? c s.policy.required_votes_for_decision with
    | some resolution =>
        simp [hRoundComplete, hCurrent] at hCont
        cases hCont
        have hChosen : resolution = "demonstrated" := by
          simpa [stateWithCase] using hResolution
        subst hChosen
        simpa [demonstratedOutcomeSound, stateWithCase] using
          currentResolution_demonstrated_implies_sound c s.policy.required_votes_for_decision hCurrent
    | none =>
        by_cases hTooFew : seatedCouncilMemberCount c < s.policy.required_votes_for_decision
        · simp [hRoundComplete, hCurrent, hTooFew] at hCont
          cases hCont
          simp [stateWithCase] at hResolution
        · by_cases hLastRound : c.deliberation_round ≥ s.policy.max_deliberation_rounds
          · simp [hRoundComplete, hCurrent, hTooFew, hLastRound] at hCont
            cases hCont
            simp [stateWithCase] at hResolution
          · simp [hRoundComplete, hCurrent, hTooFew, hLastRound] at hCont
            cases hCont
            simp [stateWithCase, hDeliberation] at hClosed
  · simp [hRoundComplete] at hCont
    cases hCont
    simp [stateWithCase, hDeliberation] at hClosed

/--
If deliberation closes a case as `not_demonstrated`, the closed state records
enough `not_demonstrated` votes for the configured threshold.
-/
theorem continueDeliberation_closed_not_demonstrated_sound
    (s t : ArbitrationState)
    (c : ArbitrationCase)
    (hDeliberation : c.phase = "deliberation")
    (hCont : continueDeliberation s c = .ok t)
    (hClosed : t.case.phase = "closed")
    (hResolution : t.case.resolution = "not_demonstrated") :
    notDemonstratedOutcomeSound t := by
  unfold continueDeliberation at hCont
  by_cases hRoundComplete : (currentRoundVotes c).length = seatedCouncilMemberCount c
  · cases hCurrent : currentResolution? c s.policy.required_votes_for_decision with
    | some resolution =>
        simp [hRoundComplete, hCurrent] at hCont
        cases hCont
        have hChosen : resolution = "not_demonstrated" := by
          simpa [stateWithCase] using hResolution
        subst hChosen
        simpa [notDemonstratedOutcomeSound, stateWithCase] using
          currentResolution_not_demonstrated_implies_sound c s.policy.required_votes_for_decision hCurrent
    | none =>
        by_cases hTooFew : seatedCouncilMemberCount c < s.policy.required_votes_for_decision
        · simp [hRoundComplete, hCurrent, hTooFew] at hCont
          cases hCont
          simp [stateWithCase] at hResolution
        · by_cases hLastRound : c.deliberation_round ≥ s.policy.max_deliberation_rounds
          · simp [hRoundComplete, hCurrent, hTooFew, hLastRound] at hCont
            cases hCont
            simp [stateWithCase] at hResolution
          · simp [hRoundComplete, hCurrent, hTooFew, hLastRound] at hCont
            cases hCont
            simp [stateWithCase, hDeliberation] at hClosed
  · simp [hRoundComplete] at hCont
    cases hCont
    simp [stateWithCase, hDeliberation] at hClosed

/--
If deliberation closes a case as `no_majority`, neither side reached the vote
threshold, and closure happened for one of the two executable reasons:

1. too few seated members remained to make the threshold possible; or
2. the round was complete, the final round had been reached, and no side had
   yet reached the threshold.
-/
theorem continueDeliberation_closed_no_majority_sound
    (s t : ArbitrationState)
    (c : ArbitrationCase)
    (hDeliberation : c.phase = "deliberation")
    (hCont : continueDeliberation s c = .ok t)
    (hClosed : t.case.phase = "closed")
    (hResolution : t.case.resolution = "no_majority") :
    noMajorityOutcomeSound t := by
  unfold continueDeliberation at hCont
  by_cases hRoundComplete : (currentRoundVotes c).length = seatedCouncilMemberCount c
  · cases hCurrent : currentResolution? c s.policy.required_votes_for_decision with
    | some resolution =>
        simp [hRoundComplete, hCurrent] at hCont
        cases hCont
        subst hResolution
        unfold currentResolution? at hCurrent
        by_cases hDem : voteCountFor (currentRoundVotes c) "demonstrated" ≥ s.policy.required_votes_for_decision
        · simp [hDem] at hCurrent
        · by_cases hNot : voteCountFor (currentRoundVotes c) "not_demonstrated" ≥ s.policy.required_votes_for_decision
          · simp [hDem, hNot] at hCurrent
          · simp [hDem, hNot] at hCurrent
    | none =>
        have hBelow :=
          currentResolution_none_implies_below_threshold c s.policy.required_votes_for_decision hCurrent
        by_cases hTooFew : seatedCouncilMemberCount c < s.policy.required_votes_for_decision
        · simp [hRoundComplete, hCurrent, hTooFew] at hCont
          cases hCont
          simpa [noMajorityOutcomeSound, stateWithCase] using
            And.intro hBelow.1 (And.intro hBelow.2 (Or.inl hTooFew))
        · by_cases hLastRound : c.deliberation_round ≥ s.policy.max_deliberation_rounds
          · simp [hRoundComplete, hCurrent, hTooFew, hLastRound] at hCont
            cases hCont
            simpa [noMajorityOutcomeSound, stateWithCase] using
              And.intro hBelow.1 (And.intro hBelow.2 (Or.inr ⟨hRoundComplete, hLastRound⟩))
          · simp [hRoundComplete, hCurrent, hTooFew, hLastRound] at hCont
            cases hCont
            simp [stateWithCase, hDeliberation] at hClosed
  · simp [hRoundComplete] at hCont
    cases hCont
    simp [stateWithCase, hDeliberation] at hClosed

/--
An opening-statement step never closes the case.

Openings can keep the case in `openings` or advance it to `arguments`.  They
do not jump directly to `closed`.
-/
theorem step_record_opening_statement_phase_ne_closed
    (s t : ArbitrationState)
    (action : CourtAction)
    (hType : action.action_type = "record_opening_statement")
    (hStep : step { state := s, action := action } = .ok t) :
    t.case.phase ≠ "closed" := by
  have hPhase : s.case.phase = "openings" := by
    by_cases hOpen : s.case.phase = "openings"
    · exact hOpen
    · have hClosed : s.case.phase != "openings" := by simpa using hOpen
      simp [step, hType, hClosed] at hStep
      cases hStep
  rcases step_record_opening_statement_result s t action hType hStep with ⟨rawText, rfl⟩
  by_cases hAdvance : 1 ≤ s.case.openings.length
  · simp [stateWithCase, addFiling, advanceAfterMerits, hPhase, hAdvance]
  · simp [stateWithCase, addFiling, advanceAfterMerits, hPhase, hAdvance]

/--
An argument step never closes the case.

Arguments can keep the case in `arguments` or advance it to `rebuttals`.
-/
theorem step_submit_argument_phase_ne_closed
    (s t : ArbitrationState)
    (action : CourtAction)
    (hType : action.action_type = "submit_argument")
    (hStep : step { state := s, action := action } = .ok t) :
    t.case.phase ≠ "closed" := by
  have hSubmit :
      recordMeritsSubmission
        s
        "arguments"
        action.actor_role
        (if s.case.arguments.isEmpty then "plaintiff" else "defendant")
        "argument"
        s.policy.max_argument_chars
        true
        action.payload = .ok t := by
    simpa [step, hType] using hStep
  have hPhase : s.case.phase = "arguments" := by
    unfold recordMeritsSubmission at hSubmit
    by_cases hArg : s.case.phase = "arguments"
    · exact hArg
    · have hClosed : s.case.phase != "arguments" := by simpa using hArg
      simp [hClosed] at hSubmit
      cases hSubmit
  rcases recordMeritsSubmission_with_materials_result
      s t "arguments" action.actor_role
      (if s.case.arguments.isEmpty then "plaintiff" else "defendant")
      "argument" s.policy.max_argument_chars action.payload hSubmit with
    ⟨rawText, offered, reports, rfl⟩
  by_cases hAdvance : 1 ≤ s.case.arguments.length
  · simp [stateWithCase, appendSupplementalMaterials, addFiling, advanceAfterMerits, hPhase, hAdvance]
  · simp [stateWithCase, appendSupplementalMaterials, addFiling, advanceAfterMerits, hPhase, hAdvance]

/--
A rebuttal step never closes the case.

Rebuttals can keep the case in `rebuttals` or advance it to `surrebuttals`.
-/
theorem step_submit_rebuttal_phase_ne_closed
    (s t : ArbitrationState)
    (action : CourtAction)
    (hType : action.action_type = "submit_rebuttal")
    (hStep : step { state := s, action := action } = .ok t) :
    t.case.phase ≠ "closed" := by
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
    simpa [step, hType] using hStep
  have hPhase : s.case.phase = "rebuttals" := by
    unfold recordMeritsSubmission at hSubmit
    by_cases hRebuttal : s.case.phase = "rebuttals"
    · exact hRebuttal
    · have hClosed : s.case.phase != "rebuttals" := by simpa using hRebuttal
      simp [hClosed] at hSubmit
      cases hSubmit
  rcases recordMeritsSubmission_with_materials_result
      s t "rebuttals" action.actor_role "plaintiff"
      "rebuttal" s.policy.max_rebuttal_chars action.payload hSubmit with
    ⟨rawText, offered, reports, rfl⟩
  simp [stateWithCase, appendSupplementalMaterials, addFiling, advanceAfterMerits, hPhase]

/--
A surrebuttal step never closes the case.

Surrebuttal can keep the case in `surrebuttals` or advance it to `closings`.
-/
theorem step_submit_surrebuttal_phase_ne_closed
    (s t : ArbitrationState)
    (action : CourtAction)
    (hType : action.action_type = "submit_surrebuttal")
    (hStep : step { state := s, action := action } = .ok t) :
    t.case.phase ≠ "closed" := by
  have hSubmit :
      recordMeritsSubmission
        s
        "surrebuttals"
        action.actor_role
        "defendant"
        "surrebuttal"
        s.policy.max_surrebuttal_chars
        false
        action.payload = .ok t := by
    simpa [step, hType] using hStep
  have hPhase : s.case.phase = "surrebuttals" := by
    unfold recordMeritsSubmission at hSubmit
    by_cases hSurrebuttal : s.case.phase = "surrebuttals"
    · exact hSurrebuttal
    · have hClosed : s.case.phase != "surrebuttals" := by simpa using hSurrebuttal
      simp [hClosed] at hSubmit
      cases hSubmit
  rcases recordMeritsSubmission_without_materials_result
      s t "surrebuttals" action.actor_role "defendant"
      "surrebuttal" s.policy.max_surrebuttal_chars action.payload hSubmit with
    ⟨rawText, rfl⟩
  simp [stateWithCase, addFiling, advanceAfterMerits, hPhase]

/--
A closing-statement step never closes the case.

Closings can keep the case in `closings` or advance it to `deliberation`.
-/
theorem step_deliver_closing_statement_phase_ne_closed
    (s t : ArbitrationState)
    (action : CourtAction)
    (hType : action.action_type = "deliver_closing_statement")
    (hStep : step { state := s, action := action } = .ok t) :
    t.case.phase ≠ "closed" := by
  have hPhase : s.case.phase = "closings" := by
    by_cases hClosing : s.case.phase = "closings"
    · exact hClosing
    · have hClosed : s.case.phase != "closings" := by simpa using hClosing
      simp [step, hType, hClosed] at hStep
      cases hStep
  rcases step_deliver_closing_statement_result s t action hType hStep with ⟨rawText, rfl⟩
  by_cases hAdvance : 1 ≤ s.case.closings.length
  · simp [stateWithCase, addFiling, advanceAfterMerits, hPhase, hAdvance]
  · simp [stateWithCase, addFiling, advanceAfterMerits, hPhase, hAdvance]

/--
Passing an optional phase never closes the case.

Passing rebuttal advances to `surrebuttals`.  Passing surrebuttal advances to
`closings`.
-/
theorem step_pass_phase_opportunity_phase_ne_closed
    (s t : ArbitrationState)
    (action : CourtAction)
    (hType : action.action_type = "pass_phase_opportunity")
    (hStep : step { state := s, action := action } = .ok t) :
    t.case.phase ≠ "closed" := by
  by_cases hRebuttals : s.case.phase = "rebuttals"
  · have hPass :
        (do
          requireRole action.actor_role "plaintiff"
          if !s.case.rebuttals.isEmpty then
            throw "rebuttal already submitted"
          pure <| stateWithCase s { s.case with phase := "surrebuttals" }) = .ok t := by
      simpa [step, hType, hRebuttals] using hStep
    cases hRole : requireRole action.actor_role "plaintiff" with
    | error err =>
        rw [hRole] at hPass
        simp at hPass
        cases hPass
    | ok okv =>
        cases okv
        rw [hRole] at hPass
        cases hEmpty : s.case.rebuttals.isEmpty with
        | false =>
            simp [hEmpty] at hPass
            cases hPass
        | true =>
            simp [hEmpty] at hPass
            cases hPass
            simp [stateWithCase]
  · by_cases hSurrebuttals : s.case.phase = "surrebuttals"
    · have hPass :
          (do
            requireRole action.actor_role "defendant"
            if !s.case.surrebuttals.isEmpty then
              throw "surrebuttal already submitted"
            pure <| stateWithCase s { s.case with phase := "closings" }) = .ok t := by
        simpa [step, hType, hRebuttals, hSurrebuttals] using hStep
      cases hRole : requireRole action.actor_role "defendant" with
      | error err =>
          rw [hRole] at hPass
          simp at hPass
          cases hPass
      | ok okv =>
          cases okv
          rw [hRole] at hPass
          cases hEmpty : s.case.surrebuttals.isEmpty with
          | false =>
              simp [hEmpty] at hPass
              cases hPass
          | true =>
              simp [hEmpty] at hPass
              cases hPass
              simp [stateWithCase]
    · simp [step, hType, hRebuttals, hSurrebuttals] at hStep

/--
Any successful public step that closes as `demonstrated` is sound.

The non-deliberation actions cannot close the case.  The only closing public
steps are the council vote and council member removal actions, and those now
delegate to the direct soundness theorems above.
-/
theorem step_closed_demonstrated_sound
    (s t : ArbitrationState)
    (action : CourtAction)
    (hStep : step { state := s, action := action } = .ok t)
    (hClosed : t.case.phase = "closed")
    (hResolution : t.case.resolution = "demonstrated") :
    demonstratedOutcomeSound t := by
  by_cases hOpening : action.action_type = "record_opening_statement"
  · exact False.elim ((step_record_opening_statement_phase_ne_closed s t action hOpening hStep) hClosed)
  · by_cases hArgument : action.action_type = "submit_argument"
    · exact False.elim ((step_submit_argument_phase_ne_closed s t action hArgument hStep) hClosed)
    · by_cases hRebuttal : action.action_type = "submit_rebuttal"
      · exact False.elim ((step_submit_rebuttal_phase_ne_closed s t action hRebuttal hStep) hClosed)
      · by_cases hSurrebuttal : action.action_type = "submit_surrebuttal"
        · exact False.elim ((step_submit_surrebuttal_phase_ne_closed s t action hSurrebuttal hStep) hClosed)
        · by_cases hClosing : action.action_type = "deliver_closing_statement"
          · exact False.elim ((step_deliver_closing_statement_phase_ne_closed s t action hClosing hStep) hClosed)
          · by_cases hPass : action.action_type = "pass_phase_opportunity"
            · exact False.elim ((step_pass_phase_opportunity_phase_ne_closed s t action hPass hStep) hClosed)
            · by_cases hVote : action.action_type = "submit_council_vote"
              · rcases step_submit_council_vote_result s t action hVote hStep with
                  ⟨memberId, vote, rationale, hDeliberation, hCont⟩
                let c1 := { s.case with council_votes := s.case.council_votes.concat {
                  member_id := memberId
                  round := s.case.deliberation_round
                  vote := trimString vote
                  rationale := trimString rationale
                } }
                have hDeliberation1 : c1.phase = "deliberation" := by
                  simpa [c1] using hDeliberation
                exact continueDeliberation_closed_demonstrated_sound s t c1
                  hDeliberation1 hCont hClosed hResolution
              · by_cases hRemoval : action.action_type = "remove_council_member"
                · rcases step_remove_council_member_result s t action hRemoval hStep with
                    ⟨memberId, status, hDeliberation, hCont⟩
                  let c1 := {
                    s.case with council_members := s.case.council_members.map (fun (member : CouncilMember) =>
                      if member.member_id = memberId then
                        { member with status := trimString status }
                      else
                        member)
                  }
                  have hDeliberation1 : c1.phase = "deliberation" := by
                    simpa [c1] using hDeliberation
                  exact continueDeliberation_closed_demonstrated_sound s t c1
                    hDeliberation1 hCont hClosed hResolution
                · simp [step] at hStep

theorem step_closed_not_demonstrated_sound
    (s t : ArbitrationState)
    (action : CourtAction)
    (hStep : step { state := s, action := action } = .ok t)
    (hClosed : t.case.phase = "closed")
    (hResolution : t.case.resolution = "not_demonstrated") :
    notDemonstratedOutcomeSound t := by
  by_cases hOpening : action.action_type = "record_opening_statement"
  · exact False.elim ((step_record_opening_statement_phase_ne_closed s t action hOpening hStep) hClosed)
  · by_cases hArgument : action.action_type = "submit_argument"
    · exact False.elim ((step_submit_argument_phase_ne_closed s t action hArgument hStep) hClosed)
    · by_cases hRebuttal : action.action_type = "submit_rebuttal"
      · exact False.elim ((step_submit_rebuttal_phase_ne_closed s t action hRebuttal hStep) hClosed)
      · by_cases hSurrebuttal : action.action_type = "submit_surrebuttal"
        · exact False.elim ((step_submit_surrebuttal_phase_ne_closed s t action hSurrebuttal hStep) hClosed)
        · by_cases hClosing : action.action_type = "deliver_closing_statement"
          · exact False.elim ((step_deliver_closing_statement_phase_ne_closed s t action hClosing hStep) hClosed)
          · by_cases hPass : action.action_type = "pass_phase_opportunity"
            · exact False.elim ((step_pass_phase_opportunity_phase_ne_closed s t action hPass hStep) hClosed)
            · by_cases hVote : action.action_type = "submit_council_vote"
              · rcases step_submit_council_vote_result s t action hVote hStep with
                  ⟨memberId, vote, rationale, hDeliberation, hCont⟩
                let c1 := { s.case with council_votes := s.case.council_votes.concat {
                  member_id := memberId
                  round := s.case.deliberation_round
                  vote := trimString vote
                  rationale := trimString rationale
                } }
                have hDeliberation1 : c1.phase = "deliberation" := by
                  simpa [c1] using hDeliberation
                exact continueDeliberation_closed_not_demonstrated_sound s t c1
                  hDeliberation1 hCont hClosed hResolution
              · by_cases hRemoval : action.action_type = "remove_council_member"
                · rcases step_remove_council_member_result s t action hRemoval hStep with
                    ⟨memberId, status, hDeliberation, hCont⟩
                  let c1 := {
                    s.case with council_members := s.case.council_members.map (fun (member : CouncilMember) =>
                      if member.member_id = memberId then
                        { member with status := trimString status }
                      else
                        member)
                  }
                  have hDeliberation1 : c1.phase = "deliberation" := by
                    simpa [c1] using hDeliberation
                  exact continueDeliberation_closed_not_demonstrated_sound s t c1
                    hDeliberation1 hCont hClosed hResolution
                · simp [step] at hStep

theorem step_closed_no_majority_sound
    (s t : ArbitrationState)
    (action : CourtAction)
    (hStep : step { state := s, action := action } = .ok t)
    (hClosed : t.case.phase = "closed")
    (hResolution : t.case.resolution = "no_majority") :
    noMajorityOutcomeSound t := by
  by_cases hOpening : action.action_type = "record_opening_statement"
  · exact False.elim ((step_record_opening_statement_phase_ne_closed s t action hOpening hStep) hClosed)
  · by_cases hArgument : action.action_type = "submit_argument"
    · exact False.elim ((step_submit_argument_phase_ne_closed s t action hArgument hStep) hClosed)
    · by_cases hRebuttal : action.action_type = "submit_rebuttal"
      · exact False.elim ((step_submit_rebuttal_phase_ne_closed s t action hRebuttal hStep) hClosed)
      · by_cases hSurrebuttal : action.action_type = "submit_surrebuttal"
        · exact False.elim ((step_submit_surrebuttal_phase_ne_closed s t action hSurrebuttal hStep) hClosed)
        · by_cases hClosing : action.action_type = "deliver_closing_statement"
          · exact False.elim ((step_deliver_closing_statement_phase_ne_closed s t action hClosing hStep) hClosed)
          · by_cases hPass : action.action_type = "pass_phase_opportunity"
            · exact False.elim ((step_pass_phase_opportunity_phase_ne_closed s t action hPass hStep) hClosed)
            · by_cases hVote : action.action_type = "submit_council_vote"
              · rcases step_submit_council_vote_result s t action hVote hStep with
                  ⟨memberId, vote, rationale, hDeliberation, hCont⟩
                let c1 := { s.case with council_votes := s.case.council_votes.concat {
                  member_id := memberId
                  round := s.case.deliberation_round
                  vote := trimString vote
                  rationale := trimString rationale
                } }
                have hDeliberation1 : c1.phase = "deliberation" := by
                  simpa [c1] using hDeliberation
                exact continueDeliberation_closed_no_majority_sound s t c1
                  hDeliberation1 hCont hClosed hResolution
              · by_cases hRemoval : action.action_type = "remove_council_member"
                · rcases step_remove_council_member_result s t action hRemoval hStep with
                    ⟨memberId, status, hDeliberation, hCont⟩
                  let c1 := {
                    s.case with council_members := s.case.council_members.map (fun (member : CouncilMember) =>
                      if member.member_id = memberId then
                        { member with status := trimString status }
                      else
                        member)
                  }
                  have hDeliberation1 : c1.phase = "deliberation" := by
                    simpa [c1] using hDeliberation
                  exact continueDeliberation_closed_no_majority_sound s t c1
                    hDeliberation1 hCont hClosed hResolution
                · simp [step] at hStep

/--
Every reachable closed `demonstrated` result is sound.

The base case is impossible because initialization always opens the case.  The
step case reduces immediately to the public step theorem above.
-/
theorem reachable_closed_demonstrated_sound
    (s : ArbitrationState)
    (hs : Reachable s)
    (hClosed : s.case.phase = "closed")
    (hResolution : s.case.resolution = "demonstrated") :
    demonstratedOutcomeSound s := by
  induction hs with
  | init req s hInit =>
      have hOpenings := initializeCase_phase_openings req s hInit
      simp [hOpenings] at hClosed
  | step s t action hs hStep _ =>
      exact step_closed_demonstrated_sound s t action hStep hClosed hResolution

theorem reachable_closed_not_demonstrated_sound
    (s : ArbitrationState)
    (hs : Reachable s)
    (hClosed : s.case.phase = "closed")
    (hResolution : s.case.resolution = "not_demonstrated") :
    notDemonstratedOutcomeSound s := by
  induction hs with
  | init req s hInit =>
      have hOpenings := initializeCase_phase_openings req s hInit
      simp [hOpenings] at hClosed
  | step s t action hs hStep _ =>
      exact step_closed_not_demonstrated_sound s t action hStep hClosed hResolution

theorem reachable_closed_no_majority_sound
    (s : ArbitrationState)
    (hs : Reachable s)
    (hClosed : s.case.phase = "closed")
    (hResolution : s.case.resolution = "no_majority") :
    noMajorityOutcomeSound s := by
  induction hs with
  | init req s hInit =>
      have hOpenings := initializeCase_phase_openings req s hInit
      simp [hOpenings] at hClosed
  | step s t action hs hStep _ =>
      exact step_closed_no_majority_sound s t action hStep hClosed hResolution

/--
A successful council-vote step that closes as `demonstrated` has a sound
outcome.

This theorem lifts the previous result from the raw deliberation helper to the
public `step` boundary for council votes.
-/
theorem step_submit_council_vote_closed_demonstrated_sound
    (s t : ArbitrationState)
    (action : CourtAction)
    (hType : action.action_type = "submit_council_vote")
    (hStep : step { state := s, action := action } = .ok t)
    (hClosed : t.case.phase = "closed")
    (hResolution : t.case.resolution = "demonstrated") :
    demonstratedOutcomeSound t := by
  rcases step_submit_council_vote_result s t action hType hStep with
    ⟨memberId, vote, rationale, hDeliberation, hCont⟩
  let c1 := { s.case with council_votes := s.case.council_votes.concat {
    member_id := memberId
    round := s.case.deliberation_round
    vote := trimString vote
    rationale := trimString rationale
  } }
  have hDeliberation1 : c1.phase = "deliberation" := by
    simpa [c1] using hDeliberation
  exact continueDeliberation_closed_demonstrated_sound s t c1
    hDeliberation1 hCont hClosed hResolution

theorem step_submit_council_vote_closed_not_demonstrated_sound
    (s t : ArbitrationState)
    (action : CourtAction)
    (hType : action.action_type = "submit_council_vote")
    (hStep : step { state := s, action := action } = .ok t)
    (hClosed : t.case.phase = "closed")
    (hResolution : t.case.resolution = "not_demonstrated") :
    notDemonstratedOutcomeSound t := by
  rcases step_submit_council_vote_result s t action hType hStep with
    ⟨memberId, vote, rationale, hDeliberation, hCont⟩
  let c1 := { s.case with council_votes := s.case.council_votes.concat {
    member_id := memberId
    round := s.case.deliberation_round
    vote := trimString vote
    rationale := trimString rationale
  } }
  have hDeliberation1 : c1.phase = "deliberation" := by
    simpa [c1] using hDeliberation
  exact continueDeliberation_closed_not_demonstrated_sound s t c1
    hDeliberation1 hCont hClosed hResolution

theorem step_submit_council_vote_closed_no_majority_sound
    (s t : ArbitrationState)
    (action : CourtAction)
    (hType : action.action_type = "submit_council_vote")
    (hStep : step { state := s, action := action } = .ok t)
    (hClosed : t.case.phase = "closed")
    (hResolution : t.case.resolution = "no_majority") :
    noMajorityOutcomeSound t := by
  rcases step_submit_council_vote_result s t action hType hStep with
    ⟨memberId, vote, rationale, hDeliberation, hCont⟩
  let c1 := { s.case with council_votes := s.case.council_votes.concat {
    member_id := memberId
    round := s.case.deliberation_round
    vote := trimString vote
    rationale := trimString rationale
  } }
  have hDeliberation1 : c1.phase = "deliberation" := by
    simpa [c1] using hDeliberation
  exact continueDeliberation_closed_no_majority_sound s t c1
    hDeliberation1 hCont hClosed hResolution

/--
A successful council-removal step that closes the case has a sound outcome.

The removal step can close as `no_majority` immediately when the threshold
becomes impossible.  The theorem is stated in the same shape as the vote-step
wrappers so later global theorems can work uniformly over the public step
boundary.
-/
theorem step_remove_council_member_closed_demonstrated_sound
    (s t : ArbitrationState)
    (action : CourtAction)
    (hType : action.action_type = "remove_council_member")
    (hStep : step { state := s, action := action } = .ok t)
    (hClosed : t.case.phase = "closed")
    (hResolution : t.case.resolution = "demonstrated") :
    demonstratedOutcomeSound t := by
  rcases step_remove_council_member_result s t action hType hStep with
    ⟨memberId, status, hDeliberation, hCont⟩
  let c1 := {
    s.case with council_members := s.case.council_members.map (fun (member : CouncilMember) =>
      if member.member_id = memberId then
        { member with status := trimString status }
      else
        member)
  }
  have hDeliberation1 : c1.phase = "deliberation" := by
    simpa [c1] using hDeliberation
  exact continueDeliberation_closed_demonstrated_sound s t c1
    hDeliberation1 hCont hClosed hResolution

theorem step_remove_council_member_closed_not_demonstrated_sound
    (s t : ArbitrationState)
    (action : CourtAction)
    (hType : action.action_type = "remove_council_member")
    (hStep : step { state := s, action := action } = .ok t)
    (hClosed : t.case.phase = "closed")
    (hResolution : t.case.resolution = "not_demonstrated") :
    notDemonstratedOutcomeSound t := by
  rcases step_remove_council_member_result s t action hType hStep with
    ⟨memberId, status, hDeliberation, hCont⟩
  let c1 := {
    s.case with council_members := s.case.council_members.map (fun (member : CouncilMember) =>
      if member.member_id = memberId then
        { member with status := trimString status }
      else
        member)
  }
  have hDeliberation1 : c1.phase = "deliberation" := by
    simpa [c1] using hDeliberation
  exact continueDeliberation_closed_not_demonstrated_sound s t c1
    hDeliberation1 hCont hClosed hResolution

theorem step_remove_council_member_closed_no_majority_sound
    (s t : ArbitrationState)
    (action : CourtAction)
    (hType : action.action_type = "remove_council_member")
    (hStep : step { state := s, action := action } = .ok t)
    (hClosed : t.case.phase = "closed")
    (hResolution : t.case.resolution = "no_majority") :
    noMajorityOutcomeSound t := by
  rcases step_remove_council_member_result s t action hType hStep with
    ⟨memberId, status, hDeliberation, hCont⟩
  let c1 := {
    s.case with council_members := s.case.council_members.map (fun (member : CouncilMember) =>
      if member.member_id = memberId then
        { member with status := trimString status }
      else
        member)
  }
  have hDeliberation1 : c1.phase = "deliberation" := by
    simpa [c1] using hDeliberation
  exact continueDeliberation_closed_no_majority_sound s t c1
    hDeliberation1 hCont hClosed hResolution

end ArbProofs
