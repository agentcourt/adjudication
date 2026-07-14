import Proofs.CertificateFacts

def juryVerdictAccounted (state : CourtState) : Bool :=
  match state.case.jury_verdict with
  | none => false
  | some verdict =>
      (verdict.verdict_for == "plaintiff" || verdict.verdict_for == "defendant") &&
        decide (verdict.required_votes ≤ verdict.votes_for_verdict) &&
          (!(verdict.verdict_for == "defendant") ||
            verdict.damages.toBits == (0.0).toBits)

def judgmentFromVerdictAccounted (state : CourtState) : Bool :=
  match state.case.jury_verdict with
  | none => false
  | some verdict =>
      state.case.status == "judgment_entered" &&
        state.case.monetary_judgment.toBits == verdict.damages.toBits &&
          hasDecisionTraceAction state.case "enter_judgment"

structure VerdictCertificateFacts
    (init : ReplayInitializeRequest)
    (transitions : List ReplayTransition)
    (claimed : CourtState) : Prop where
  replay_exact :
    AcceptedReplayCertificate init transitions claimed
  reachable_from_start :
    ∃ start,
      replayInitial init = .ok start ∧
        ReplayReachableFrom start claimed
  verdict_accounted :
    juryVerdictAccounted claimed = true

structure JudgmentCertificateFacts
    (init : ReplayInitializeRequest)
    (transitions : List ReplayTransition)
    (claimed : CourtState) : Prop where
  replay_exact :
    AcceptedReplayCertificate init transitions claimed
  reachable_from_start :
    ∃ start,
      replayInitial init = .ok start ∧
        ReplayReachableFrom start claimed
  judgment_accounted :
    judgmentFromVerdictAccounted claimed = true

theorem acceptedReplayCertificate_verdict_facts
    (init : ReplayInitializeRequest)
    (transitions : List ReplayTransition)
    (claimed : CourtState)
    (hAccepted : AcceptedReplayCertificate init transitions claimed)
    (hVerdict : juryVerdictAccounted claimed = true) :
    VerdictCertificateFacts init transitions claimed := by
  exact
    { replay_exact := hAccepted
      reachable_from_start :=
        acceptedReplayCertificate_reachableFrom init transitions claimed hAccepted
      verdict_accounted := hVerdict }

theorem acceptedReplayCertificate_judgment_facts
    (init : ReplayInitializeRequest)
    (transitions : List ReplayTransition)
    (claimed : CourtState)
    (hAccepted : AcceptedReplayCertificate init transitions claimed)
    (hJudgment : judgmentFromVerdictAccounted claimed = true) :
    JudgmentCertificateFacts init transitions claimed := by
  exact
    { replay_exact := hAccepted
      reachable_from_start :=
        acceptedReplayCertificate_reachableFrom init transitions claimed hAccepted
      judgment_accounted := hJudgment }
