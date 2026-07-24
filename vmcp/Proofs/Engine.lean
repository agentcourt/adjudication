import Vmcp.Engine

namespace Vmcp

/-
Engine proofs.  The invariant `outcomeSound` ties every resolution to
its ground: a substantive resolution reached the vote threshold, a
`no_majority` resolution reached neither, and any settled resolution
means the case is closed.  It is established at initialization,
preserved by `step`, and transferred to any state an accepted
certificate claims.
-/

/-- Every resolution is backed by its ground. -/
def outcomeSound (c : CaseState) : Prop :=
  (c.resolution = .demonstrated → c.policy.required_votes ≤ voteCount c .demonstrated) ∧
  (c.resolution = .not_demonstrated → c.policy.required_votes ≤ voteCount c .not_demonstrated) ∧
  (c.resolution = .no_majority →
    voteCount c .demonstrated < c.policy.required_votes ∧
    voteCount c .not_demonstrated < c.policy.required_votes) ∧
  (c.resolution ≠ .pending → c.phase = .closed)

theorem outcomeSound_of_pending (c : CaseState)
    (h : c.resolution = .pending) : outcomeSound c := by
  simp [outcomeSound, h]

theorem initializeCase_outcomeSound
    (cfg : InitConfig) (c : CaseState)
    (h : initializeCase cfg = .ok c) : outcomeSound c := by
  unfold initializeCase at h
  repeat' split at h
  any_goals simp at h
  cases h
  simp [outcomeSound]

/-- Deliberation resolution from a pending state grounds whatever
resolution it sets. -/
theorem resolveDeliberation_outcomeSound (c : CaseState)
    (hPending : c.resolution = .pending) : outcomeSound (resolveDeliberation c) := by
  unfold resolveDeliberation
  repeat' split
  all_goals simp_all [outcomeSound, voteCount]
  all_goals omega

/-- Success facts: each accepted core action ran in its phase. -/
theorem submitStatementCore_phase
    (c c' : CaseState) (actor : Actor) (text : String)
    (hStep : submitStatementCore c actor text = .ok c') : c.phase = .openings := by
  unfold submitStatementCore at hStep
  repeat' split at hStep
  any_goals simp at hStep
  cases hStep
  simp_all

theorem submitVoteCore_phase
    (c c' : CaseState) (actor : Actor) (vote : Vote) (rationale : String)
    (hStep : submitVoteCore c actor vote rationale = .ok c') : c.phase = .deliberation := by
  unfold submitVoteCore at hStep
  repeat' split at hStep
  any_goals simp at hStep
  cases hStep
  simp_all

theorem failMemberCore_phase
    (c c' : CaseState) (actor : Actor) (memberId reason : String)
    (hStep : failMemberCore c actor memberId reason = .ok c') : c.phase = .deliberation := by
  unfold failMemberCore at hStep
  repeat' split at hStep
  any_goals simp at hStep
  cases hStep
  simp_all

/-- An open phase under `outcomeSound` means the resolution is pending. -/
theorem pending_of_open (c : CaseState)
    (h : outcomeSound c) (hPhase : c.phase ≠ .closed) : c.resolution = .pending := by
  by_cases hres : c.resolution = .pending
  · exact hres
  · exact absurd (h.2.2.2 hres) hPhase

theorem afterStatement_resolution (c : CaseState) (role : Role) (text : String) :
    (afterStatement c role text).resolution = c.resolution := by
  unfold afterStatement withStatement
  split <;> rfl

theorem submitStatementCore_outcomeSound
    (c c' : CaseState) (actor : Actor) (text : String)
    (h : outcomeSound c)
    (hStep : submitStatementCore c actor text = .ok c') : outcomeSound c' := by
  have hPhase := submitStatementCore_phase c c' actor text hStep
  have hPending := pending_of_open c h (by simp [hPhase])
  unfold submitStatementCore at hStep
  repeat' split at hStep
  any_goals simp at hStep
  cases hStep
  exact outcomeSound_of_pending _ (by rw [afterStatement_resolution]; exact hPending)

theorem submitVoteCore_outcomeSound
    (c c' : CaseState) (actor : Actor) (vote : Vote) (rationale : String)
    (h : outcomeSound c)
    (hStep : submitVoteCore c actor vote rationale = .ok c') : outcomeSound c' := by
  have hPhase := submitVoteCore_phase c c' actor vote rationale hStep
  have hPending := pending_of_open c h (by simp [hPhase])
  unfold submitVoteCore at hStep
  repeat' split at hStep
  any_goals simp at hStep
  cases hStep
  exact resolveDeliberation_outcomeSound _ hPending

theorem failMemberCore_outcomeSound
    (c c' : CaseState) (actor : Actor) (memberId reason : String)
    (h : outcomeSound c)
    (hStep : failMemberCore c actor memberId reason = .ok c') : outcomeSound c' := by
  have hPhase := failMemberCore_phase c c' actor memberId reason hStep
  have hPending := pending_of_open c h (by simp [hPhase])
  unfold failMemberCore at hStep
  repeat' split at hStep
  any_goals simp at hStep
  cases hStep
  exact resolveDeliberation_outcomeSound _ hPending

theorem step_outcomeSound
    (c c' : CaseState) (a : Action)
    (h : outcomeSound c)
    (hStep : step c a = .ok c') : outcomeSound c' := by
  unfold step at hStep
  split at hStep
  · cases hStep
  · cases hDispatch : dispatch c a with
    | error e => rw [hDispatch] at hStep; cases hStep
    | ok next =>
        rw [hDispatch] at hStep
        cases hStep
        have hNext : outcomeSound next := by
          cases a with
          | submitStatement actor text =>
              exact submitStatementCore_outcomeSound c next actor text h hDispatch
          | submitVote actor vote rationale =>
              exact submitVoteCore_outcomeSound c next actor vote rationale h hDispatch
          | failMember actor memberId reason =>
              exact failMemberCore_outcomeSound c next actor memberId reason h hDispatch
        unfold outcomeSound at hNext ⊢
        simp_all [bumpVersion, voteCount]

/-- A state is reachable when some accepted action sequence produces it. -/
def Reachable (cfg : InitConfig) (c : CaseState) : Prop :=
  ∃ actions : List Action, replay cfg actions = .ok c

theorem replaySteps_preserves
    (P : CaseState → Prop)
    (hPres : ∀ c a c', P c → step c a = .ok c' → P c')
    (actions : List Action) (start final : CaseState)
    (hStart : P start)
    (hFold : replaySteps start actions = .ok final) : P final := by
  induction actions generalizing start with
  | nil =>
      unfold replaySteps at hFold
      cases hFold
      exact hStart
  | cons a rest ih =>
      unfold replaySteps at hFold
      cases hStep : step start a with
      | error e => rw [hStep] at hFold; cases hFold
      | ok next =>
          rw [hStep] at hFold
          exact ih next (hPres start a next hStart hStep) hFold

theorem reachable_outcomeSound
    (cfg : InitConfig) (c : CaseState)
    (h : Reachable cfg c) : outcomeSound c := by
  obtain ⟨actions, hReplay⟩ := h
  unfold replay at hReplay
  cases hInit : initializeCase cfg with
  | error e => rw [hInit] at hReplay; cases hReplay
  | ok start =>
      rw [hInit] at hReplay
      exact replaySteps_preserves outcomeSound
        (fun c a c' hP hs => step_outcomeSound c c' a hP hs) actions start c
        (initializeCase_outcomeSound cfg start hInit) hReplay

theorem checkCertificate_ok_reachable
    (cfg : InitConfig) (actions : List Action) (claimed : CaseState)
    (h : checkCertificate cfg actions claimed = .ok ()) : Reachable cfg claimed := by
  unfold checkCertificate at h
  cases hReplay : replay cfg actions with
  | error e => rw [hReplay] at h; simp at h
  | ok final =>
      simp only [hReplay] at h
      by_cases hEq : final = claimed
      · exact ⟨actions, hEq ▸ hReplay⟩
      · rw [if_neg hEq] at h
        simp at h

/-- Every resolution an accepted certificate claims is backed by its
ground in the claimed state. -/
theorem checkCertificate_outcomeSound
    (cfg : InitConfig) (actions : List Action) (claimed : CaseState)
    (h : checkCertificate cfg actions claimed = .ok ()) : outcomeSound claimed :=
  reachable_outcomeSound cfg claimed (checkCertificate_ok_reachable cfg actions claimed h)

end Vmcp
