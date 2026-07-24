import Lean

open Lean

namespace Vmcp

/-
The engine is the pure process core.  It holds the case state for a
simplified arbitration: an openings phase with one statement per side,
a deliberation phase with one vote per seated council member, and a
closed phase with a resolution.  Every state change goes through
`step`, and `obligation` reports who may act next with which tools.
-/

def trimString (s : String) : String :=
  s.trimAscii.toString

inductive Role where
  | plaintiff
  | defendant
  | council
  | system
  deriving Inhabited, DecidableEq, Repr

def Role.toString : Role → String
  | .plaintiff => "plaintiff"
  | .defendant => "defendant"
  | .council => "council"
  | .system => "system"

def Role.fromString? : String → Option Role
  | "plaintiff" => some .plaintiff
  | "defendant" => some .defendant
  | "council" => some .council
  | "system" => some .system
  | _ => none

instance : ToJson Role := ⟨fun r => Json.str r.toString⟩

instance : FromJson Role :=
  ⟨fun j => do
    let s ← j.getStr?
    match Role.fromString? s with
    | some r => pure r
    | none => throw s!"unknown role: {s}"⟩

/-- The actor of an action: a role, plus a member id for council. -/
structure Actor where
  role : Role
  member_id : String := ""
  deriving Inhabited, DecidableEq, Repr, ToJson, FromJson

inductive Vote where
  | demonstrated
  | not_demonstrated
  deriving Inhabited, DecidableEq, Repr

def Vote.toString : Vote → String
  | .demonstrated => "demonstrated"
  | .not_demonstrated => "not_demonstrated"

def Vote.fromString? : String → Option Vote
  | "demonstrated" => some .demonstrated
  | "not_demonstrated" => some .not_demonstrated
  | _ => none

instance : ToJson Vote := ⟨fun v => Json.str v.toString⟩

instance : FromJson Vote :=
  ⟨fun j => do
    let s ← j.getStr?
    match Vote.fromString? s with
    | some v => pure v
    | none => throw s!"unknown vote: {s}"⟩

inductive Phase where
  | openings
  | deliberation
  | closed
  deriving Inhabited, DecidableEq, Repr

def Phase.toString : Phase → String
  | .openings => "openings"
  | .deliberation => "deliberation"
  | .closed => "closed"

instance : ToJson Phase := ⟨fun p => Json.str p.toString⟩

instance : FromJson Phase :=
  ⟨fun j => do
    let s ← j.getStr?
    match s with
    | "openings" => pure .openings
    | "deliberation" => pure .deliberation
    | "closed" => pure .closed
    | _ => throw s!"unknown phase: {s}"⟩

inductive Resolution where
  | pending
  | demonstrated
  | not_demonstrated
  | no_majority
  deriving Inhabited, DecidableEq, Repr

def Resolution.toString : Resolution → String
  | .pending => "pending"
  | .demonstrated => "demonstrated"
  | .not_demonstrated => "not_demonstrated"
  | .no_majority => "no_majority"

instance : ToJson Resolution := ⟨fun r => Json.str r.toString⟩

instance : FromJson Resolution :=
  ⟨fun j => do
    let s ← j.getStr?
    match s with
    | "pending" => pure .pending
    | "demonstrated" => pure .demonstrated
    | "not_demonstrated" => pure .not_demonstrated
    | "no_majority" => pure .no_majority
    | _ => throw s!"unknown resolution: {s}"⟩

structure Statement where
  role : Role
  text : String
  deriving Inhabited, DecidableEq, Repr, ToJson, FromJson

structure CouncilMember where
  member_id : String
  seated : Bool := true
  failure_reason : String := ""
  deriving Inhabited, DecidableEq, Repr, ToJson, FromJson

structure CastVote where
  member_id : String
  vote : Vote
  rationale : String := ""
  deriving Inhabited, DecidableEq, Repr, ToJson, FromJson

structure Policy where
  required_votes : Nat
  max_statement_chars : Nat := 4000
  deriving Inhabited, DecidableEq, Repr, ToJson, FromJson

structure CaseState where
  case_id : String
  proposition : String
  policy : Policy
  phase : Phase := .openings
  members : List CouncilMember := []
  statements : List Statement := []
  votes : List CastVote := []
  resolution : Resolution := .pending
  state_version : Nat := 0
  deriving Inhabited, DecidableEq, Repr, ToJson, FromJson

/-- One action against the engine.  The actor comes from the session
binding, never from client input. -/
inductive Action where
  | submitStatement (actor : Actor) (text : String)
  | submitVote (actor : Actor) (vote : Vote) (rationale : String)
  | failMember (actor : Actor) (member_id : String) (reason : String)
  deriving Inhabited, DecidableEq, Repr

def Action.toJson : Action → Json
  | .submitStatement actor text =>
      Json.mkObj [("action", "submit_statement"), ("actor", ToJson.toJson actor), ("text", text)]
  | .submitVote actor vote rationale =>
      Json.mkObj [("action", "submit_vote"), ("actor", ToJson.toJson actor),
        ("vote", ToJson.toJson vote), ("rationale", rationale)]
  | .failMember actor memberId reason =>
      Json.mkObj [("action", "fail_member"), ("actor", ToJson.toJson actor),
        ("member_id", memberId), ("reason", reason)]

instance : ToJson Action := ⟨Action.toJson⟩

def Action.fromJson? (j : Json) : Except String Action := do
  let kind ← (← j.getObjVal? "action").getStr?
  let actor ← j.getObjValAs? Actor "actor"
  match kind with
  | "submit_statement" =>
      pure <| .submitStatement actor (← (← j.getObjVal? "text").getStr?)
  | "submit_vote" =>
      pure <| .submitVote actor (← j.getObjValAs? Vote "vote")
        (← (← j.getObjVal? "rationale").getStr?)
  | "fail_member" =>
      pure <| .failMember actor (← (← j.getObjVal? "member_id").getStr?)
        (← (← j.getObjVal? "reason").getStr?)
  | other => throw s!"unknown action: {other}"

instance : FromJson Action := ⟨Action.fromJson?⟩

/-- The actor an action claims to act as. -/
def Action.actor : Action → Actor
  | .submitStatement actor _ => actor
  | .submitVote actor _ _ => actor
  | .failMember actor _ _ => actor

def seatedMembers (c : CaseState) : List CouncilMember :=
  c.members.filter (fun m => m.seated)

def seatedCount (c : CaseState) : Nat :=
  (seatedMembers c).length

def hasVoted (c : CaseState) (memberId : String) : Bool :=
  c.votes.any (fun v => v.member_id = memberId)

def voteCount (c : CaseState) (value : Vote) : Nat :=
  c.votes.foldl (fun acc v => if v.vote = value then acc + 1 else acc) 0

def statementFor (c : CaseState) (role : Role) : Bool :=
  c.statements.any (fun s => s.role = role)

def nextVoter? (c : CaseState) : Option CouncilMember :=
  (seatedMembers c).find? (fun m => !hasVoted c m.member_id)

/-- The current obligation: who may act, and with which tool. -/
structure Obligation where
  role : Role
  member_id : String := ""
  tool : String
  deriving Inhabited, DecidableEq, Repr, ToJson

/-- Obligations for the current state.  During deliberation the system
role also holds the member-failure tool. -/
def obligations (c : CaseState) : List Obligation :=
  match c.phase with
  | .openings =>
      if !statementFor c .plaintiff then
        [{ role := .plaintiff, tool := "submit_statement" }]
      else
        [{ role := .defendant, tool := "submit_statement" }]
  | .deliberation =>
      let voter :=
        match nextVoter? c with
        | some m => [{ role := .council, member_id := m.member_id, tool := "submit_vote" }]
        | none => []
      voter ++ [{ role := .system, tool := "fail_member" }]
  | .closed => []

def bumpVersion (c : CaseState) : CaseState :=
  { c with state_version := c.state_version + 1 }

/-- Close the case when deliberation has resolved.  A resolution exists
when a vote value reaches the threshold, when every seated member has
voted without one, or when the seated count can no longer reach the
threshold. -/
def resolveDeliberation (c : CaseState) : CaseState :=
  if voteCount c .demonstrated >= c.policy.required_votes then
    { c with phase := .closed, resolution := .demonstrated }
  else if voteCount c .not_demonstrated >= c.policy.required_votes then
    { c with phase := .closed, resolution := .not_demonstrated }
  else if seatedCount c < c.policy.required_votes then
    { c with phase := .closed, resolution := .no_majority }
  else if (nextVoter? c).isNone then
    { c with phase := .closed, resolution := .no_majority }
  else
    c

/-- The side expected to file next. -/
def expectedSide (c : CaseState) : Role :=
  if !statementFor c .plaintiff then .plaintiff else .defendant

def withStatement (c : CaseState) (role : Role) (text : String) : CaseState :=
  { c with statements := c.statements.concat { role := role, text := text } }

/-- Append a statement and advance to deliberation when both sides have
filed. -/
def afterStatement (c : CaseState) (role : Role) (text : String) : CaseState :=
  if statementFor (withStatement c role text) .plaintiff &&
      statementFor (withStatement c role text) .defendant then
    { withStatement c role text with phase := .deliberation }
  else
    withStatement c role text

def submitStatementCore (c : CaseState) (actor : Actor) (text : String) : Except String CaseState :=
  if c.phase ≠ .openings then
    .error "statements are allowed only in openings"
  else if trimString text = "" then
    .error "statement text is required"
  else if (trimString text).length > c.policy.max_statement_chars then
    .error s!"statement exceeds character limit of {c.policy.max_statement_chars}"
  else if actor.role ≠ expectedSide c then
    .error s!"expected {(expectedSide c).toString} to act"
  else
    .ok (afterStatement c (expectedSide c) (trimString text))

def submitVoteCore (c : CaseState) (actor : Actor) (vote : Vote) (rationale : String) : Except String CaseState :=
  if c.phase ≠ .deliberation then
    .error "votes are allowed only in deliberation"
  else if actor.role ≠ .council then
    .error "only council members vote"
  else
    match nextVoter? c with
    | none => .error "no council member awaits a vote"
    | some m =>
        if m.member_id ≠ actor.member_id then
          .error s!"expected vote by {m.member_id}"
        else
          .ok (resolveDeliberation { c with votes := c.votes.concat {
            member_id := actor.member_id
            vote := vote
            rationale := trimString rationale
          } })

def failMemberCore (c : CaseState) (actor : Actor) (memberId reason : String) : Except String CaseState :=
  if c.phase ≠ .deliberation then
    .error "member failure is allowed only in deliberation"
  else if actor.role ≠ .system then
    .error "only the system fails members"
  else if trimString reason = "" then
    .error "member failure requires a reason"
  else if !(c.members.any (fun m => m.member_id = memberId && m.seated)) then
    .error s!"member is unknown or not seated: {memberId}"
  else if hasVoted c memberId then
    .error s!"cannot fail a member who has voted: {memberId}"
  else
    .ok (resolveDeliberation { c with members := c.members.map (fun m =>
      if m.member_id = memberId then
        { m with seated := false, failure_reason := trimString reason }
      else
        m) })

def dispatch (c : CaseState) : Action → Except String CaseState
  | .submitStatement actor text => submitStatementCore c actor text
  | .submitVote actor vote rationale => submitVoteCore c actor vote rationale
  | .failMember actor memberId reason => failMemberCore c actor memberId reason

def step (c : CaseState) (a : Action) : Except String CaseState :=
  if c.phase = .closed then
    .error "case is closed"
  else
    match dispatch c a with
    | .error e => .error e
    | .ok next => .ok (bumpVersion next)

structure InitConfig where
  case_id : String
  proposition : String
  policy : Policy
  member_ids : List String
  deriving Inhabited, DecidableEq, Repr, ToJson, FromJson

def hasDuplicate : List String → Bool
  | [] => false
  | x :: xs => xs.contains x || hasDuplicate xs

def initializeCase (cfg : InitConfig) : Except String CaseState :=
  if trimString cfg.proposition = "" then
    .error "proposition is required"
  else if cfg.member_ids.isEmpty then
    .error "at least one council member is required"
  else if cfg.member_ids.any (fun id => trimString id = "" || trimString id ≠ id) then
    .error "member ids must be trimmed and non-empty"
  else if hasDuplicate cfg.member_ids then
    .error "member ids must be distinct"
  else if cfg.policy.required_votes = 0 then
    .error "policy.required_votes must be positive"
  else if 2 * cfg.policy.required_votes ≤ cfg.member_ids.length then
    .error "policy.required_votes must be a strict majority of the council"
  else if cfg.policy.max_statement_chars = 0 then
    .error "policy.max_statement_chars must be positive"
  else
    .ok {
      case_id := cfg.case_id
      proposition := trimString cfg.proposition
      policy := cfg.policy
      members := cfg.member_ids.map (fun id => { member_id := id })
    }

/-- Fold accepted actions through `step`. -/
def replaySteps : CaseState → List Action → Except String CaseState
  | c, [] => .ok c
  | c, a :: rest =>
      match step c a with
      | .error e => .error e
      | .ok next => replaySteps next rest

/-- Replay: fold accepted actions from the initialized state. -/
def replay (cfg : InitConfig) (actions : List Action) : Except String CaseState :=
  match initializeCase cfg with
  | .error e => .error e
  | .ok start => replaySteps start actions

/-- Certificate check: replay and compare with the claimed final state. -/
def checkCertificate (cfg : InitConfig) (actions : List Action) (claimed : CaseState) : Except String Unit :=
  match replay cfg actions with
  | .error e => .error e
  | .ok final =>
      if final = claimed then
        .ok ()
      else
        .error "final state mismatch"

end Vmcp
