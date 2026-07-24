import Vmcp.Gate
import Proofs.Engine

open Lean

namespace Vmcp

/-
Gate proofs.  Three facts carry the design:

1. Stamping: every action `parseCall` builds carries the actor it was
   given, which the gate takes from the session binding.
2. Advertisement: every tool offered to a session comes from an engine
   obligation matching the session's binding.
3. No bypass: the gate's engine state changes only through `step` on a
   stamped action from a bound session.
-/

theorem parseCall_actor
    (actor : Actor) (name : String) (args : Json) (a : Action)
    (h : parseCall actor name args = .ok a) : a.actor = actor := by
  unfold parseCall at h
  repeat' split at h
  any_goals simp at h
  all_goals cases h
  all_goals rfl

theorem toolsFor_sound
    (g : GateState) (a : Actor) (t : String)
    (h : t ∈ toolsFor g a) :
    ∃ ob ∈ obligations g.engine,
      ob.tool = t ∧ ob.role = a.role ∧ (ob.role = .council → ob.member_id = a.member_id) := by
  unfold toolsFor at h
  rw [List.mem_filterMap] at h
  obtain ⟨ob, hmem, hcond⟩ := h
  refine ⟨ob, hmem, ?_⟩
  split at hcond
  · rename_i hguard
    cases hcond
    simp only [Bool.and_eq_true, decide_eq_true_eq, Bool.or_eq_true, bne_iff_ne] at hguard
    obtain ⟨hrole, hmemb⟩ := hguard
    refine ⟨rfl, hrole, ?_⟩
    intro hc
    cases hmemb with
    | inl hne => exact absurd hc hne
    | inr heq => simpa using heq
  · cases hcond

theorem toolsFor_complete
    (g : GateState) (a : Actor) (ob : Obligation)
    (hmem : ob ∈ obligations g.engine)
    (hrole : ob.role = a.role)
    (hmemb : ob.role = .council → ob.member_id = a.member_id) :
    ob.tool ∈ toolsFor g a := by
  unfold toolsFor
  rw [List.mem_filterMap]
  refine ⟨ob, hmem, ?_⟩
  have hguard :
      (ob.role = a.role && (ob.role != .council || ob.member_id = a.member_id)) = true := by
    by_cases hc : ob.role = .council <;> simp_all
  simp [hguard]

theorem findSession_id
    (g : GateState) (sessionId : String) (s : Session)
    (h : findSession g sessionId = some s) : s.session_id = sessionId := by
  unfold findSession at h
  have := List.find?_some h
  simpa using this

theorem handleCall_no_bypass
    (g : GateState) (s : Session) (id params : Json) :
    (handleCall g s id params).1.engine = g.engine ∨
      ∃ a : Action, a.actor = s.actor ∧
        step g.engine a = .ok (handleCall g s id params).1.engine := by
  cases hParse : parseCall s.actor (callName params) (callArgs params) with
  | error e => left; simp [handleCall, hParse]
  | ok action =>
      cases hStep : step g.engine action with
      | error e => left; simp [handleCall, hParse, hStep]
      | ok engine1 =>
          right
          refine ⟨action, parseCall_actor s.actor _ _ action hParse, ?_⟩
          simp [handleCall, hParse, hStep]

theorem gateStepList_engine
    (g : GateState) (sessionId : String) (id : Json) :
    (gateStepList g sessionId id).1.engine = g.engine := by
  unfold gateStepList
  split <;> rfl

theorem gateStepCall_no_bypass
    (g : GateState) (sessionId : String) (id params : Json) :
    (gateStepCall g sessionId id params).1.engine = g.engine ∨
      ∃ s : Session, findSession g sessionId = some s ∧
        ∃ a : Action, a.actor = s.actor ∧
          step g.engine a = .ok (gateStepCall g sessionId id params).1.engine := by
  cases hFound : findSession g sessionId with
  | none => left; simp [gateStepCall, hFound]
  | some s =>
      simp only [gateStepCall, hFound]
      cases handleCall_no_bypass g s id params with
      | inl hSame => left; exact hSame
      | inr hEx =>
          right
          obtain ⟨a, hActor, hOk⟩ := hEx
          exact ⟨s, rfl, a, hActor, hOk⟩

/-- An engine change carries its log record: when a call changes the
engine state, the commands include the appended record for the stamped
action that produced it. -/
theorem handleCall_change_logged
    (g : GateState) (s : Session) (id params : Json)
    (hChange : (handleCall g s id params).1.engine ≠ g.engine) :
    ∃ a : Action, a.actor = s.actor ∧
      step g.engine a = .ok (handleCall g s id params).1.engine ∧
      Command.appendLog (logRecord s.session_id a (handleCall g s id params).1.engine)
        ∈ (handleCall g s id params).2 := by
  cases hParse : parseCall s.actor (callName params) (callArgs params) with
  | error e => exact absurd (by simp [handleCall, hParse]) hChange
  | ok action =>
      cases hStep : step g.engine action with
      | error e => exact absurd (by simp [handleCall, hParse, hStep]) hChange
      | ok engine1 =>
          refine ⟨action, parseCall_actor s.actor _ _ action hParse, ?_, ?_⟩
          · simp [handleCall, hParse, hStep]
          · simp [handleCall, hParse, hStep]

theorem gateStepCall_change_logged
    (g : GateState) (sessionId : String) (id params : Json)
    (hChange : (gateStepCall g sessionId id params).1.engine ≠ g.engine) :
    ∃ s : Session, findSession g sessionId = some s ∧
      ∃ a : Action, a.actor = s.actor ∧
        step g.engine a = .ok (gateStepCall g sessionId id params).1.engine ∧
        Command.appendLog (logRecord sessionId a (gateStepCall g sessionId id params).1.engine)
          ∈ (gateStepCall g sessionId id params).2 := by
  cases hFound : findSession g sessionId with
  | none => exact absurd (by simp [gateStepCall, hFound]) hChange
  | some s =>
      simp only [gateStepCall, hFound] at hChange ⊢
      obtain ⟨a, hActor, hOk, hLog⟩ := handleCall_change_logged g s id params hChange
      exact ⟨s, rfl, a, hActor, hOk,
        findSession_id g sessionId s hFound ▸ hLog⟩

theorem gateStep_no_bypass
    (g : GateState) (i : Inbound) :
    (gateStep g i).1.engine = g.engine ∨
      ∃ s : Session, findSession g i.session = some s ∧
        ∃ a : Action, a.actor = s.actor ∧
          step g.engine a = .ok (gateStep g i).1.engine := by
  unfold gateStep
  repeat' split
  all_goals first
    | (left; rfl)
    | (left; exact gateStepList_engine g i.session (msgId i.payload))
    | (cases gateStepCall_no_bypass g i.session (msgId i.payload) (msgParams i.payload) with
       | inl hSame => left; exact hSame
       | inr hEx => right; exact hEx)

end Vmcp
