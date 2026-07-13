import Proofs.OutcomeSoundness

namespace ArbProofs

theorem voteCountFor_perm
    {votes1 votes2 : List CouncilVote}
    (value : String)
    (hPerm : List.Perm votes1 votes2) :
    voteCountFor votes1 value = voteCountFor votes2 value := by
  let countStep := fun acc (vote : CouncilVote) =>
    if trimString vote.vote = value then acc + 1 else acc
  have hComm :
      ∀ x ∈ votes1, ∀ y ∈ votes1, ∀ z,
        countStep (countStep z x) y = countStep (countStep z y) x := by
    intro x _hx y _hy z
    by_cases hx : trimString x.vote = value
    · by_cases hy : trimString y.vote = value
      · simp [countStep, hx, hy, Nat.add_comm]
      · simp [countStep, hx, hy]
    · by_cases hy : trimString y.vote = value
      · simp [countStep, hx, hy]
      · simp [countStep, hx, hy]
  simpa [voteCountFor, countStep] using
    (List.Perm.foldl_eq' hPerm hComm 0)

theorem currentResolution_eq_of_currentRoundVotes_perm
    (c d : ArbitrationCase)
    (requiredVotes : Nat)
    (hVotes : List.Perm (currentRoundVotes c) (currentRoundVotes d)) :
    currentResolution? c requiredVotes = currentResolution? d requiredVotes := by
  have hDemonstrated :
      voteCountFor (currentRoundVotes c) "demonstrated" =
        voteCountFor (currentRoundVotes d) "demonstrated" :=
    voteCountFor_perm "demonstrated" hVotes
  have hNotDemonstrated :
      voteCountFor (currentRoundVotes c) "not_demonstrated" =
        voteCountFor (currentRoundVotes d) "not_demonstrated" :=
    voteCountFor_perm "not_demonstrated" hVotes
  simp [currentResolution?, hDemonstrated, hNotDemonstrated]

end ArbProofs
