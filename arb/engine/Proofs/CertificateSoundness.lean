import Proofs.Replay
import Proofs.OutcomeSoundness

namespace ArbProofs

theorem checkReplayCertificate_closed_demonstrated_sound
    (req : InitializeCaseRequest)
    (actions : List CourtAction)
    (claimed : ArbitrationState)
    (hCheck : checkReplayCertificate req actions claimed = .ok ())
    (hClosed : claimed.case.phase = "closed")
    (hResolution : claimed.case.resolution = "demonstrated") :
    demonstratedOutcomeSound claimed := by
  exact reachable_closed_demonstrated_sound claimed
    (checkReplayCertificate_ok_reachable req actions claimed hCheck)
    hClosed
    hResolution

theorem checkReplayCertificate_closed_not_demonstrated_sound
    (req : InitializeCaseRequest)
    (actions : List CourtAction)
    (claimed : ArbitrationState)
    (hCheck : checkReplayCertificate req actions claimed = .ok ())
    (hClosed : claimed.case.phase = "closed")
    (hResolution : claimed.case.resolution = "not_demonstrated") :
    notDemonstratedOutcomeSound claimed := by
  exact reachable_closed_not_demonstrated_sound claimed
    (checkReplayCertificate_ok_reachable req actions claimed hCheck)
    hClosed
    hResolution

theorem checkReplayCertificate_closed_no_majority_sound
    (req : InitializeCaseRequest)
    (actions : List CourtAction)
    (claimed : ArbitrationState)
    (hCheck : checkReplayCertificate req actions claimed = .ok ())
    (hClosed : claimed.case.phase = "closed")
    (hResolution : claimed.case.resolution = "no_majority") :
    noMajorityOutcomeSound claimed := by
  exact reachable_closed_no_majority_sound claimed
    (checkReplayCertificate_ok_reachable req actions claimed hCheck)
    hClosed
    hResolution

theorem checkReplayCertificate_closed_substantive_threshold
    (req : InitializeCaseRequest)
    (actions : List CourtAction)
    (claimed : ArbitrationState)
    (hCheck : checkReplayCertificate req actions claimed = .ok ())
    (hClosed : claimed.case.phase = "closed")
    (hResolution :
      claimed.case.resolution = "demonstrated" ∨
        claimed.case.resolution = "not_demonstrated") :
    (claimed.case.resolution = "demonstrated" ∧ demonstratedOutcomeSound claimed) ∨
      (claimed.case.resolution = "not_demonstrated" ∧ notDemonstratedOutcomeSound claimed) := by
  rcases hResolution with hDemonstrated | hNotDemonstrated
  · exact Or.inl ⟨hDemonstrated,
      checkReplayCertificate_closed_demonstrated_sound
        req actions claimed hCheck hClosed hDemonstrated⟩
  · exact Or.inr ⟨hNotDemonstrated,
      checkReplayCertificate_closed_not_demonstrated_sound
        req actions claimed hCheck hClosed hNotDemonstrated⟩

end ArbProofs
