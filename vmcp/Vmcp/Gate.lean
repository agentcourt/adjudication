import Vmcp.Engine

open Lean

namespace Vmcp

/-
The gate is the pure protocol layer.  It parses JSON-RPC messages
arriving on named sessions, binds sessions to principals at
initialization, stamps every engine action with the session's binding,
and emits commands for the shell to execute.  The shell owns transport
and files; this file performs no I/O.
-/

/-- One configured principal.  The token authenticates a connection, and
the binding names who that connection is. -/
structure Principal where
  token : String
  role : Role
  member_id : String := ""
  deriving Inhabited, DecidableEq, Repr, ToJson, FromJson

structure GateConfig where
  init : InitConfig
  principals : List Principal
  deriving Inhabited, DecidableEq, Repr, ToJson, FromJson

structure Session where
  session_id : String
  token : String
  actor : Actor
  deriving Inhabited, DecidableEq, Repr, ToJson, FromJson

structure GateState where
  config : GateConfig
  engine : CaseState
  sessions : List Session := []
  deriving Inhabited, Repr

/-- One inbound line from the shell: a session name and a JSON-RPC
payload. -/
structure Inbound where
  session : String
  payload : Json
  deriving Inhabited

/-- Commands for the shell.  `reply` and `notify` carry outbound JSON
for a session; `appendLog` records an accepted action. -/
inductive Command where
  | reply (session : String) (payload : Json)
  | notify (session : String) (payload : Json)
  | appendLog (record : Json)
  deriving Inhabited

def jsonrpcResult (id : Json) (result : Json) : Json :=
  Json.mkObj [("jsonrpc", "2.0"), ("id", id), ("result", result)]

def jsonrpcError (id : Json) (code : Int) (message : String) : Json :=
  Json.mkObj [("jsonrpc", "2.0"), ("id", id),
    ("error", Json.mkObj [("code", Json.num code), ("message", message)])]

def toolListChanged : Json :=
  Json.mkObj [("jsonrpc", "2.0"), ("method", "notifications/tools/list_changed")]

def findSession (g : GateState) (sessionId : String) : Option Session :=
  g.sessions.find? (fun s => s.session_id = sessionId)

def findPrincipal (g : GateState) (token : String) : Option Principal :=
  g.config.principals.find? (fun p => p.token = token)

/-- An obligation matches a binding when the roles agree and, when the
obligation names a member, the member agrees.  The rule reads only the
obligation's own fields, so the gate carries no procedure knowledge. -/
def bindingMatches (ob : Obligation) (actor : Actor) : Bool :=
  ob.role = actor.role && (ob.member_id = "" || ob.member_id = actor.member_id)

/-- Tool definitions offered to a session, derived from the engine's
current obligations filtered by the session's binding. -/
def toolsFor (g : GateState) (actor : Actor) : List String :=
  (obligations g.engine).filterMap (fun ob =>
    if bindingMatches ob actor then
      some ob.tool
    else
      none)

def toolJson (name : String) : Json :=
  let schema := fun (props : List (String × Json)) =>
    Json.mkObj [("type", "object"), ("properties", Json.mkObj props)]
  let textProp := Json.mkObj [("type", "string")]
  match name with
  | "submit_statement" =>
      Json.mkObj [("name", name), ("description", "File the statement for your side."),
        ("inputSchema", schema [("text", textProp)])]
  | "submit_vote" =>
      Json.mkObj [("name", name), ("description", "Cast your vote with a rationale."),
        ("inputSchema", schema [("vote", textProp), ("rationale", textProp)])]
  | "fail_member" =>
      Json.mkObj [("name", name), ("description", "Record a council member failure."),
        ("inputSchema", schema [("member_id", textProp), ("reason", textProp)])]
  | _ =>
      Json.mkObj [("name", name), ("inputSchema", schema [])]

def toolListResult (tools : List String) : Json :=
  Json.mkObj [("tools", Json.arr (tools.map toolJson).toArray)]

def getStringField (j : Json) (k : String) : Except String String :=
  match j.getObjVal? k with
  | .error e => .error e
  | .ok v =>
      match v.getStr? with
      | .error e => .error e
      | .ok s => .ok s

/-- An absent field is "", and a present field must be a string. -/
def optionalString (j : Json) (k : String) : Except String String :=
  match j.getObjVal? k with
  | .error _ => .ok ""
  | .ok v =>
      match v.getStr? with
      | .error _ => .error s!"{k} must be a string"
      | .ok s => .ok s

def optionalStringField (j : Json) (k : String) : String :=
  match getStringField j k with
  | .ok s => s
  | .error _ => ""

/-- Parse a tools/call into an engine action, stamped with the session's
actor.  Argument fields come from the client; identity never does. -/
def parseCall (actor : Actor) (name : String) (args : Json) : Except String Action :=
  match name with
  | "submit_statement" =>
      match getStringField args "text" with
      | .error e => .error e
      | .ok text => .ok (.submitStatement actor text)
  | "submit_vote" =>
      match getStringField args "vote" with
      | .error e => .error e
      | .ok raw =>
          match Vote.fromString? raw with
          | none => .error s!"unknown vote: {raw}"
          | some vote =>
              match optionalString args "rationale" with
              | .error e => .error e
              | .ok rationale => .ok (.submitVote actor vote rationale)
  | "fail_member" =>
      match getStringField args "member_id" with
      | .error e => .error e
      | .ok memberId =>
          match getStringField args "reason" with
          | .error e => .error e
          | .ok reason => .ok (.failMember actor memberId reason)
  | other => .error s!"unknown tool: {other}"

/-- Notifications for sessions whose advertised tools changed between
two states. -/
def changeNotifications (g0 g1 : GateState) : List Command :=
  g1.sessions.filterMap (fun s =>
    if toolsFor g0 s.actor = toolsFor g1 s.actor then
      none
    else
      some (Command.notify s.session_id toolListChanged))

def logRecord (sessionId : String) (a : Action) (result : CaseState) : Json :=
  Json.mkObj [
    ("session", sessionId),
    ("action", ToJson.toJson a),
    ("state_version", Json.num result.state_version)
  ]

def msgId (payload : Json) : Json :=
  match payload.getObjVal? "id" with
  | .ok v => v
  | .error _ => Json.null

/-- A JSON-RPC message without an id is a notification.  Notifications
never receive a response. -/
def isRequest (payload : Json) : Bool :=
  match payload.getObjVal? "id" with
  | .ok _ => true
  | .error _ => false

def msgMethod (payload : Json) : String :=
  optionalStringField payload "method"

def msgParams (payload : Json) : Json :=
  match payload.getObjVal? "params" with
  | .ok v => v
  | .error _ => Json.mkObj []

def callName (params : Json) : String :=
  optionalStringField params "name"

def callArgs (params : Json) : Json :=
  match params.getObjVal? "arguments" with
  | .ok v => v
  | .error _ => Json.mkObj []

/-- Handle one tools/call on a bound session. -/
def handleCall (g : GateState) (s : Session) (id : Json) (params : Json) :
    GateState × List Command :=
  match parseCall s.actor (callName params) (callArgs params) with
  | .error err => (g, [.reply s.session_id (jsonrpcError id (-32602) err)])
  | .ok action =>
      match step g.engine action with
      | .error err => (g, [.reply s.session_id (jsonrpcError id (-32000) err)])
      | .ok engine1 =>
          let g1 := { g with engine := engine1 }
          let content := Json.mkObj [("type", "text"),
            ("text", s!"accepted; state_version {engine1.state_version}")]
          let result := Json.mkObj [("content", Json.arr #[content])]
          (g1,
            [.appendLog (logRecord s.session_id action engine1),
             .reply s.session_id (jsonrpcResult id result)] ++
            changeNotifications g g1)

def gateStepList (g : GateState) (sessionId : String) (id : Json) :
    GateState × List Command :=
  match findSession g sessionId with
  | none => (g, [.reply sessionId (jsonrpcError id (-32002) "session is not initialized")])
  | some s => (g, [.reply sessionId (jsonrpcResult id (toolListResult (toolsFor g s.actor)))])

def gateStepCall (g : GateState) (sessionId : String) (id params : Json) :
    GateState × List Command :=
  match findSession g sessionId with
  | none => (g, [.reply sessionId (jsonrpcError id (-32002) "session is not initialized")])
  | some s => handleCall g s id params

/-- Handle one inbound message.  Notifications are ignored without a
response, as JSON-RPC requires; the engine acts only on requests. -/
def gateStep (g : GateState) (i : Inbound) : GateState × List Command :=
  if !isRequest i.payload then
    (g, [])
  else
  match msgMethod i.payload with
  | "initialize" =>
      match getStringField (msgParams i.payload) "token" with
      | .error e => (g, [.reply i.session (jsonrpcError (msgId i.payload) (-32602) s!"token: {e}")])
      | .ok token =>
          match findPrincipal g token with
          | none => (g, [.reply i.session (jsonrpcError (msgId i.payload) (-32001) "unknown token")])
          | some p =>
              if g.sessions.any (fun s => s.token = token && s.session_id != i.session) then
                (g, [.reply i.session (jsonrpcError (msgId i.payload) (-32001)
                  "token is bound to another live session")])
              else
                let sessions := (g.sessions.filter (fun s => s.session_id != i.session)).concat
                  { session_id := i.session, token := token,
                    actor := { role := p.role, member_id := p.member_id } }
                let result := Json.mkObj [
                  ("protocolVersion", "2025-06-18"),
                  ("capabilities", Json.mkObj [("tools", Json.mkObj [("listChanged", true)])]),
                  ("serverInfo", Json.mkObj [("name", "vmcp"), ("version", "0.1.0")])
                ]
                ({ g with sessions := sessions }, [.reply i.session (jsonrpcResult (msgId i.payload) result)])
  | "tools/list" => gateStepList g i.session (msgId i.payload)
  | "tools/call" => gateStepCall g i.session (msgId i.payload) (msgParams i.payload)
  | "" =>
      (g, [.reply i.session (jsonrpcError (msgId i.payload) (-32600) "method is required")])
  | other =>
      (g, [.reply i.session (jsonrpcError (msgId i.payload) (-32601) s!"unknown method: {other}")])

def initialGateState (cfg : GateConfig) : Except String GateState := do
  if cfg.principals.any (fun p => trimString p.token = "") then
    throw "principals need non-empty tokens"
  if hasDuplicate (cfg.principals.map (fun p => p.token)) then
    throw "principal tokens must be distinct"
  let councilIds := cfg.principals.filterMap (fun p =>
    if p.role = .council then some p.member_id else none)
  if councilIds != cfg.init.member_ids then
    throw "council principals must match init.member_ids in order"
  let engine ← initializeCase cfg.init
  pure { config := cfg, engine := engine }

end Vmcp
