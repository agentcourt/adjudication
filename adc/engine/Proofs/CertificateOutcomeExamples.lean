import Proofs.CertificateOutcomeFacts

def certificateOutcomeSwornJuror (jurorId : String) : JurorRecord :=
  { juror_id := jurorId
  , name := jurorId
  , status := "sworn"
  }

def certificateOutcomeVote
    (jurorId vote : String)
    (damages : Float)
    (explanation : String) :
    JurorVote :=
  { juror_id := jurorId
  , round := 1
  , vote := vote
  , damages := damages
  , confidence := "high"
  , explanation := explanation
  , submitted_at := "2026-03-15"
  }

def certificateVerdictInitialCase : CaseState :=
  { (default : CaseState) with
    case_id := "certificate-verdict"
    filed_on := "2026-03-15"
    status := "trial"
    trial_mode := "jury"
    phase := "deliberation"
    jury_configuration :=
      some { juror_count := 6, unanimous_required := false, minimum_concurring := 4 }
    jurors :=
      [ certificateOutcomeSwornJuror "J1"
      , certificateOutcomeSwornJuror "J2"
      , certificateOutcomeSwornJuror "J3"
      , certificateOutcomeSwornJuror "J4"
      , certificateOutcomeSwornJuror "J5"
      , certificateOutcomeSwornJuror "J6"
      ]
    juror_votes :=
      [ certificateOutcomeVote "J1" "plaintiff" 100.0 "J1 explanation"
      , certificateOutcomeVote "J2" "plaintiff" 100.0 "J2 explanation"
      , certificateOutcomeVote "J3" "plaintiff" 100.0 "J3 explanation"
      , certificateOutcomeVote "J4" "defendant" 0.0 "J4 explanation"
      , certificateOutcomeVote "J5" "defendant" 0.0 "J5 explanation"
      ]
  }

def certificateVerdictInitialState : CourtState :=
  { (default : CourtState) with
    schema_version := "v1"
    court_name := "Test Court"
    case := certificateVerdictInitialCase
  }

def certificateVerdictFinalVoteAction : CourtAction :=
  { action_type := "submit_juror_vote"
  , actor_role := "juror"
  , payload := Lean.Json.mkObj
      [ ("juror_id", Lean.Json.str "J6")
      , ("vote", Lean.Json.str "plaintiff")
      , ("damages", Lean.Json.num 100)
      , ("confidence", Lean.Json.str "high")
      , ("explanation", Lean.Json.str "J6 explanation")
      ]
  }

def sampleVerdictCertificateInit : ReplayInitializeRequest :=
  { state := certificateVerdictInitialState
  , initialize_case := none
  }

def sampleVerdictCertificateTransitions : List ReplayTransition :=
  [ReplayTransition.step certificateVerdictFinalVoteAction]

def sampleVerdictCertificateState : CourtState :=
  certificateStateOrDefault
    (replayCertificate
      sampleVerdictCertificateInit
      sampleVerdictCertificateTransitions)

theorem sample_verdict_certificate_replay_bool :
    certificateReplayAccepted
      (replayCertificate
        sampleVerdictCertificateInit
        sampleVerdictCertificateTransitions) = true := by
  native_decide

theorem sample_verdict_certificate_replay :
    AcceptedReplayCertificate
      sampleVerdictCertificateInit
      sampleVerdictCertificateTransitions
      sampleVerdictCertificateState := by
  exact certificateStateOrDefault_ok
    (replayCertificate
      sampleVerdictCertificateInit
      sampleVerdictCertificateTransitions)
    sample_verdict_certificate_replay_bool

theorem sample_verdict_certificate_accounted :
    juryVerdictAccounted sampleVerdictCertificateState = true := by
  native_decide

theorem sample_verdict_certificate_facts :
    VerdictCertificateFacts
      sampleVerdictCertificateInit
      sampleVerdictCertificateTransitions
      sampleVerdictCertificateState := by
  exact acceptedReplayCertificate_verdict_facts
    sampleVerdictCertificateInit
    sampleVerdictCertificateTransitions
    sampleVerdictCertificateState
    sample_verdict_certificate_replay
    sample_verdict_certificate_accounted

def certificateJudgmentClaim : Lean.Json :=
  Lean.Json.mkObj
    [ ("claim_id", Lean.Json.str "claim-1")
    , ("label", Lean.Json.str "Misrepresentation")
    , ("legal_theory", Lean.Json.str "misrepresentation")
    , ("standard_of_proof", Lean.Json.str "preponderance_of_the_evidence")
    , ("burden_holder", Lean.Json.str "plaintiff")
    , ("elements", Lean.Json.mkObj [])
    , ("defenses", Lean.Json.mkObj [])
    , ("damages_question", Lean.Json.str "What damages, if any, did plaintiff prove?")
    ]

def certificateJudgmentInitialCase : CaseState :=
  { (default : CaseState) with
    case_id := "certificate-judgment"
    filed_on := "2026-03-15"
    status := "trial"
    trial_mode := "jury"
    phase := "post_verdict"
    single_claim := some certificateJudgmentClaim
    jury_verdict :=
      some
        { verdict_for := "plaintiff"
        , votes_for_verdict := 6
        , required_votes := 4
        , damages := 125.0
        }
  }

def certificateJudgmentInitialState : CourtState :=
  { (default : CourtState) with
    schema_version := "v1"
    court_name := "Test Court"
    case := certificateJudgmentInitialCase
  }

def certificateEnterJudgmentAction : CourtAction :=
  { action_type := "enter_judgment"
  , actor_role := "judge"
  , payload := Lean.Json.mkObj
      [ ("claim_id", Lean.Json.str "claim-1")
      , ("basis", Lean.Json.str "jury verdict")
      ]
  }

def sampleJudgmentCertificateInit : ReplayInitializeRequest :=
  { state := certificateJudgmentInitialState
  , initialize_case := none
  }

def sampleJudgmentCertificateTransitions : List ReplayTransition :=
  [ReplayTransition.step certificateEnterJudgmentAction]

def sampleJudgmentCertificateState : CourtState :=
  certificateStateOrDefault
    (replayCertificate
      sampleJudgmentCertificateInit
      sampleJudgmentCertificateTransitions)

theorem sample_judgment_certificate_replay_bool :
    certificateReplayAccepted
      (replayCertificate
        sampleJudgmentCertificateInit
        sampleJudgmentCertificateTransitions) = true := by
  native_decide

theorem sample_judgment_certificate_replay :
    AcceptedReplayCertificate
      sampleJudgmentCertificateInit
      sampleJudgmentCertificateTransitions
      sampleJudgmentCertificateState := by
  exact certificateStateOrDefault_ok
    (replayCertificate
      sampleJudgmentCertificateInit
      sampleJudgmentCertificateTransitions)
    sample_judgment_certificate_replay_bool

theorem sample_judgment_certificate_accounted :
    judgmentFromVerdictAccounted sampleJudgmentCertificateState = true := by
  native_decide

theorem sample_judgment_certificate_facts :
    JudgmentCertificateFacts
      sampleJudgmentCertificateInit
      sampleJudgmentCertificateTransitions
      sampleJudgmentCertificateState := by
  exact acceptedReplayCertificate_judgment_facts
    sampleJudgmentCertificateInit
    sampleJudgmentCertificateTransitions
    sampleJudgmentCertificateState
    sample_judgment_certificate_replay
    sample_judgment_certificate_accounted
