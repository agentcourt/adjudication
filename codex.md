# Codex Findings

## Scope

These notes record what the AAR OpenClaw lawyer tests showed about using Codex subscription credentials inside stock OpenClaw containers.  The relevant runs used `--openclaw-auth codex`, `--openclaw-codex-auth "$CODEX_AUTH_JSON"`, and `--council-pool pool.jsonl`.  The OpenClaw containers used `ghcr.io/openclaw/openclaw:latest` and `gpt-5.5`, which OpenClaw reported as the `openai-codex` provider path.

The important runs were `arb/out/ex1-openclaw-pi-20260603044948`, `arb/out/ex1-openclaw-pi-20260603051639`, `arb/out/ex2-openclaw-pi-20260603052858`, and `arb/out/ex2-openclaw-pi-20260603053222`.  The first `ex1` run exposed the embedded Codex app-server timeout.  The later `ex1` run completed after the OpenClaw config fix, while both `ex2` attempts failed at startup because Codex returned `429 Too Many Requests`.

## Authentication Path

The Codex auth path works inside stock OpenClaw containers.  `aar run` stages a copy of `auth.json` for each lawyer container, mounts that directory as `/aar-codex`, sets `CODEX_HOME=/aar-codex`, and unsets `OPENAI_API_KEY` before starting OpenClaw.  OpenClaw then uses the Codex auth profile rather than the OpenAI Platform API key path.

The staged auth directories contain bearer and refresh material, so `aar run` removes them when the run exits.  During active runs, those staged directories are visible under the run output directory as `openclaw-plaintiff-codex/auth.json` and `openclaw-defendant-codex/auth.json`.  After a clean `aar run` exit, those directories should be gone because they are secret material, not diagnostic output.

## Embedded Codex Timeout

The first `ex1` attempt, `arb/out/ex1-openclaw-pi-20260603044948`, failed before plaintiff opening completed.  The plaintiff made successful AAR MCP calls: `wait_for_opportunity`, `get_case`, `list_evidence`, and repeated `read_evidence_range` calls all returned `http_status=200 ok=true`.  The failure came from OpenClaw after those calls, with stderr reporting: `codex app-server turn idle timed out waiting for completion`.

OpenClaw stdout for that failed run reported `livenessState: "abandoned"`, `timeoutPhase: "provider"`, and `providerStarted: true`.  The elapsed duration was about 121 seconds, even though `aar run` invoked `openclaw agent --timeout 3600`.  That shows `openclaw agent --timeout` did not control the embedded Codex app-server quiet-window timeout.

OpenClaw accepts a config patch for the relevant Codex app-server fields.  The validated fields are `plugins.entries.codex.config.appServer.turnCompletionIdleTimeoutMs` and `plugins.entries.codex.config.appServer.postToolRawAssistantCompletionIdleTimeoutMs`.  `aar run` now patches those values in each OpenClaw lawyer container before it registers the MCP server and starts the lawyer agent.

The current implementation sets both OpenClaw app-server timeout fields to the effective AAR lawyer turn timeout in milliseconds.  With the default AAR lawyer turn timeout, both fields become `900000`.  The config shape was validated in the stock OpenClaw image with `openclaw config validate`.

## Successful `ex1` Run

The successful `ex1` run, `arb/out/ex1-openclaw-pi-20260603051639`, closed with `status: "ok"` and `resolution: "demonstrated"`.  The lawyers completed all filings, used AAR evidence tools to read the case-packet evidence, and sent work notes throughout the run.  The earlier OpenClaw provider quiet-window failure did not recur after the config patch.

The lawyers did not submit new evidence in `ex1`, which fits the case.  The dispute was based on private transaction documents already imported as case-packet evidence: message thread, confession, signature, public key, print approval note, invoice, distribution work order, damages breakdown, instructions, session summary, and time log.  The work notes show that the lawyers scanned the record each turn and treated outside investigation as unnecessary because the missing material was transaction-specific rather than public web evidence.

There was one invalid lawyer filing in the successful `ex1` run.  Plaintiff submitted a rebuttal over the character limit, received a rejected AAR response, sent work notes explaining the length error, and resubmitted a shorter rebuttal within the attempt budget.  This is useful evidence that malformed or invalid lawyer output can be corrected during the same turn without failing the case.

## Codex Rate Limits

Both `ex2` attempts failed before meaningful case work began.  In `arb/out/ex2-openclaw-pi-20260603052858`, the defendant OpenClaw container exited during prompt startup.  In `arb/out/ex2-openclaw-pi-20260603053222`, both lawyer containers hit the same failure path.

The OpenClaw stderr signature was consistent across the failed `ex2` attempts: `reason=rate_limit from=openai-codex/gpt-5.5`, followed by `exceeded retry limit, last status: 429 Too Many Requests`.  OpenClaw then reported a `FailoverError` because there was no fallback candidate.  AAR had only initialized the case and started lawyer sessions; the failure occurred before any useful lawyer work.

This limit is tied to the Codex/OpenClaw auth profile used by `gpt-5.5`, not to AAR's HTTP API, MCP server, Pi council code, or the OpenRouter council pool.  It also differs from OpenAI Platform API billing behavior because the OpenClaw lawyers were using the staged Codex auth file with `OPENAI_API_KEY` unset.  Immediate retries after the first `429` produced another failure rather than progress.

## Council Separation

The Pi council path remained separate from the OpenClaw Codex path.  Council members used `pool.jsonl` entries with OpenRouter model, provider, quantization, persona, and variant metadata.  The successful `ex1` run used those pool-derived configs and wrote them to `council.json`.

One council member in the successful `ex1` run exited before submitting a vote.  AAR recorded `opportunity_failed` and `council_member_removed` with `status: "failed"`, then continued to a final result with the remaining members.  That behavior is council-process failure handling, not a Codex-lawyer failure.

## Operational Consequences

The Codex auth path can run stock OpenClaw lawyer containers for AAR, but it has two distinct limits.  Long lawyer turns require OpenClaw's embedded Codex app-server timeout fields to match the AAR lawyer turn timeout.  The staged Codex auth path can also hit `openai-codex/gpt-5.5` rate limits before a lawyer makes any AAR tool call.

The timeout issue is now addressed in AAR by patching OpenClaw config inside each lawyer container.  The `429` issue is an external model-provider limit for the Codex auth profile.  When that limit is active, the correct observation is a provider startup failure, not an AAR case failure, evidence-access failure, MCP failure, or Pi council failure.

## Reference

OpenClaw documents the Codex app-server config fields in its Codex plugin reference: <https://docs.openclaw.ai/plugins/codex-harness-reference>.
