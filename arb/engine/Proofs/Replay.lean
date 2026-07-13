import Proofs.BoundedTermination

namespace ArbProofs

def replaySteps : ArbitrationState → List CourtAction → Except String ArbitrationState
  | s, [] => .ok s
  | s, action :: rest => do
      let t ← step { state := s, action := action }
      replaySteps t rest

theorem initializeCase_deterministic
    (req : InitializeCaseRequest)
    (s t : ArbitrationState)
    (hs : initializeCase req = .ok s)
    (ht : initializeCase req = .ok t) :
    s = t := by
  rw [hs] at ht
  cases ht
  rfl

theorem step_deterministic
    (s t u : ArbitrationState)
    (action : CourtAction)
    (ht : step { state := s, action := action } = .ok t)
    (hu : step { state := s, action := action } = .ok u) :
    t = u := by
  rw [ht] at hu
  cases hu
  rfl

theorem replaySteps_concat_ok
    (start middle target : ArbitrationState)
    (actions : List CourtAction)
    (action : CourtAction)
    (hReplay : replaySteps start actions = .ok middle)
    (hStep : step { state := middle, action := action } = .ok target) :
    replaySteps start (actions.concat action) = .ok target := by
  induction actions generalizing start with
  | nil =>
      simp [replaySteps] at hReplay
      cases hReplay
      simp [replaySteps, hStep]
      rfl
  | cons first rest ih =>
      simp [replaySteps] at hReplay ⊢
      cases hFirst : step { state := start, action := first } with
      | error err =>
          rw [hFirst] at hReplay
          cases hReplay
      | ok next =>
          rw [hFirst] at hReplay
          simpa using ih next hReplay

theorem replaySteps_success_stepReachableFrom_of_base
    (base current target : ArbitrationState)
    (actions : List CourtAction)
    (hBase : StepReachableFrom base current)
    (hReplay : replaySteps current actions = .ok target) :
    StepReachableFrom base target := by
  induction actions generalizing current with
  | nil =>
      simp [replaySteps] at hReplay
      cases hReplay
      exact hBase
  | cons action rest ih =>
      simp [replaySteps] at hReplay
      cases hStep : step { state := current, action := action } with
      | error err =>
          rw [hStep] at hReplay
          cases hReplay
      | ok next =>
          rw [hStep] at hReplay
          exact ih next (StepReachableFrom.step current next action hBase hStep) hReplay

theorem replaySteps_success_stepReachableFrom
    (start target : ArbitrationState)
    (actions : List CourtAction)
    (hReplay : replaySteps start actions = .ok target) :
    StepReachableFrom start target := by
  exact replaySteps_success_stepReachableFrom_of_base
    start start target actions StepReachableFrom.refl hReplay

theorem stepReachableFrom_replaySteps_exists
    (start target : ArbitrationState)
    (hRun : StepReachableFrom start target) :
    ∃ actions, replaySteps start actions = .ok target := by
  induction hRun with
  | refl =>
      exact ⟨[], rfl⟩
  | step s t action hs hStep ih =>
      rcases ih with ⟨actions, hReplay⟩
      exact ⟨actions.concat action, replaySteps_concat_ok start s t actions action hReplay hStep⟩

theorem stepReachableFrom_reachable
    (start target : ArbitrationState)
    (hStart : Reachable start)
    (hRun : StepReachableFrom start target) :
    Reachable target := by
  induction hRun with
  | refl =>
      exact hStart
  | step s t action hs hStep ih =>
      exact Reachable.step s t action ih hStep

theorem replaySteps_success_reachable
    (start target : ArbitrationState)
    (actions : List CourtAction)
    (hStart : Reachable start)
    (hReplay : replaySteps start actions = .ok target) :
    Reachable target := by
  exact stepReachableFrom_reachable start target hStart
    (replaySteps_success_stepReachableFrom start target actions hReplay)

theorem replaySteps_success_stepPath_of_base
    (base current target : ArbitrationState)
    (n : Nat)
    (actions : List CourtAction)
    (hBase : StepPath base n current)
    (hReplay : replaySteps current actions = .ok target) :
    StepPath base (n + actions.length) target := by
  induction actions generalizing current n with
  | nil =>
      simp [replaySteps] at hReplay
      cases hReplay
      simpa using hBase
  | cons action rest ih =>
      simp [replaySteps] at hReplay
      cases hStep : step { state := current, action := action } with
      | error err =>
          rw [hStep] at hReplay
          cases hReplay
      | ok next =>
          rw [hStep] at hReplay
          have hBaseNext : StepPath base (n + 1) next :=
            StepPath.step n current next action hBase hStep
          have hRest := ih next (n + 1) hBaseNext hReplay
          simpa [Nat.add_assoc, Nat.add_comm, Nat.add_left_comm] using hRest

theorem replaySteps_success_stepPath
    (start target : ArbitrationState)
    (actions : List CourtAction)
    (hReplay : replaySteps start actions = .ok target) :
    StepPath start actions.length target := by
  simpa using replaySteps_success_stepPath_of_base
    start start target 0 actions StepPath.refl hReplay

theorem stepPath_replaySteps_exists
    (start target : ArbitrationState)
    (n : Nat)
    (hPath : StepPath start n target) :
    ∃ actions, actions.length = n ∧ replaySteps start actions = .ok target := by
  induction hPath with
  | refl =>
      exact ⟨[], rfl, rfl⟩
  | step n s t action hs hStep ih =>
      rcases ih with ⟨actions, hLength, hReplay⟩
      exact ⟨actions.concat action, by simp [hLength],
        replaySteps_concat_ok start s t actions action hReplay hStep⟩

theorem replaySteps_length_le_initializedBudget
    (req : InitializeCaseRequest)
    (start target : ArbitrationState)
    (actions : List CourtAction)
    (hInit : initializeCase req = .ok start)
    (hReplay : replaySteps start actions = .ok target) :
    actions.length ≤ 2 * start.policy.max_submitted_evidence_per_side +
      8 + start.policy.max_deliberation_rounds * start.policy.council_size := by
  exact stepPath_length_le_initializedBudget req start target actions.length hInit
    (replaySteps_success_stepPath start target actions hReplay)

end ArbProofs
