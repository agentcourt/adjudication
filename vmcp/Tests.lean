import Vmcp

/-
Compile-time tests.  Every `#guard` evaluates during the build, so
`lake build` fails when a behavior regresses.  These cover the engine
paths the demo script does not reach and concrete codec round trips.
-/

open Lean Vmcp

def testCfg : InitConfig := {
  case_id := "t"
  proposition := "P"
  policy := { required_votes := 2, max_statement_chars := 100 }
  member_ids := ["C1", "C2", "C3"]
}

def plaintiff : Actor := { role := .plaintiff }
def defendant : Actor := { role := .defendant }
def operator : Actor := { role := .system }
def juror (id : String) : Actor := { role := .council, member_id := id }

def openingActions : List Action := [
  .submitStatement plaintiff "p says",
  .submitStatement defendant "d says"
]

def happyActions : List Action := openingActions ++ [
  .submitVote (juror "C1") .demonstrated "r1",
  .submitVote (juror "C2") .demonstrated "r2"
]

-- The happy path closes demonstrated after two of three votes.
def happyFinal : CaseState := (replay testCfg happyActions).toOption.getD default
#guard replay testCfg happyActions matches .ok _
#guard happyFinal.resolution = .demonstrated
#guard happyFinal.phase = .closed

-- The happy certificate verifies, and a flipped resolution fails.
#guard checkCertificate testCfg happyActions happyFinal matches .ok _
#guard checkCertificate testCfg happyActions
  { happyFinal with resolution := .not_demonstrated } matches .error _

-- Rejections: wrong side first, vote before deliberation, out-of-order
-- vote, double vote, failing a member who voted.
#guard replay testCfg [.submitStatement defendant "first"] matches .error _
#guard replay testCfg [.submitVote (juror "C1") .demonstrated ""] matches .error _
#guard replay testCfg (openingActions ++ [.submitVote (juror "C2") .demonstrated ""])
  matches .error _
#guard replay testCfg (happyActions ++ [.submitVote (juror "C1") .demonstrated ""])
  matches .error _
#guard replay testCfg (openingActions ++ [
  .submitVote (juror "C1") .demonstrated "r1",
  .failMember operator "C1" "late"]) matches .error _

-- Failing a member and splitting the remaining votes closes no_majority
-- with the failed member unseated.
def splitFinal : CaseState := (replay testCfg (openingActions ++ [
  .failMember operator "C3" "unresponsive",
  .submitVote (juror "C1") .demonstrated "r1",
  .submitVote (juror "C2") .not_demonstrated "r2"])).toOption.getD default
#guard splitFinal.resolution = .no_majority
#guard splitFinal.phase = .closed
#guard seatedCount splitFinal = 2

-- Failing enough members to make the threshold unreachable closes
-- no_majority at once.
def unreachableFinal : CaseState := (replay testCfg (openingActions ++ [
  .failMember operator "C1" "gone",
  .failMember operator "C2" "gone"])).toOption.getD default
#guard unreachableFinal.resolution = .no_majority
#guard unreachableFinal.phase = .closed

-- Initialization validation.
#guard initializeCase { testCfg with member_ids := ["C1", "C1", "C2"] } matches .error _
#guard initializeCase { testCfg with policy.required_votes := 1 } matches .error _
#guard initializeCase { testCfg with proposition := "  " } matches .error _

-- Concrete codec round trips for the log record types and the state.
#guard (fromJson? (toJson (Action.submitVote (juror "C1") .demonstrated "why"))
  : Except String Action).toOption = some (.submitVote (juror "C1") .demonstrated "why")
#guard (fromJson? (toJson (Action.submitStatement plaintiff "text"))
  : Except String Action).toOption = some (.submitStatement plaintiff "text")
#guard (fromJson? (toJson (Action.failMember operator "C2" "reason"))
  : Except String Action).toOption = some (.failMember operator "C2" "reason")
#guard (fromJson? (toJson happyFinal) : Except String CaseState).toOption = some happyFinal

-- Gate: notifications get no commands; an unknown token and a second
-- binding of a bound token get error replies without a new session.
def testGateCfg : GateConfig := {
  init := testCfg
  principals := [
    { token := "tp", role := .plaintiff },
    { token := "tc1", role := .council, member_id := "C1" },
    { token := "tc2", role := .council, member_id := "C2" },
    { token := "tc3", role := .council, member_id := "C3" }
  ]
}

def initMsg (token : String) : Json :=
  Json.mkObj [("jsonrpc", "2.0"), ("id", Json.num 1), ("method", "initialize"),
    ("params", Json.mkObj [("token", token)])]

def g0 : GateState := (initialGateState testGateCfg).toOption.getD default
#guard initialGateState testGateCfg matches .ok _
#guard (gateStep g0 { session := "s1", payload := Json.mkObj [("method", "x")] }).2.isEmpty

def g1 : GateState := (gateStep g0 { session := "s1", payload := initMsg "tp" }).1
def afterDup : GateState × List Vmcp.Command :=
  gateStep g1 { session := "s2", payload := initMsg "tp" }
#guard g1.sessions.length = 1
#guard afterDup.1.sessions.length = 1
#guard afterDup.2 matches [Vmcp.Command.reply "s2" _]
#guard (gateStep g0 { session := "s9", payload := initMsg "bogus" }).2
  matches [Vmcp.Command.reply "s9" _]
#guard (gateStep g0 { session := "s9", payload := initMsg "bogus" }).1.sessions.isEmpty
