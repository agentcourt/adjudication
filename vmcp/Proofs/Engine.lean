import Vmcp.Engine

namespace Vmcp

/-
Engine proofs.  The invariant `resSound` states that a substantive
resolution is backed by the vote threshold.  It is established at
initialization, preserved by `step`, and transferred to any state an
accepted certificate claims.
-/

/-- A substantive resolution is backed by the configured threshold. -/
def resSound (c : CaseState) : Prop :=
  (c.resolution = .demonstrated → c.policy.required_votes ≤ voteCount c .demonstrated) ∧
  (c.resolution = .not_demonstrated → c.policy.required_votes ≤ voteCount c .not_demonstrated)

theorem initializeCase_resSound
    (cfg : InitConfig) (c : CaseState)
    (h : initializeCase cfg = .ok c) : resSound c := by
  unfold initializeCase at h
  repeat' split at h
  any_goals simp at h
  cases h
  simp [resSound]

theorem resolveDeliberation_votes (c : CaseState) :
    (resolveDeliberation c).votes = c.votes := by
  unfold resolveDeliberation
  repeat' split
  all_goals rfl

theorem resolveDeliberation_policy (c : CaseState) :
    (resolveDeliberation c).policy = c.policy := by
  unfold resolveDeliberation
  repeat' split
  all_goals rfl

theorem voteCount_eq_of_votes_eq (c c' : CaseState)
    (h : c'.votes = c.votes) (v : Vote) : voteCount c' v = voteCount c v := by
  simp [voteCount, h]

theorem resolveDeliberation_resSound (c : CaseState)
    (h : resSound c) : resSound (resolveDeliberation c) := by
  have hv := resolveDeliberation_votes c
  have hp := resolveDeliberation_policy c
  unfold resSound at h ⊢
  rw [voteCount_eq_of_votes_eq _ _ hv, voteCount_eq_of_votes_eq _ _ hv, hp]
  unfold resolveDeliberation
  repeat' split
  all_goals simp_all

theorem afterStatement_resSound
    (c : CaseState) (role : Role) (text : String)
    (h : resSound c) : resSound (afterStatement c role text) := by
  unfold resSound at h ⊢
  unfold afterStatement
  split
  all_goals simp_all [withStatement, voteCount]

theorem voteCount_concat (c : CaseState) (x : CastVote) (v : Vote) :
    voteCount { c with votes := c.votes.concat x } v =
      if x.vote = v then voteCount c v + 1 else voteCount c v := by
  simp [voteCount, List.concat_eq_append, List.foldl_append]

theorem submitStatementCore_resSound
    (c c' : CaseState) (actor : Actor) (text : String)
    (h : resSound c)
    (hStep : submitStatementCore c actor text = .ok c') : resSound c' := by
  unfold submitStatementCore at hStep
  repeat' split at hStep
  any_goals simp at hStep
  cases hStep
  exact afterStatement_resSound c (expectedSide c) (trimString text) h

theorem submitVoteCore_resSound
    (c c' : CaseState) (actor : Actor) (vote : Vote) (rationale : String)
    (h : resSound c)
    (hStep : submitVoteCore c actor vote rationale = .ok c') : resSound c' := by
  unfold submitVoteCore at hStep
  repeat' split at hStep
  any_goals simp at hStep
  cases hStep
  apply resolveDeliberation_resSound
  unfold resSound at h ⊢
  constructor
  · intro hres
    have hc := h.1 hres
    simp [voteCount, List.concat_eq_append, List.foldl_append] at hc ⊢
    split <;> omega
  · intro hres
    have hc := h.2 hres
    simp [voteCount, List.concat_eq_append, List.foldl_append] at hc ⊢
    split <;> omega

theorem failMemberCore_resSound
    (c c' : CaseState) (actor : Actor) (memberId reason : String)
    (h : resSound c)
    (hStep : failMemberCore c actor memberId reason = .ok c') : resSound c' := by
  unfold failMemberCore at hStep
  repeat' split at hStep
  any_goals simp at hStep
  cases hStep
  apply resolveDeliberation_resSound
  unfold resSound at h ⊢
  simp_all [voteCount]

theorem step_resSound
    (c c' : CaseState) (a : Action)
    (h : resSound c)
    (hStep : step c a = .ok c') : resSound c' := by
  unfold step at hStep
  split at hStep
  · cases hStep
  · rename_i hOpen
    cases hDispatch : dispatch c a with
    | error e => rw [hDispatch] at hStep; cases hStep
    | ok next =>
        rw [hDispatch] at hStep
        cases hStep
        have hNext : resSound next := by
          cases a with
          | submitStatement actor text =>
              exact submitStatementCore_resSound c next actor text h hDispatch
          | submitVote actor vote rationale =>
              exact submitVoteCore_resSound c next actor vote rationale h hDispatch
          | failMember actor memberId reason =>
              exact failMemberCore_resSound c next actor memberId reason h hDispatch
        unfold resSound at hNext ⊢
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

theorem reachable_resSound
    (cfg : InitConfig) (c : CaseState)
    (h : Reachable cfg c) : resSound c := by
  obtain ⟨actions, hReplay⟩ := h
  unfold replay at hReplay
  cases hInit : initializeCase cfg with
  | error e => rw [hInit] at hReplay; cases hReplay
  | ok start =>
      rw [hInit] at hReplay
      exact replaySteps_preserves resSound
        (fun c a c' hP hs => step_resSound c c' a hP hs) actions start c
        (initializeCase_resSound cfg start hInit) hReplay

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

/-- An accepted certificate's substantive resolution is backed by the
vote threshold in the claimed state. -/
theorem checkCertificate_resSound
    (cfg : InitConfig) (actions : List Action) (claimed : CaseState)
    (h : checkCertificate cfg actions claimed = .ok ()) : resSound claimed :=
  reachable_resSound cfg claimed (checkCertificate_ok_reachable cfg actions claimed h)

end Vmcp
