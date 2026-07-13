import Proofs.Replay
import Proofs.OutcomeSoundness
import Proofs.DueProcess

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

theorem checkReplayCertificate_closed_filing_counts
    (req : InitializeCaseRequest)
    (actions : List CourtAction)
    (claimed : ArbitrationState)
    (hCheck : checkReplayCertificate req actions claimed = .ok ())
    (hClosed : claimed.case.phase = "closed") :
    filingCount claimed.case.openings "plaintiff" = 1 ∧
      filingCount claimed.case.openings "defendant" = 1 ∧
        filingCount claimed.case.arguments "plaintiff" = 1 ∧
          filingCount claimed.case.arguments "defendant" = 1 ∧
            filingCount claimed.case.closings "plaintiff" = 1 ∧
              filingCount claimed.case.closings "defendant" = 1 ∧
                filingCount claimed.case.rebuttals "plaintiff" ≤ 1 ∧
                  filingCount claimed.case.rebuttals "defendant" = 0 ∧
                    filingCount claimed.case.surrebuttals "plaintiff" = 0 ∧
                      filingCount claimed.case.surrebuttals "defendant" ≤ 1 := by
  exact reachable_closed_filing_counts claimed
    (checkReplayCertificate_ok_reachable req actions claimed hCheck)
    hClosed

end ArbProofs
