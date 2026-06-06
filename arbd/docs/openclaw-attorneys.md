# OpenClaw Degree Lawyers

This note describes the `arbd` side of OpenClaw-backed plaintiff and defendant lawyers.  AARD uses the Lawyer API as its case interface and uses MCP as the adapter that OpenClaw can consume.  `aard run` starts the case process, starts `aard mcp`, starts local OpenClaw containers when requested, and gives each lawyer instructions for one role in one case.

## Local Lawyers

`aard run` is the local OpenClaw path.  It starts OpenClaw containers for the selected lawyer roles, configures the AARD MCP server inside each container, and gives each lawyer the generated instructions from `agent-instructions/openclaw-lawyer.md.tmpl`.  The lawyer then waits for opportunities, reads the current prompt and tools through MCP, submits work notes, inspects and submits evidence when appropriate, and files the required degree act before the turn deadline.

OpenClaw authentication can use either `OPENAI_API_KEY` or a Codex `auth.json` file.  The preferred local form uses `--openclaw-auth codex --openclaw-codex-auth PATH`, because that lets the container use the same OpenAI subscription credentials used by Codex.  The lawyer model and thinking settings come from the `aard run` flags unless the caller leaves them at their defaults.

```bash
.bin/aard run ex1 \
  --openclaw-auth codex \
  --openclaw-codex-auth PATH/TO/auth.json \
  --council-pool pool.jsonl
```

## Remote Lawyers

Remote lawyers use the same MCP tools, but `aard run` does not start the remote OpenClaw.  Use `--auto-lawyers plaintiff` to start only the plaintiff locally, `--auto-lawyers defendant` to start only the defendant locally, or an equivalent setting when one side belongs to an independently running OpenClaw.  Provide `--mcp-public-base-url` when the remote machine needs a URL different from the local MCP listen address.

`aard run` writes a role-specific remote lawyer instruction file from `agent-instructions/openclaw-remote-lawyer-skill.md.tmpl`.  The instruction tells the remote OpenClaw the case id, role id, MCP URL, and working procedure.  The remote lawyer should create an MCP session for that case and role, keep calling the wait tool until a turn is ready or the case is done, and use the tools returned for the current turn.

## Evidence And Work Notes

AARD owns the case record, evidence custody, filing validation, invalid-attempt feedback, transcript, and Lean state transitions.  OpenClaw owns its own model, browser or search access, local programs, and analysis work.  Source material enters the case through `submit_evidence` or chunked evidence upload before a lawyer cites the returned `evidence_id` in `offered_evidence`.

Lawyers should send work notes during every turn.  `send_work_notes` records private work notes outside the case record, so those notes do not become evidence, argument, technical reports, or case events.  The notes are for later review of search strategy, evidence analysis, local tool use, and filing decisions.

## Inspection

After a run, inspect the submitted evidence, evidence reads, work notes, filings, and final answer map.  Source material submitted through AARD evidence tools appears in `submitted-evidence/`, `evidence-store/`, `evidence-manifest.json`, `state.json`, and `digest.md`.  Work notes appear in `work-notes.ndjson`, and final degree answers appear in `run.json`, `council.json`, `digest.md`, and the final-result API response.
