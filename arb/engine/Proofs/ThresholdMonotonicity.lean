import Proofs.DeliberationSummaryCore

namespace ArbProofs

theorem currentResolution_demonstrated_of_requiredVotes_le
    (c : ArbitrationCase)
    {lower higher : Nat}
    (hLe : lower ≤ higher)
    (hResolution : currentResolution? c higher = some "demonstrated") :
    currentResolution? c lower = some "demonstrated" := by
  have hHigh :
      voteCountFor (currentRoundVotes c) "demonstrated" ≥ higher :=
    currentResolution_demonstrated_implies_sound c higher hResolution
  have hLower :
      voteCountFor (currentRoundVotes c) "demonstrated" ≥ lower :=
    Nat.le_trans hLe hHigh
  unfold currentResolution?
  simp [hLower]

theorem currentResolution_not_demonstrated_of_requiredVotes_le
    (c : ArbitrationCase)
    {lower higher : Nat}
    (hLe : lower ≤ higher)
    (hDemBelow :
      voteCountFor (currentRoundVotes c) "demonstrated" < lower)
    (hResolution : currentResolution? c higher = some "not_demonstrated") :
    currentResolution? c lower = some "not_demonstrated" := by
  have hHigh :
      voteCountFor (currentRoundVotes c) "not_demonstrated" ≥ higher :=
    currentResolution_not_demonstrated_implies_sound c higher hResolution
  have hLowerNot :
      voteCountFor (currentRoundVotes c) "not_demonstrated" ≥ lower :=
    Nat.le_trans hLe hHigh
  have hLowerDem :
      ¬ voteCountFor (currentRoundVotes c) "demonstrated" ≥ lower :=
    Nat.not_le.mpr hDemBelow
  unfold currentResolution?
  simp [hLowerDem, hLowerNot]

theorem currentResolution_none_of_requiredVotes_le
    (c : ArbitrationCase)
    {lower higher : Nat}
    (hLe : lower ≤ higher)
    (hResolution : currentResolution? c lower = none) :
    currentResolution? c higher = none := by
  have hBelow := currentResolution_none_implies_below_threshold c lower hResolution
  have hDemBelow :
      voteCountFor (currentRoundVotes c) "demonstrated" < higher :=
    Nat.lt_of_lt_of_le hBelow.1 hLe
  have hNotBelow :
      voteCountFor (currentRoundVotes c) "not_demonstrated" < higher :=
    Nat.lt_of_lt_of_le hBelow.2 hLe
  have hDemFalse :
      ¬ voteCountFor (currentRoundVotes c) "demonstrated" ≥ higher :=
    Nat.not_le.mpr hDemBelow
  have hNotFalse :
      ¬ voteCountFor (currentRoundVotes c) "not_demonstrated" ≥ higher :=
    Nat.not_le.mpr hNotBelow
  unfold currentResolution?
  simp [hDemFalse, hNotFalse]

end ArbProofs
