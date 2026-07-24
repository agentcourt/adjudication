import Vmcp.Engine

open Lean

namespace Vmcp

/-
Codec round-trip lemmas: decoding an encoded value yields the value.
These close the seam between the proven engine model and the wire
format the log and state files use.
-/

theorem role_roundtrip (r : Role) :
    (fromJson? (toJson r) : Except String Role) = .ok r := by
  cases r <;> rfl

theorem vote_roundtrip (v : Vote) :
    (fromJson? (toJson v) : Except String Vote) = .ok v := by
  cases v <;> rfl

theorem phase_roundtrip (p : Phase) :
    (fromJson? (toJson p) : Except String Phase) = .ok p := by
  cases p <;> rfl

theorem resolution_roundtrip (r : Resolution) :
    (fromJson? (toJson r) : Except String Resolution) = .ok r := by
  cases r <;> rfl

theorem nat_roundtrip (n : Nat) : decNat (encNat n) = .ok n := by
  show (if h : 0 ≤ (n : Int) then Except.ok (Int.toNat n) else
    Except.error "number must be a natural") = .ok n
  rw [dif_pos (by exact Int.natCast_nonneg n)]
  simp

theorem mapM_map_roundtrip
    (f : Json → Except String α) (g : α → Json)
    (h : ∀ x, f (g x) = .ok x) :
    ∀ xs : List α, (xs.map g).mapM f = .ok xs
  | [] => rfl
  | x :: rest => by
      rw [List.map_cons, List.mapM_cons, h x, mapM_map_roundtrip f g h rest]
      rfl

theorem decList_encList
    (f : Json → Except String α) (g : α → Json)
    (h : ∀ x, f (g x) = .ok x) (xs : List α) :
    decList f (encList g xs) = .ok xs := by
  show ((xs.map g).toArray).toList.mapM f = .ok xs
  have harr : ((xs.map g).toArray).toList = xs.map g := by simp
  rw [harr]
  exact mapM_map_roundtrip f g h xs

theorem actor_roundtrip (a : Actor) : Actor.dec (Actor.enc a) = .ok a := by
  have hr : (Actor.enc a).getObjVal? "role" = .ok (toJson a.role) := rfl
  have hm : (Actor.enc a).getObjVal? "member_id" = .ok (Json.str a.member_id) := rfl
  simp [Actor.dec, decField, hr, hm, role_roundtrip, decStr]

theorem statement_roundtrip (s : Statement) : Statement.dec (Statement.enc s) = .ok s := by
  have hr : (Statement.enc s).getObjVal? "role" = .ok (toJson s.role) := rfl
  have ht : (Statement.enc s).getObjVal? "text" = .ok (Json.str s.text) := rfl
  simp [Statement.dec, decField, hr, ht, role_roundtrip, decStr]

theorem councilMember_roundtrip (m : CouncilMember) :
    CouncilMember.dec (CouncilMember.enc m) = .ok m := by
  have h1 : (CouncilMember.enc m).getObjVal? "member_id" = .ok (Json.str m.member_id) := rfl
  have h2 : (CouncilMember.enc m).getObjVal? "seated" = .ok (Json.bool m.seated) := rfl
  have h3 : (CouncilMember.enc m).getObjVal? "failure_reason" = .ok (Json.str m.failure_reason) := rfl
  simp [CouncilMember.dec, decField, h1, h2, h3, decStr, decBool]

theorem castVote_roundtrip (v : CastVote) : CastVote.dec (CastVote.enc v) = .ok v := by
  have h1 : (CastVote.enc v).getObjVal? "member_id" = .ok (Json.str v.member_id) := rfl
  have h2 : (CastVote.enc v).getObjVal? "vote" = .ok (toJson v.vote) := rfl
  have h3 : (CastVote.enc v).getObjVal? "rationale" = .ok (Json.str v.rationale) := rfl
  simp [CastVote.dec, decField, h1, h2, h3, decStr, vote_roundtrip]

theorem policy_roundtrip (p : Policy) : Policy.dec (Policy.enc p) = .ok p := by
  have h1 : (Policy.enc p).getObjVal? "required_votes" = .ok (encNat p.required_votes) := rfl
  have h2 : (Policy.enc p).getObjVal? "max_statement_chars" = .ok (encNat p.max_statement_chars) := rfl
  simp [Policy.dec, decField, h1, h2, nat_roundtrip]

theorem caseState_roundtrip (c : CaseState) :
    (fromJson? (toJson c) : Except String CaseState) = .ok c := by
  show CaseState.dec (CaseState.enc c) = .ok c
  have h1 : (CaseState.enc c).getObjVal? "case_id" = .ok (Json.str c.case_id) := rfl
  have h2 : (CaseState.enc c).getObjVal? "proposition" = .ok (Json.str c.proposition) := rfl
  have h3 : (CaseState.enc c).getObjVal? "policy" = .ok (Policy.enc c.policy) := rfl
  have h4 : (CaseState.enc c).getObjVal? "phase" = .ok (toJson c.phase) := rfl
  have h5 : (CaseState.enc c).getObjVal? "members" = .ok (encList CouncilMember.enc c.members) := rfl
  have h6 : (CaseState.enc c).getObjVal? "statements" = .ok (encList Statement.enc c.statements) := rfl
  have h7 : (CaseState.enc c).getObjVal? "votes" = .ok (encList CastVote.enc c.votes) := rfl
  have h8 : (CaseState.enc c).getObjVal? "resolution" = .ok (toJson c.resolution) := rfl
  have h9 : (CaseState.enc c).getObjVal? "state_version" = .ok (encNat c.state_version) := rfl
  simp [CaseState.dec, decField, h1, h2, h3, h4, h5, h6, h7, h8, h9,
    decStr, policy_roundtrip, phase_roundtrip, resolution_roundtrip, nat_roundtrip,
    decList_encList CouncilMember.dec CouncilMember.enc councilMember_roundtrip,
    decList_encList Statement.dec Statement.enc statement_roundtrip,
    decList_encList CastVote.dec CastVote.enc castVote_roundtrip]

theorem action_roundtrip (a : Action) :
    (fromJson? (toJson a) : Except String Action) = .ok a := by
  show Action.fromJson? (Action.toJson a) = .ok a
  cases a with
  | submitStatement actor text =>
      have h1 : (Action.toJson (.submitStatement actor text)).getObjVal? "action"
        = .ok (Json.str "submit_statement") := rfl
      have h2 : (Action.toJson (.submitStatement actor text)).getObjVal? "actor"
        = .ok (Actor.enc actor) := rfl
      have h3 : (Action.toJson (.submitStatement actor text)).getObjVal? "text"
        = .ok (Json.str text) := rfl
      simp [Action.fromJson?, decField, h1, h2, h3, decStr, actor_roundtrip]
  | submitVote actor vote rationale =>
      have h1 : (Action.toJson (.submitVote actor vote rationale)).getObjVal? "action"
        = .ok (Json.str "submit_vote") := rfl
      have h2 : (Action.toJson (.submitVote actor vote rationale)).getObjVal? "actor"
        = .ok (Actor.enc actor) := rfl
      have h3 : (Action.toJson (.submitVote actor vote rationale)).getObjVal? "vote"
        = .ok (toJson vote) := rfl
      have h4 : (Action.toJson (.submitVote actor vote rationale)).getObjVal? "rationale"
        = .ok (Json.str rationale) := rfl
      simp [Action.fromJson?, decField, h1, h2, h3, h4, decStr,
        actor_roundtrip, vote_roundtrip]
  | failMember actor memberId reason =>
      have h1 : (Action.toJson (.failMember actor memberId reason)).getObjVal? "action"
        = .ok (Json.str "fail_member") := rfl
      have h2 : (Action.toJson (.failMember actor memberId reason)).getObjVal? "actor"
        = .ok (Actor.enc actor) := rfl
      have h3 : (Action.toJson (.failMember actor memberId reason)).getObjVal? "member_id"
        = .ok (Json.str memberId) := rfl
      have h4 : (Action.toJson (.failMember actor memberId reason)).getObjVal? "reason"
        = .ok (Json.str reason) := rfl
      simp [Action.fromJson?, decField, h1, h2, h3, h4, decStr, actor_roundtrip]

end Vmcp
