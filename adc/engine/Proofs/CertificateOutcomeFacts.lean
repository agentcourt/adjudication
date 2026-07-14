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

def replayTransitionProcessJurorTimeoutFor
    (transition : ReplayTransition)
    (jurorId : String) :
    Bool :=
  match transition with
  | ReplayTransition.step action =>
      action.action_type == "process_juror_timeout" &&
        action.actor_role == "system" &&
          match getString action.payload "juror_id" with
          | .ok actual => actual == jurorId
          | .error _ => false
  | ReplayTransition.applyDecision _ => false

def replayTransitionsContainJurorTimeout
    (transitions : List ReplayTransition)
    (jurorId : String) :
    Bool :=
  transitions.any (fun transition =>
    replayTransitionProcessJurorTimeoutFor transition jurorId)

def timedOutJurorRecorded (state : CourtState) (jurorId : String) : Bool :=
  state.case.jurors.any (fun juror =>
    juror.juror_id == jurorId && juror.status == "timed_out")

def verdictUsesEffectiveConcurrence (state : CourtState) : Bool :=
  match state.case.jury_configuration, state.case.jury_verdict with
  | some cfg, some verdict =>
      verdict.required_votes ==
        effectiveMinimumConcurring cfg (countJurorsByStatus state.case.jurors "sworn")
  | _, _ => false

def jurorFailureVerdictAccounted (state : CourtState) (jurorId : String) : Bool :=
  timedOutJurorRecorded state jurorId &&
    verdictUsesEffectiveConcurrence state &&
      juryVerdictAccounted state

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

structure JurorFailureVerdictCertificateFacts
    (init : ReplayInitializeRequest)
    (transitions : List ReplayTransition)
    (claimed : CourtState)
    (jurorId : String) : Prop where
  replay_exact :
    AcceptedReplayCertificate init transitions claimed
  reachable_from_start :
    ∃ start,
      replayInitial init = .ok start ∧
        ReplayReachableFrom start claimed
  timeout_transition_recorded :
    replayTransitionsContainJurorTimeout transitions jurorId = true
  juror_failure_verdict_accounted :
    jurorFailureVerdictAccounted claimed jurorId = true

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

theorem acceptedReplayCertificate_juror_failure_verdict_facts
    (init : ReplayInitializeRequest)
    (transitions : List ReplayTransition)
    (claimed : CourtState)
    (jurorId : String)
    (hAccepted : AcceptedReplayCertificate init transitions claimed)
    (hTimeout : replayTransitionsContainJurorTimeout transitions jurorId = true)
    (hVerdict : jurorFailureVerdictAccounted claimed jurorId = true) :
    JurorFailureVerdictCertificateFacts init transitions claimed jurorId := by
  exact
    { replay_exact := hAccepted
      reachable_from_start :=
        acceptedReplayCertificate_reachableFrom init transitions claimed hAccepted
      timeout_transition_recorded := hTimeout
      juror_failure_verdict_accounted := hVerdict }
