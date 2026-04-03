import Proofs.BoundedTermination

namespace ArbProofs

/-
This file proves Stage 7 of the verification plan: deliberation neutrality.

The aggregation rule in `currentResolution?` checks `demonstrated` before
`not_demonstrated`.  That ordering is neutral only when both substantive
outcomes cannot reach the configured threshold at the same time.  The policy
validator now enforces exactly that condition through the strict-majority
requirement.

The proof still needs one additional invariant from reachable states.  The
current round cannot contain more distinct votes than there are seated council
members, and seated membership cannot exceed `policy.council_size`.  Those are
the facts that let the strict-majority arithmetic rule out the dual-threshold
case.
-/

def flipOutcomeLabel (value : String) : String :=
  let cleaned := trimString value
  if cleaned = "demonstrated" then
    "not_demonstrated"
  else if cleaned = "not_demonstrated" then
    "demonstrated"
  else
    value

def flipCouncilVote (vote : CouncilVote) : CouncilVote :=
  { vote with vote := flipOutcomeLabel vote.vote }

def flipCaseVotes (c : ArbitrationCase) : ArbitrationCase :=
  { c with council_votes := c.council_votes.map flipCouncilVote }

def flipResolution : Option String → Option String
  | some value => some (flipOutcomeLabel value)
  | none => none

@[simp] private theorem trimString_demonstrated :
    trimString "demonstrated" = "demonstrated" := by
  native_decide

@[simp] private theorem trimString_not_demonstrated :
    trimString "not_demonstrated" = "not_demonstrated" := by
  native_decide

@[simp] private theorem flipOutcomeLabel_demonstrated :
    flipOutcomeLabel "demonstrated" = "not_demonstrated" := by
  simp [flipOutcomeLabel]

@[simp] private theorem flipOutcomeLabel_not_demonstrated :
    flipOutcomeLabel "not_demonstrated" = "demonstrated" := by
  simp [flipOutcomeLabel]

private theorem currentRoundVotes_flipCaseVotes
    (c : ArbitrationCase) :
    currentRoundVotes (flipCaseVotes c) = (currentRoundVotes c).map flipCouncilVote := by
  unfold currentRoundVotes flipCaseVotes
  induction c.council_votes with
  | nil =>
      simp
  | cons vote votes ih =>
      by_cases hRound : vote.round = c.deliberation_round
      · simp [flipCouncilVote, hRound, ih]
      · simp [flipCouncilVote, hRound, ih]

private theorem voteCountFor_foldl_acc
    (votes : List CouncilVote)
    (value : String)
    (acc : Nat) :
    votes.foldl
        (fun acc vote => if trimString vote.vote = value then acc + 1 else acc)
        acc =
      acc + voteCountFor votes value := by
  induction votes generalizing acc with
  | nil =>
      simp [voteCountFor]
  | cons vote votes ih =>
      by_cases hValue : trimString vote.vote = value
      · calc
          (vote :: votes).foldl
              (fun acc vote => if trimString vote.vote = value then acc + 1 else acc)
              acc
            = votes.foldl
                (fun acc vote => if trimString vote.vote = value then acc + 1 else acc)
                (acc + 1) := by
                  simp [List.foldl, hValue]
          _ = (acc + 1) + voteCountFor votes value := by
                simpa using ih (acc + 1)
          _ = acc + (1 + voteCountFor votes value) := by
                omega
          _ = acc + votes.foldl
                (fun acc vote => if trimString vote.vote = value then acc + 1 else acc)
                1 := by
                  have hOne : votes.foldl
                      (fun acc vote => if trimString vote.vote = value then acc + 1 else acc)
                      1 =
                    1 + voteCountFor votes value := by
                      simpa using ih 1
                  rw [hOne]
          _ = acc + voteCountFor (vote :: votes) value := by
                unfold voteCountFor
                simp [List.foldl, hValue]
      · calc
          (vote :: votes).foldl
              (fun acc vote => if trimString vote.vote = value then acc + 1 else acc)
              acc
            = votes.foldl
                (fun acc vote => if trimString vote.vote = value then acc + 1 else acc)
                acc := by
                  simp [List.foldl, hValue]
          _ = acc + voteCountFor votes value := by
                simpa using ih acc
          _ = acc + voteCountFor (vote :: votes) value := by
                unfold voteCountFor
                simp [List.foldl, hValue]

private theorem voteCountFor_cons
    (vote : CouncilVote)
    (votes : List CouncilVote)
    (value : String) :
    voteCountFor (vote :: votes) value =
      (if trimString vote.vote = value then 1 else 0) + voteCountFor votes value := by
  unfold voteCountFor
  simpa using voteCountFor_foldl_acc votes value
    (if trimString vote.vote = value then 1 else 0)

private theorem flipCouncilVote_demonstrated_increment
    (vote : CouncilVote) :
    (if trimString (flipCouncilVote vote).vote = "demonstrated" then 1 else 0) =
      (if trimString vote.vote = "not_demonstrated" then 1 else 0) := by
  by_cases hDem : trimString vote.vote = "demonstrated"
  · simp [flipCouncilVote, flipOutcomeLabel, hDem]
  · by_cases hNot : trimString vote.vote = "not_demonstrated"
    · simp [flipCouncilVote, flipOutcomeLabel, hNot]
    · simp [flipCouncilVote, flipOutcomeLabel, hDem, hNot]

private theorem flipCouncilVote_not_demonstrated_increment
    (vote : CouncilVote) :
    (if trimString (flipCouncilVote vote).vote = "not_demonstrated" then 1 else 0) =
      (if trimString vote.vote = "demonstrated" then 1 else 0) := by
  by_cases hDem : trimString vote.vote = "demonstrated"
  · simp [flipCouncilVote, flipOutcomeLabel, hDem]
  · by_cases hNot : trimString vote.vote = "not_demonstrated"
    · simp [flipCouncilVote, flipOutcomeLabel, hNot]
    · simp [flipCouncilVote, flipOutcomeLabel, hDem, hNot]

private theorem voteCountFor_flipped_votes_demonstrated
    (votes : List CouncilVote) :
    voteCountFor (votes.map flipCouncilVote) "demonstrated" =
      voteCountFor votes "not_demonstrated" := by
  induction votes with
  | nil =>
      simp [voteCountFor]
  | cons vote votes ih =>
      simp only [List.map, voteCountFor_cons, ih]
      rw [flipCouncilVote_demonstrated_increment]

private theorem voteCountFor_flipped_votes_not_demonstrated
    (votes : List CouncilVote) :
    voteCountFor (votes.map flipCouncilVote) "not_demonstrated" =
      voteCountFor votes "demonstrated" := by
  induction votes with
  | nil =>
      simp [voteCountFor]
  | cons vote votes ih =>
      simp only [List.map, voteCountFor_cons, ih]
      rw [flipCouncilVote_not_demonstrated_increment]

private theorem substantive_vote_counts_le_length
    (votes : List CouncilVote) :
    voteCountFor votes "demonstrated" +
      voteCountFor votes "not_demonstrated" ≤
      votes.length := by
  induction votes with
  | nil =>
      simp [voteCountFor]
  | cons vote votes ih =>
      rw [voteCountFor_cons, voteCountFor_cons]
      by_cases hDem : trimString vote.vote = "demonstrated"
      · have hNotNe : trimString vote.vote ≠ "not_demonstrated" := by
          intro hEq
          simp [hDem] at hEq
        have hTail := ih
        simp [hDem]
        omega
      · by_cases hNot : trimString vote.vote = "not_demonstrated"
        · have hTail := ih
          simp [hNot]
          omega
        · have hTail := ih
          simp [hDem, hNot]
          omega

private theorem strict_majority_excludes_dual_threshold
    (votes : List CouncilVote)
    (councilSize requiredVotes : Nat)
    (hLength : votes.length ≤ councilSize)
    (hMajority : councilSize < 2 * requiredVotes) :
    ¬ (voteCountFor votes "demonstrated" ≥ requiredVotes ∧
        voteCountFor votes "not_demonstrated" ≥ requiredVotes) := by
  intro hBoth
  have hCounts := substantive_vote_counts_le_length votes
  rcases hBoth with ⟨hDem, hNot⟩
  omega

private theorem validatePolicy_ok_implies_strict_majority
    (p : ArbitrationPolicy)
    (hPolicy : validatePolicy p = .ok PUnit.unit) :
    p.council_size < 2 * p.required_votes_for_decision := by
  unfold validatePolicy at hPolicy
  by_cases hCouncil : p.council_size = 0
  · simp [hCouncil] at hPolicy
    cases hPolicy
  · by_cases hEvidence : trimString p.evidence_standard = ""
    · simp [hCouncil, hEvidence] at hPolicy
      cases hPolicy
    · by_cases hVotes : p.required_votes_for_decision = 0
      · simp [hCouncil, hEvidence, hVotes] at hPolicy
        cases hPolicy
      · by_cases hTooLarge : p.required_votes_for_decision > p.council_size
        · simp [hCouncil, hEvidence, hVotes, hTooLarge] at hPolicy
          cases hPolicy
        · by_cases hNotMajority : 2 * p.required_votes_for_decision ≤ p.council_size
          · simp [hCouncil, hEvidence, hVotes, hTooLarge, hNotMajority] at hPolicy
            cases hPolicy
          · omega

private theorem initializeCase_establishes_strict_majority
    (req : InitializeCaseRequest)
    (s : ArbitrationState)
    (hInit : initializeCase req = .ok s) :
    s.policy.council_size < 2 * s.policy.required_votes_for_decision := by
  have hFrame := initializeCase_establishes_caseFrame req s hInit
  rcases hFrame with ⟨_hProp, hPolicyEq, _hMembers⟩
  have hValid : validatePolicy req.state.policy = .ok PUnit.unit := by
    unfold initializeCase at hInit
    cases hPolicy : validatePolicy req.state.policy with
    | error err =>
        simp [hPolicy] at hInit
        cases hInit
    | ok okv =>
        cases okv
        rfl
  simpa [hPolicyEq] using validatePolicy_ok_implies_strict_majority req.state.policy hValid

private theorem step_preserves_strict_majority
    (s t : ArbitrationState)
    (action : CourtAction)
    (hMajority : s.policy.council_size < 2 * s.policy.required_votes_for_decision)
    (hStep : step { state := s, action := action } = .ok t) :
    t.policy.council_size < 2 * t.policy.required_votes_for_decision := by
  have hFrame : caseFrameMatches
      s.case.proposition
      s.policy
      (councilMemberIds s.case.council_members)
      s := by
    simp [caseFrameMatches]
  have hFrame' := step_preserves_caseFrame
    s t action
    s.case.proposition
    s.policy
    (councilMemberIds s.case.council_members)
    hFrame
    hStep
  rcases hFrame' with ⟨_hProp, hPolicyEq, _hMembers⟩
  simpa [hPolicyEq] using hMajority

private theorem reachable_strict_majority
    (s : ArbitrationState)
    (hs : Reachable s) :
    s.policy.council_size < 2 * s.policy.required_votes_for_decision := by
  induction hs with
  | init req s hInit =>
      exact initializeCase_establishes_strict_majority req s hInit
  | step s t action hs hStep ih =>
      exact step_preserves_strict_majority s t action ih hStep

private theorem currentRoundVotes_length_le_seatedCouncilMemberCount
    (c : ArbitrationCase)
    (hUnique : councilIdsUnique c)
    (hIntegrity : councilVoteIntegrity c) :
    (currentRoundVotes c).length ≤ seatedCouncilMemberCount c := by
  have hLen :
      (currentRoundVoteIds c).length ≤ (seatedCouncilMemberIds c).length :=
    list_length_le_of_nodup_subset
      hIntegrity.1
      (seatedCouncilMemberIds_nodup c hUnique)
      (currentRoundVoteIds_subset_seatedCouncilMemberIds c hIntegrity)
  simpa [currentRoundVoteIds, seatedCouncilMemberIds, seatedCouncilMemberCount,
    councilMemberIds] using hLen

private theorem currentResolution_flip_of_bound
    (c : ArbitrationCase)
    (councilSize requiredVotes : Nat)
    (hLength : (currentRoundVotes c).length ≤ councilSize)
    (hMajority : councilSize < 2 * requiredVotes) :
    currentResolution? (flipCaseVotes c) requiredVotes =
      flipResolution (currentResolution? c requiredVotes) := by
  have hFlipDemCount :
      voteCountFor (currentRoundVotes (flipCaseVotes c)) "demonstrated" =
        voteCountFor (currentRoundVotes c) "not_demonstrated" := by
    simpa [currentRoundVotes_flipCaseVotes] using
      voteCountFor_flipped_votes_demonstrated (currentRoundVotes c)
  have hFlipNotCount :
      voteCountFor (currentRoundVotes (flipCaseVotes c)) "not_demonstrated" =
        voteCountFor (currentRoundVotes c) "demonstrated" := by
    simpa [currentRoundVotes_flipCaseVotes] using
      voteCountFor_flipped_votes_not_demonstrated (currentRoundVotes c)
  by_cases hDem : voteCountFor (currentRoundVotes c) "demonstrated" ≥ requiredVotes
  · have hNotLt : voteCountFor (currentRoundVotes c) "not_demonstrated" < requiredVotes := by
      apply Nat.lt_of_not_ge
      intro hNot
      exact strict_majority_excludes_dual_threshold
        (currentRoundVotes c) councilSize requiredVotes hLength hMajority ⟨hDem, hNot⟩
    have hFlipDemLt : voteCountFor (currentRoundVotes (flipCaseVotes c)) "demonstrated" < requiredVotes := by
      simpa [hFlipDemCount] using hNotLt
    have hFlipNot : voteCountFor (currentRoundVotes (flipCaseVotes c)) "not_demonstrated" ≥ requiredVotes := by
      simpa [hFlipNotCount] using hDem
    simp [currentResolution?, flipResolution, hDem, hFlipDemLt, hFlipNot]
  · by_cases hNot : voteCountFor (currentRoundVotes c) "not_demonstrated" ≥ requiredVotes
    · have hFlipDem : voteCountFor (currentRoundVotes (flipCaseVotes c)) "demonstrated" ≥ requiredVotes := by
        simpa [hFlipDemCount] using hNot
      simp [currentResolution?, flipResolution, hDem, hNot, hFlipDem]
    · have hFlipDemLt : voteCountFor (currentRoundVotes (flipCaseVotes c)) "demonstrated" < requiredVotes := by
        exact Nat.lt_of_not_ge (by simpa [hFlipDemCount] using hNot)
      have hFlipNotLt : voteCountFor (currentRoundVotes (flipCaseVotes c)) "not_demonstrated" < requiredVotes := by
        exact Nat.lt_of_not_ge (by simpa [hFlipNotCount] using hDem)
      have hFlipDemFalse :
          ¬ voteCountFor (currentRoundVotes (flipCaseVotes c)) "demonstrated" ≥ requiredVotes :=
        Nat.not_le.mpr hFlipDemLt
      have hFlipNotFalse :
          ¬ voteCountFor (currentRoundVotes (flipCaseVotes c)) "not_demonstrated" ≥ requiredVotes :=
        Nat.not_le.mpr hFlipNotLt
      simp [currentResolution?, flipResolution, hDem, hNot, hFlipDemFalse, hFlipNotFalse]

/--
Flipping every current-round vote in a reachable state swaps the result of
`currentResolution?` in the same way.

This is the public neutrality theorem for the aggregation rule over the full
validated policy space.  It depends on two facts that the earlier proof layers
already established: reachable states preserve a strict-majority threshold, and
reachable deliberation records never contain more distinct current-round votes
than there are council seats available to cast them.
-/
theorem reachable_currentResolution_is_neutral_under_vote_flip
    (s : ArbitrationState)
    (hs : Reachable s) :
    currentResolution? (flipCaseVotes s.case) s.policy.required_votes_for_decision =
      flipResolution (currentResolution? s.case s.policy.required_votes_for_decision) := by
  have hUnique : councilIdsUnique s.case := reachable_councilIdsUnique s hs
  have hIntegrity : councilVoteIntegrity s.case := reachable_councilVoteIntegrity s hs
  have hVoteBound : (currentRoundVotes s.case).length ≤ seatedCouncilMemberCount s.case :=
    currentRoundVotes_length_le_seatedCouncilMemberCount s.case hUnique hIntegrity
  have hSeatedBound : seatedCouncilMemberCount s.case ≤ s.policy.council_size :=
    reachable_seatedCouncilMemberCount_le_councilSize s hs
  have hLength : (currentRoundVotes s.case).length ≤ s.policy.council_size := by
    exact Nat.le_trans hVoteBound hSeatedBound
  have hMajority : s.policy.council_size < 2 * s.policy.required_votes_for_decision :=
    reachable_strict_majority s hs
  exact currentResolution_flip_of_bound
    s.case
    s.policy.council_size
    s.policy.required_votes_for_decision
    hLength
    hMajority

end ArbProofs
