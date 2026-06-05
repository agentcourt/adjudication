# Consistency Review

## Scope

This review compares `adc/`, `arb/`, and `arbd/` for the current external-agent capabilities: `auth.json` OpenClaw lawyers, HTTP role APIs, MCP adapters, Pi juror or council agents using full model-pool request specifications, case-management services, output limits, and operator documentation.  The review covered runtime source, MCP source, service source, local-agent templates, and manuals.  After the first review, the code was updated where the inconsistency was accidental or unsafe.

`arb/` and `arbd/` remain close mirrors.  `adc/` implements the same broad system, but jury lifecycle, voir dire, generated case setup, and juror context require procedure-specific state.  The remaining differences are documented below as intentional procedure differences rather than open cleanup items.

## Resolution Summary

| Area | Status | Result |
|---|---|---|
| ADC observer authority | Fixed | ADC Role API mutating calls now require the active actor, not observer visibility.  The ADC MCP adapter also rejects observer calls for tools outside the observer list before forwarding to the Role API. |
| OpenClaw Codex auth | Aligned | AAR and AARD now match ADC by importing the staged Codex access token into OpenClaw with `openclaw models auth paste-token --provider openai --profile-id openai:codex`. |
| Clerk request JSON | Aligned | ADC Clerk JSON now uses `docker_command`, `podman_command`, and `openclaw_codex_auth_path`, matching AAR and AARD.  ADC start delay now uses a nullable field so an explicit zero can pass through. |
| Clerk API completeness | Aligned | AAR and AARD Clerk APIs now expose inspect, result, artifact listing, artifact download, evidence download, list, create, and kill routes. |
| Direct service behavior | Cleaned and documented | AAR and AARD direct `/api/v1/cases` paths still start direct case processes and keep separate registry records.  That path serves HTTP-driven cases without local OpenClaw and Pi startup.  Direct `out_dir` now must be an immediate child of the service output root, matching the Clerk safety rule. |
| MCP tool sets | Cleaned | ADC non-observer MCP sessions now list `get_case_result`, which the handler already supported.  ADC keeps `report_failure` for external juror/lawyer failure reporting.  AAR and AARD do not add a lawyer MCP failure tool because council-member failures are handled by local process supervision and the direct Council API, not by lawyer sessions. |
| ADC local lawyer evidence guidance | Aligned | ADC local OpenClaw lawyer instructions now include the broader evidence-search and analysis guidance already present in ADC remote-lawyer instructions and AAR/AARD local lawyer instructions. |
| Pi max-output-token defaults | Cleaned | ADC now exposes `DefaultJurorMaxOutputTokens` through runtime limits and uses that value when writing Pi model config.  The localrun-only `defaultPiMaxOutputTokens` constant was removed. |
| Default ports | Cleaned | AARD service and MCP defaults moved to `127.0.0.1:19790` and `127.0.0.1:19800`.  AAR remains on `19770` and `19780`; ADC remains on `19870` and `19880`. |
| Work-note side effects | Fixed | ADC `send_work_notes` no longer appends a turn transcript entry.  Work-note text and note-submission metadata stay in `work-notes.ndjson`, outside the case record. |
| AAR manual auth cleanup | Fixed | The AAR manual now says staged Codex homes are removed during normal cleanup and notes the interruption case.  AARD and ADC manuals also describe the OpenClaw token import step. |
| Artifact route scope | Fixed | ADC, AAR, and AARD artifact routes now serve only exact listed artifact names.  They do not serve logs, generated remote-lawyer instructions, staged Codex auth directories, or other arbitrary files in a run output directory. |

## Intentional Differences

ADC starts jurors when a juror first appears, while AAR and AARD start council agents after the council roster becomes available.  The procedures require that difference.  AAR and AARD have a fixed council roster after argument closes; ADC can generate juror state through court procedure and voir dire.

ADC uses plaintiff, defendant, observer, and juror roles, plus a juror `principal_id`.  AAR and AARD use plaintiff, defendant, observer, and council `member_id`.  Common HTTP and MCP clients still need procedure-specific handling for the juror or council identity field.

ADC case setup includes generated complaints, voir dire, judge and clerk internals, juror questionnaires, and case-file visibility rules.  AAR and AARD start from complaint or example material and use evidence tools tied to arbitration phases.  Those differences explain the ADC-specific support tools such as `list_case_files`, `read_case_text_file`, `request_case_file`, `read_case_file_bytes`, `explain_decisions`, and `get_juror_context`.

The AAR and AARD direct service API remains separate from the Clerk API.  Clerk starts full local runs with local OpenClaw lawyers and Pi council agents.  The direct service API starts `aar case` or `aard case` children for clients that will drive lawyer and council APIs themselves; it keeps `/cancel` for those direct cases and uses `--registry-dir` for direct-case records.

## Verification

`go test ./adc/... ./arb/... ./arbd/...` passes.  The tests now cover ADC observer read-only enforcement, ADC work-note transcript isolation, ADC MCP `get_case_result` listing, AAR/AARD Codex auth token import command generation, AAR/AARD direct `out_dir` rejection, `.` and `..` case-id rejection, exact artifact-name allowlists, and the new AAR/AARD Clerk result/artifact/evidence routes.  The remaining manual differences match procedure-specific behavior rather than accidental drift.
