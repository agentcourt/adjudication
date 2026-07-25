import Vmcp

open Lean Vmcp

/-
The shell: the only I/O code.  It reads envelope lines from stdin, runs
the pure gate step, and executes the returned commands.  Envelope form:
{"session": "s1", "payload": <JSON-RPC message>}.  Replies and
notifications print to stdout in the same envelope form.  Accepted
actions append to the log before their effects print.

Usage:
  vmcp serve --config CONFIG.json --log LOG.ndjson
  vmcp verify --config CONFIG.json --log LOG.ndjson --state STATE.json
-/

def envelope (session : String) (payload : Json) : Json :=
  Json.mkObj [("session", session), ("payload", payload)]

/-- Replies must flush: stdout to a file or pipe is fully buffered, and
a buffered reply deadlocks a live client that waits for it. -/
def printFlushed (j : Json) : IO Unit := do
  IO.println j.compress
  (← IO.getStdout).flush

def runCommand (logPath : String) (statePath : String) (g : GateState) : Vmcp.Command → IO Unit
  | .reply session payload =>
      printFlushed (envelope session payload)
  | .notify session payload =>
      printFlushed (envelope session payload)
  | .appendLog record => do
      IO.FS.withFile logPath IO.FS.Mode.append fun h => do
        h.putStrLn record.compress
        h.flush
      IO.FS.writeFile statePath ((ToJson.toJson g.engine).pretty ++ "\n")

partial def serveLoop (logPath statePath : String) (stdin : IO.FS.Stream) : GateState → IO Unit :=
  fun g => do
    let line ← stdin.getLine
    if line.isEmpty then
      pure ()
    else
      let trimmed := line.trimAscii.toString
      if trimmed = "" then
        serveLoop logPath statePath stdin g
      else
        match Json.parse trimmed with
        | .error err => do
            IO.eprintln s!"input is not JSON: {err}"
            serveLoop logPath statePath stdin g
        | .ok j =>
            let session :=
              match j.getObjVal? "session" with
              | .ok v => (v.getStr?).toOption.getD ""
              | .error _ => ""
            let payload :=
              match j.getObjVal? "payload" with
              | .ok v => v
              | .error _ => Json.null
            if session = "" then do
              IO.eprintln "envelope needs a session"
              serveLoop logPath statePath stdin g
            else do
              let (g1, commands) := gateStep g { session := session, payload := payload }
              -- The log append runs before replies and notifications print.
              for c in commands do
                match c with
                | .appendLog _ => runCommand logPath statePath g1 c
                | _ => pure ()
              for c in commands do
                match c with
                | .appendLog _ => pure ()
                | _ => runCommand logPath statePath g1 c
              serveLoop logPath statePath stdin g1

def readConfig (path : String) : IO GateConfig := do
  let text ← IO.FS.readFile path
  match Json.parse text >>= fromJson? with
  | .error err => throw <| IO.userError s!"config {path}: {err}"
  | .ok cfg => pure cfg

def parseLogActions (text : String) : Except String (List Action) := do
  let lines := text.splitOn "\n" |>.map (fun l => l.trimAscii.toString) |>.filter (fun l => l != "")
  lines.mapM (fun line => do
    let j ← Json.parse line
    j.getObjValAs? Action "action")

def flagValue (args : List String) (flag : String) : Option String :=
  match args with
  | [] => none
  | f :: v :: rest => if f = flag then some v else flagValue (v :: rest) flag
  | [_] => none

def serve (args : List String) : IO UInt32 := do
  let some configPath := flagValue args "--config"
    | IO.eprintln "serve needs --config"; return 1
  let some logPath := flagValue args "--log"
    | IO.eprintln "serve needs --log"; return 1
  let statePath := (flagValue args "--state").getD (logPath ++ ".state.json")
  let cfg ← readConfig configPath
  match initialGateState cfg with
  | .error err => IO.eprintln s!"config: {err}"; return 1
  | .ok g0 => do
      -- Recover from an existing log by replaying it through the engine.
      let logText ← try IO.FS.readFile logPath catch _ => pure ""
      match parseLogActions logText with
      | .error err => IO.eprintln s!"log {logPath}: {err}"; return 1
      | .ok actions =>
          match replay cfg.init actions with
          | .error err => IO.eprintln s!"log replay: {err}"; return 1
          | .ok engine => do
              let g := { g0 with engine := engine }
              IO.eprintln s!"vmcp case {engine.case_id} at state_version {engine.state_version}"
              serveLoop logPath statePath (← IO.getStdin) g
              return 0

def verify (args : List String) : IO UInt32 := do
  let some configPath := flagValue args "--config"
    | IO.eprintln "verify needs --config"; return 1
  let some logPath := flagValue args "--log"
    | IO.eprintln "verify needs --log"; return 1
  let some statePath := flagValue args "--state"
    | IO.eprintln "verify needs --state"; return 1
  let cfg ← readConfig configPath
  let logText ← IO.FS.readFile logPath
  let stateText ← IO.FS.readFile statePath
  let checked : Except String Unit := do
    let actions ← parseLogActions logText
    let claimed : CaseState ← Json.parse stateText >>= fromJson?
    checkCertificate cfg.init actions claimed
  match checked with
  | .ok () => IO.println "ok"; return 0
  | .error err => IO.println s!"verification failed: {err}"; return 1

def main (args : List String) : IO UInt32 := do
  match args with
  | "serve" :: rest => serve rest
  | "verify" :: rest => verify rest
  | _ => do
      IO.eprintln "usage: vmcp serve --config C --log L [--state S] | vmcp verify --config C --log L --state S"
      return 1
