# Development Notes

## 2026-07-13: Terminal state artifact

### References

- Runner output writer: [`runtime/runner/io.go`](runtime/runner/io.go)
- Replay certificate implementation: [`runtime/runner/certificate.go`](runtime/runner/certificate.go)
- Certificate verifier command: [`runtime/cli/verify_certificate.go`](runtime/cli/verify_certificate.go)
- Exhibit action boundary: [`engine/Main.lean`](engine/Main.lean)
- Service artifact route: [`runtime/service/service.go`](runtime/service/service.go)
- ADC manual output section: [`manual.md`](manual.md#output-artifacts)
- Certificate plan: [`../plan-2026-07-13-certificates.md`](../plan-2026-07-13-certificates.md)

### Decisions

ADC terminal packets now include `state.json` beside `run.json`.  The runner writes `state.json` from the same `Result.FinalState` value embedded in `run.json`, so certificate verification has a stable file boundary without reconstructing state from the result envelope.  The service artifact API lists and fetches `state.json` through the same allowlist path used for `run.json`, logs, transcripts, and manifests.

`offer_case_file_as_exhibit` now sends `file_id` and `offered_at` into the Lean `offer_exhibit` action.  Lean validates the file id when present and records the corresponding `file_events` entry.  The runner no longer appends that event after the engine returns, so the terminal state is reproducible from accepted engine transitions.

ADC terminal packets now include `certificate.json`.  The certificate stores the initial state, optional seeded complaint initialization, accepted `step` transitions, accepted pass decisions, the claimed final state, and a compact JSON SHA-256 hash of that final state.  `adc verify-certificate` reads `certificate.json` and `state.json`, replays the recorded transitions through the Lean engine, and requires the certificate hash, packet-state hash, and replayed-state hash to match.

### Verification

- [x] `go test ./adc/runtime/runner ./adc/runtime/service`
- [x] `go test ./adc/runtime/runner ./adc/runtime/service ./adc/runtime/cli`
- [x] `go test ./adc/runtime/...`
- [x] `lake build Proofs.RecentExhibitLimits`
- [x] `lake build Proofs adcengine`
- [x] Real no-LLM smoke run with `adc scenario`, followed by `adc verify-certificate`

## 2026-07-10: Service process record reconciliation

### References

- Service process manager: [`runtime/service/service.go`](runtime/service/service.go)
- Service process tests: [`runtime/service/service_test.go`](runtime/service/service_test.go)

### Decisions

The service now gives child processes direct stdout and stderr log file descriptors instead of copying pipe output through the service process.  This lets the child continue writing logs if the service process exits, and it removes pipe-copy goroutines from the lifecycle path.  Completion still reads the stdout log after the child exits to populate the service summary.

Startup record loading now persists repaired case records.  Active or previously detached records are repaired from `run.json` when it appears; otherwise active records become failed with `service restarted and child process is not attached`.  The service does not reattach to a process after restart.

### Service API error cleanup

Artifact reads now distinguish names outside the allowlist from listed artifacts whose files are absent.  The first returns `unknown_artifact`, and the second returns `artifact_missing`.  Missing listed-artifact responses omit host filesystem paths.

### Verification

- [x] `go test ./adc/runtime/service`
- [x] `go test ./arb/runtime/... ./arbd/runtime/... ./adc/runtime/...`

## 2026-07-09: Live evidence manifest for service evidence fetch

### References

- Service evidence route: [`runtime/service/service.go`](runtime/service/service.go)
- Runner evidence output: [`runtime/runner/io.go`](runtime/runner/io.go)
- ADC manual service section: [`manual.md`](manual.md#service-api)

### Decisions

The ADC service evidence route now distinguishes an active missing manifest from a terminal packet that lacks a manifest.  Active cases return HTTP `409` with error code `evidence_manifest_pending`; terminal packets return HTTP `404` with error code `manifest_missing`; unreadable and malformed manifests return separate server errors.  The route accepts both the legacy array manifest and the object manifest with an `evidence` array.

The runner now writes `evidence-manifest.json` when it initializes and after case-file state changes.  The manifest uses an empty array when no files exist, avoiding the `null` shape that made service parsing fail in the ARB live test.  Case files are copied into `submitted-evidence/` with atomic replacement, so the service route can fetch a file by `evidence_id` during a live run after the corresponding manifest update.

### Verification

- [x] `go test ./runtime/runner`
- [x] `go test ./runtime/service`

## 2026-06-17: Manual review

### References

- ADC manual: [`manual.md`](manual.md)
- ADC Docker runbook: [`Dockerfile.md`](Dockerfile.md)
- ADC attested dev-host requirements: [`docs/attested-dev-host.md`](docs/attested-dev-host.md)

### Decisions

The manual distinguishes `adc case`, `adc scenario`, `adc run`, and `adc service` by the boundary under test.  It also records how to treat an output directory as the record for one case, with `events.ndjson`, `run.json`, `digest.md`, and `work-notes.ndjson` serving different review needs.  The command-selection table includes `adc case-packet`, because attested complaint runs depend on that deterministic input format.

The manual documents utility commands for `adc case-packet`, `adc validate`, `adc pool`, `adc pacer`, and `adc llm`.  The troubleshooting section names complaint setup and case-packet failures as input-resolution problems before runtime settings should be changed.  Attested Clerk troubleshooting points to the ADC Docker runbook, which contains the service-run and verification sequence.

### README review

The README uses the same documentation table as AAR, AARD, and evals.  It links to the manual, Docker runbook, dev-host requirements, attested run helper, practice guide, and rules.  The license references point to the repository-level files.

## 2026-06-17: Attested ADC complaint path

### References

- ADC Docker runbook: [`Dockerfile.md`](Dockerfile.md)
- ADC dev-host requirements: [`docs/attested-dev-host.md`](docs/attested-dev-host.md)
- Attested ADC driver: [`tools/run-adc-attested.py`](tools/run-adc-attested.py)
- Exec workload script: [`tools/run-adc.sh`](tools/run-adc.sh)
- Exec container entrypoint: [`attest/exec-container-entrypoint.sh`](attest/exec-container-entrypoint.sh)
- Clerk service attestation support: [`runtime/service/attested.go`](runtime/service/attested.go)
- Complaint packet builder: [`runtime/casepacket/case_packet.go`](runtime/casepacket/case_packet.go)

### Decisions

The first ADC attested path supports only `complaint_path`.  The driver packages the complaint and linked local files into `case.tar.gz` and `case-packet.json`, uploads those objects under `INPUT_PREFIX`, and requires verification before the clerk service can mark the case completed.  Scenario input and local runtime overrides remain rejected until each field has an explicit attested interpretation.

ADC uses its own base image and glue image.  The base image builds `.bin/adc`, embeds a Pi juror root filesystem, and includes the Docker CLI.  The glue image adds AWS CLI, `nitro-tpm-attest`, TSS libraries, and the ADC exec entrypoint.

The clerk service uses `execution.mode = "attested"` with an `execution.attestation` object.  It starts the Python driver, marks the service record running after the driver starts, exposes `/attestation/events`, and reads results, artifacts, and evidence from the extracted `adc-output/` directory after verification.  The service treats a zero driver exit without `verification.log` and readable `adc-output/run.json` as a failed service record.

ADC local OpenClaw Codex auth staging now matches the working AAR and AARD behavior.  The staged Codex home is mode `0777`, and `auth.json` is mode `0666`, because OpenClaw reads the mounted file as a container user that may not match the host user.  The attested run still excludes those staged auth directories from uploaded ADC archives.

ADC now also matches the AAR and AARD OpenClaw networking fix.  In the exec AMI topology, a child OpenClaw container using Docker bridge networking can import Codex auth and still fail a Codex request with `stream disconnected before completion` from `https://chatgpt.com/backend-api/codex/responses`.  The recorded AAR diagnosis showed the same failure on the same Docker-enabled exec AMI and showed that `--network host` fixed it.  ADC therefore supports `--openclaw-network host`, uses `127.0.0.1` as the Docker MCP host in that mode, omits the bridge-only `host.docker.internal` mapping, and passes `--openclaw-network host` from the attested exec entrypoint.

### Clerk test failure

The real Clerk attestation test `case_id=adc-real-ex1-20260617T040521Z` and `run_id=run-adc-real-ex1-20260617T040521Z` used exec instance `i-0966af392feb5b657` and S3 output prefix `s3://agentcourt-data/arbattest/adc-runs/run-adc-real-ex1-20260617T040521Z/`.  The run proved that the ADC auth and networking fixes were present in the running image: both OpenClaw lawyer containers started with host networking, imported Codex auth, filed pleadings, completed discovery, completed voir dire, tried the case, reached jury instructions, and entered deliberation.  The last S3 event record showed the third juror deliberating; the run then failed without `run.log`, `adc-partial.tar.gz`, `manifest.json`, `manifest.sha384`, `attestation.b64`, or `adc-output.tar.gz`.

The failure came from the generic `attest/exec.sh` console-output timeout.  The ADC legal runtime had not reached a terminal error.  `adc/tools/run-adc-attested.py` passed `POLL_ATTEMPTS=1800` to `exec.sh`; `exec.sh` sleeps two seconds per poll, so the generic launcher controls the instance for about 60 minutes before it exits 1 and terminates the instance.  The ADC driver had `--timeout-seconds 10800`, but the shorter generic launcher timeout killed the still-running exec instance before ADC could produce terminal S3 artifacts or a verifiable attestation.

The ADC driver now derives the default `POLL_ATTEMPTS` value from `--timeout-seconds` and `exec.sh`'s two-second poll period, with ten minutes of headroom.  Explicit `--exec-poll-attempts` and `POLL_ATTEMPTS` values still override that default.  The Clerk service default for `--attested-exec-poll-attempts` is now zero, so the service no longer passes the obsolete 1,800-attempt value unless the operator requests that value.  A shared launcher change may still be warranted so S3 terminal artifacts control long attested AAR, AARD, and ADC runs without a separate console-output timeout.

The next Clerk test attempt used `case_id=adc-real-ex1-20260617T133247Z` and fixed the service-level poll-attempt default, but it failed before ADC started because the fresh input prefix contained only `case.tar.gz` and `case-packet.json`.  The exec entrypoint requires `auth.json` and `keys.sh` in the same prefix.  Any fresh ADC attested Clerk test must stage those two secret objects to the selected input prefix before posting the create request.

ADC now has `tools/run-one-attested-adc.sh`, matching the working AAR and AARD one-run helpers at the operator layer.  The helper takes a complaint path, stages `/home/ec2-user/arbattest-secrets/auth.json` and `/home/ec2-user/arbattest-secrets/keys.sh` into the selected `adc-inputs` prefix through `dev`, and then runs `tools/run-adc-attested.py` with verification enabled.  This fixes the repeated operator mistake where ADC used a fresh input prefix without the two secret objects even though the runtime and exec entrypoint already handled OpenClaw Codex auth like AAR and AARD.

The real Clerk-managed attested ADC run `case_id=adc-real-ex1-20260617T133710Z` and `run_id=run-adc-real-ex1-20260617T133710Z` completed successfully.  The exec instance `i-061c6a07dc811676d` terminated after the driver downloaded terminal artifacts, and S3 output prefix `s3://agentcourt-data/arbattest/adc-runs/run-adc-real-ex1-20260617T133710Z/` contains `events.ndjson`, `run.log`, `adc-output.tar.gz`, `manifest.json`, `manifest.sha384`, and `attestation.b64`.  The Clerk service marked the case `completed`, exit code `0`, with attestation status `verified`.

Verification passed for `manifest.sha384`, run id, mode, input mode, input prefix, ADC case id, output prefix, archive key, run-log hash, archive hash, archive size, container image identity, case-packet keys and hashes, attestation signature, attestation user data, PCR 4, PCR 7, and PCR 12.  The final ADC state was `status=judgment_entered`, `phase=post_verdict`, `trial_mode=jury`.  The jury returned a plaintiff verdict with damages `108000`, five votes for the verdict, and required votes `5`; juror `J6` timed out during deliberation, and ADC handled that timeout under the existing effective-threshold rule.

The ADC runbook now documents the full Clerk service path as one sequence: choose fresh ids and prefixes, stage `auth.json` and `keys.sh` into the exact S3 input prefix, start `adc service` with attested defaults, post the create request, poll the case record and `/attestation/events`, inspect terminal artifacts, verify the attestation state, and stop the service.  The runbook also records the attestation troubleshooting map for the failures already diagnosed: missing staged secrets, expired or unreadable Codex auth, missing host networking for OpenClaw, an exec console polling limit shorter than the ADC runtime timeout, recursive S3 uploads, incomplete terminal artifacts, service output-directory validation, and PCR or manifest verification failures.  The README, manual, and dev-host requirements now point operators to that runbook for the Clerk-managed attested ADC process.

### Verification

- [x] `go test ./adc/runtime/casepacket`
- [x] `go test ./adc/runtime/service`
- [x] `go test ./adc/runtime/localrun`
- [x] `go test ./adc/runtime/...`
- [x] `sh -n adc/tools/run-adc.sh adc/tools/run-container-poc.sh adc/attest/exec-container-entrypoint.sh`
- [x] `python3 -m py_compile adc/tools/run-adc-attested.py`

## 2026-06-06: ADC practice guide rewrite

### References

- Practice guide: [`docs/practice.md`](docs/practice.md)
- Operating reference: [`manual.md`](manual.md)
- Lawyer instructions: [`agent-instructions/openclaw-lawyer.md.tmpl`](agent-instructions/openclaw-lawyer.md.tmpl)
- Juror instructions: [`agent-instructions/pi-juror.md.tmpl`](agent-instructions/pi-juror.md.tmpl)

### Decisions

The ADC practice guide now describes advocacy and evidence work rather than command syntax.  The command, API, MCP, service, clerk JSON, and output-artifact details stay in the manual, and the practice guide links to that manual as the operating reference.

The guide emphasizes case-file inspection, exhibit offers, technical reports, work notes, source search, browser work, OCR, local tools, source-chain analysis, and element-by-element arguments.  It also describes the current external-agent path: OpenClaw lawyers act through MCP over the Role API, and Pi jurors receive the trial transcript, instructions, visible case view, and case-file tools at deliberation.  A separate argument-writing section connects search and extraction work to motions, trial theory, exhibit use, closings, and post-judgment motions.

## 2026-06-06: Jury configuration inputs

### References

- Jury policy and deterministic clerk action: [`engine/Main.lean`](engine/Main.lean)
- Scenario policy defaults and runtime overrides: [`runtime/runner/state_init.go`](runtime/runner/state_init.go), [`runtime/runner/runner.go`](runtime/runner/runner.go)
- Direct command flags: [`runtime/cli/case.go`](runtime/cli/case.go), [`runtime/cli/run.go`](runtime/cli/run.go), [`runtime/cli/localrun.go`](runtime/cli/localrun.go)
- Clerk create request: [`runtime/service/service.go`](runtime/service/service.go)

### Decisions

Jury size and verdict threshold are ADC case-policy values.  The scenario policy keys are `jury_juror_count`, `jury_unanimous_required`, and `jury_minimum_concurring`, and the engine uses those values when the clerk sets jury configuration at trial setup.  The default policy remains a six-person unanimous jury with minimum concurring six.

Direct case commands expose the policy through `--juror-count`, `--unanimous-required`, and `--minimum-concurring`.  Complaint-based commands write the selected values into the generated scenario.  Scenario-based commands apply the same values as startup overrides, leaving the scenario file unchanged.

The clerk service exposes the same configuration through `juror_count`, `unanimous_required`, and `minimum_concurring` in the create-request JSON.  Those fields apply to local-agent children and direct children.  The service checks simple numeric ranges before it starts a child process, and the Lean action validation remains the final rule check.

## 2026-06-06: Failed deliberating jurors

### References

- Verdict derivation: [`engine/Main.lean`](engine/Main.lean)
- Lean sample proofs: [`engine/Proofs/RecentVerdictDerivation.lean`](engine/Proofs/RecentVerdictDerivation.lean)
- Juror timeout handling: [`runtime/runner/timeouts.go`](runtime/runner/timeouts.go)
- Timeout tests: [`runtime/runner/timeouts_test.go`](runtime/runner/timeouts_test.go)

### Decisions

ADC now treats failed deliberating jurors as removed from the effective concurrence threshold.  The configured jury size and nominal concurrence rule remain in the case configuration, but verdict derivation caps the required votes at the number of sworn jurors still eligible to vote.  If no sworn jurors remain, the case records a hung jury because no verdict can be formed.

This rule matches the operational goal for agent trials.  A single Pi model failure should not turn five matching votes into a hung jury.  Disagreement among the remaining jurors can still produce additional deliberation rounds or a hung jury under the existing split-vote rules.

### Plan

- [x] Add an effective concurrence helper in the Lean engine.
- [x] Add a Lean sample proof for a five-vote verdict after one failed juror.
- [x] Update the Go timeout test to expect a verdict from the remaining jurors.

## 2026-06-05: Pi juror opportunity lifetime

### References

- Local ADC full-run process management: [`runtime/localrun/localrun.go`](runtime/localrun/localrun.go)
- Pi juror instructions: [`agent-instructions/pi-juror.md.tmpl`](agent-instructions/pi-juror.md.tmpl)
- Juror deliberation prompt: [`runtime/runner/juror_prompt.go`](runtime/runner/juror_prompt.go)

### Decisions

A Pi juror process handles one active juror opportunity and then stops.  ADC starts a fresh Pi process if the same juror later receives another opportunity.  The case process owns waiting, so juror agents do not stay alive through non-juror trial phases.

A fresh deliberation process gets the deliberation prompt, which includes the trial transcript from openings through closings, jury instructions, and guidance to inspect admitted exhibits and visible case files through MCP.  Voir dire memory is not carried into deliberation through the Pi process home.  The case record remains the source of juror status and trial evidence.

### Plan

- [x] Tie Pi juror process names and homes to the active opportunity id.
- [x] Stop juror processes whose opportunity is no longer active.
- [x] Change Pi juror instructions to stop after a successful submission.
- [x] Strengthen the deliberation prompt's trial transcript and evidence review guidance.
- [x] Run focused ADC tests and build.

## 2026-06-04: Role API and MCP path

### References

- ADC update plan: [`../scratch/adc/update-plan.md`](../scratch/adc/update-plan.md)
- Role API: [`runtime/runner/roleapi.go`](runtime/runner/roleapi.go)
- MCP adapter: [`runtime/mcp/mcp.go`](runtime/mcp/mcp.go)
- Clerk service: [`runtime/service/service.go`](runtime/service/service.go)
- Local run command: [`runtime/localrun/localrun.go`](runtime/localrun/localrun.go)

### Decisions

ADC now exposes external plaintiff, defendant, and juror opportunities through a case-owned role API.  The runner remains the case owner because it holds the Lean state, opportunity deadlines, invalid-attempt counts, case-file visibility, and final result generation.  Complaint planning and scenario construction remain internal direct-model work before the live case starts.

The ADC MCP adapter is a thin HTTP adapter.  Each MCP session binds to a case id, role id, and optional juror principal id, then forwards tool calls to the role API.  The MCP tool set is stable; each active opportunity describes which legal tools Lean permits at that point.

ADC removed the active ACP and xproxy paths.  Juror pools now use JSONL request-spec records.  Those records preserve the model request configuration needed by Pi agents, including endpoint and request parameters.

`adc run` is now the operator-level local command.  It accepts either `--complaint` or `--scenario`, starts the case API and MCP in-process, starts OpenClaw lawyers according to `--auto-lawyers`, and starts Pi jurors when a juror first appears.  The former direct scenario command is now `adc scenario`.

Entries below this section are historical development notes.  ACP and xproxy entries record the old path and do not describe the current ADC runtime.

### Plan

- [x] Record the agreed ADC update plan.
- [x] Add the role API at the Lean opportunity boundary.
- [x] Add work notes and remote case-file reads through the role API.
- [x] Add a thin MCP adapter over the role API.
- [x] Add a clerk service that starts case processes and proxies role API calls.
- [x] Remove active ACP and xproxy runtime paths.
- [x] Make `adc run` the local OpenClaw/Pi command.

## 2026-05-20: ACP role portability

### References

- ACP role runtime: `runtime/runner/acp_role.go`
- PI-home staging: `runtime/runner/pi_container_home.go`
- Agent documentation: [`docs/agents.md`](docs/agents.md)
- Porting inventory: [`../scratch/adc/update.md`](../scratch/adc/update.md)

### Decisions

`adc` now accepts remote ACP endpoints for delegated roles.  The `case` and `run` commands use `--acp-endpoint`, while `acp-role` uses `--endpoint`; each path rejects simultaneous command and endpoint configuration.  Endpoint roles keep model selection and native tool availability outside ADC, while ADC still provides `_adc/*` methods and Lean validation.

Local wrapper-backed ACP roles now use typed ADC role configuration.  The staged PI settings default model comes from the effective role model, normalized to an xproxy model identifier, while `ADC_FLASH_XPROXY_MODEL` remains a compatibility override.  The staged PI home also receives role instructions and a private `/home/user/work-product/` directory, and the runner exports that directory beside `run.json` after the run.

The ACP prompt now names current limits instead of relying only on static documentation.  It includes host methods, support-method budget, decision budget, invalid-submission budget, visible file count, and party exhibit and report counts when the role is a party.  ACP decision rejections now keep an ordered history and fail with that history after the configured invalid-attempt limit.

### Plan

- [x] Add shared TCP endpoint configuration to `case`, `run`, and `acp-role`.
- [x] Stage role model and standing instructions into local PI homes.
- [x] Create and export ACP role work product.
- [x] Add dynamic capability and limit text to ACP opportunity prompts.
- [x] Preserve ordered invalid-decision history for ACP roles.
- [x] Stop tracking generated `.sig.b64` example artifacts.

## 2026-04-03: `adc acp` wrapper staging

### References

- ACP CLI entrypoint: `runtime/cli/acp.go`
- PI-home staging path: `runtime/runner/pi_container_home.go`
- ACP role wrapper setup: `runtime/runner/acp_role.go`
- Podman ACP wrapper: [`../common/pi-container/acp-podman.sh`](../common/pi-container/acp-podman.sh)

### Decisions

- `adc acp` now stages the temporary PI home when the selected command is `acp-podman.sh` or `pi-podman.sh`.
- That staging path already existed for `acp-role`.  The direct `acp` subcommand had been defaulting to the same wrapper without setting `PI_CONTAINER_HOME_DIR`, so the wrapper exited before ACP initialization.
- When the wrapper is in use, `session/new` now uses `/home/user` instead of the host working directory.  That matches the wrapper mount and the existing `acp-role` behavior.

### Plan

- [x] Reuse the runner PI-home staging helper from `adc acp`.
- [x] Switch wrapper-backed sessions to `/home/user`.
- [x] Add a focused CLI test for wrapper staging.

## 2026-04-03: `adc acp` default wrapper path

### References

- ACP CLI entrypoint: `runtime/cli/acp.go`
- CLI helper defaults: [`runtime/cli/helpers.go`](runtime/cli/helpers.go)

### Decisions

- `adc acp` now uses the repository-root relative wrapper path `common/pi-container/acp-podman.sh` as its default ACP command.
- The prior code joined the current working directory with a path that had already been resolved, producing duplicated prefixes such as `/repo/repo/common/...`.
- The ACP command default should stay simple.  If the user is not running from the repository root, the command should fail fast and require `--command` explicitly.

### Plan

- [x] Remove the extra path join in `runtime/cli/acp.go`.
- [x] Make `defaultACPServerPath` return the relative wrapper path directly.
- [ ] Add a dedicated test if this area changes again.

## 2026-03-18: `../evals/tools/cluster-personas.py`

### References

- Local xproxy model and config parsing: `runtime/xproxy/config.go`
- Local persona record parsing and prompt text: `runtime/persona/persona.go`
- Local xproxy startup and default port behavior: `runtime/cli/xproxy.go`
- OpenAI Python SDK Responses usage: https://github.com/openai/openai-python
- OpenAI embeddings API reference: https://platform.openai.com/docs/api-reference/embeddings/create

### Decisions

- The shared model and persona corpus now lives under `../common/data/personas/`.  The checked-in genes, sampled pools, cluster assignments, and PCA rows belong to the shared corpus, not to `adc/etc/`.
- The shared tools use the current working directory for their default paths.  Run them from the repository root unless you pass explicit paths.
- The Python tool talks to xproxy at `http://127.0.0.1:$PI_CONTAINER_XPROXY_PORT/v1`, with the same default port `18459` used in the Go code.
- The tool does not try to start xproxy.  The repository has no standalone xproxy CLI.  The Go commands start it internally for their own lifetimes.  The Python script instead checks `/healthz` and fails with a precise error if xproxy is absent.
- Persona records use the same `MODEL,FILE` parsing and the same juror persona prompt text as the Go runtime.
- Completions are sampled with repeated Responses API calls.  This is the direct path exposed by the current SDK usage here.  The task's "hopefully as multiple completions for one request" clause remains aspirational.
- Embeddings use the OpenAI Python SDK directly against the embeddings API.  The default embedding model is `text-embedding-3-small`, overridable with `PERSONA_SAMPLE_EMBEDDING_MODEL`.
- Embeddings run one sampled response at a time.  That avoids provider-side max-token failures on large batch requests and keeps one bad embedding response from aborting the whole run.
- PCA runs per gene over the full set of embeddings for that gene, matching the task.  When the requested PCA dimension exceeds what the sample count permits, the reduced vectors are zero-padded to keep the requested output dimension.
- The script writes cluster rows to stdout and writes per-sample PCA rows to `../common/data/personas/personas-pca.csv` by default.  Those rows are `model,persona_file,gene,x1,...,xN,cluster_num`.
- K-means cluster count is chosen per gene by maximizing silhouette score across the fixed range `3..10`.  If scoring is impossible or degenerate, all points fall into cluster `0`.

### Plan

- [x] Record the task and sources.
- [x] Add the standalone `uv` script.
- [x] Verify syntax and basic CLI behavior.

## 2026-03-18: `adc xproxy`

### References

- Root CLI dispatch and help text: [`runtime/cli/root.go`](runtime/cli/root.go)
- Existing xproxy helpers: `runtime/cli/xproxy.go`
- xproxy server entrypoint: `runtime/xproxy/xproxy.go`

### Decisions

- The new subcommand is `adc xproxy`.
- It resolves config and port the same way the rest of the CLI does: `--config` overrides `PI_CONTAINER_XPROXY_CONFIG` and `etc/xproxy.json`; `--port` overrides `PI_CONTAINER_XPROXY_PORT`.
- It starts xproxy directly through `xproxy.StartXProxyServer`, then waits for `SIGINT` or `SIGTERM` and closes the server cleanly.
- It fails fast if the target port already serves a healthy xproxy instance.

### Plan

- [x] Add root command dispatch and help wiring.
- [x] Add the server command implementation.
- [x] Verify help text and live `/healthz` behavior in tests.

### Results

- Live test: `uv run ../evals/tools/cluster-personas.py --personas-file /tmp/persona-sample-test.csv --genes-file /tmp/persona-sample-genes.json --num-samples 3 --gene-dim 3`
- Live output: three `MP,G,C` rows for one persona and one gene through local xproxy plus direct embeddings.
- Follow-up fix: `adc xproxy` initially returned an error on clean shutdown because the listener was already closed.  `runtime/xproxy/xproxy.go` now ignores `net.ErrClosed` in that path, and a live `Ctrl-C` shutdown now exits with status `0`.
