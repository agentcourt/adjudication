# Council and Juror Replay

Replay runs one fresh Pi council-member deliberation against a saved AAR output packet.  `aar council-replay` repeats one council member with a single request-spec config, usually for same-spec comparisons.  `aar juror-replay` takes a model config and persona as separate inputs, usually for persona experiments or alternative-model comparisons.

Both commands start a frozen local Council API server, a local MCP server, and one Pi container.  The replay reads the saved case record, evidence metadata, evidence bytes, policy, runtime limits, and selected model config, then records the fresh member's tool calls and vote in a separate output directory.  The source AAR output packet stays unchanged.

## Requirements

Run replay commands from `arb/` after building `.bin/aar`.  The default prompt and instruction paths are relative to `arb/`, so commands run from the repository root must pass `--prompt-dir arb/prompts` and `--council-instructions arb/agent-instructions/pi-council.md.tmpl`.  The current Pi model configs use OpenRouter, so the command environment must contain `OPENROUTER_API_KEY`.

Replay also needs a local container command that can run the Pi image.  Use `--podman docker` when Docker runs `agentcourt-pi-sandbox:latest`; use the default `--podman podman` only when Podman has the image and can expose the host MCP server.  Check the image before running with `docker image inspect agentcourt-pi-sandbox:latest` or the matching Podman command.

## Replay Bases

| Basis | Source material | Use |
| --- | --- | --- |
| `reconstructed_first_round` | Existing output packets with `run.json`, `state.json`, `policy.json`, `runtime.json`, `evidence-manifest.json`, and `evidence-store/`. | Test old completed `ex*` output packets that were produced before turn snapshots existed. |
| `snapshot` | A captured `council-turns/turn-NNNNNN-MEMBER/input.json` plus the source output packet. | Replay a saved council turn from the exact saved turn state and opportunity. |

`reconstructed_first_round` rebuilds a first-round deliberation opportunity from the final output packet.  It starts with the final case state, restores the case to deliberation round 1, clears `council_votes`, clears the final resolution, and replaces the council with the requested single member.  This mode gives existing completed outputs a useful replay path, but it reconstructs the first-round prompt from durable output files rather than from a turn snapshot captured before the original member acted.

`snapshot` reads the saved turn input written by AAR before the council member starts.  The saved input contains the case state, opportunity, policy, runtime limits, complaint, case view, visible evidence metadata, evidence manifest, tool specs, prompt, and the original member metadata.  Replay keeps the saved original prompt in `input.json` for comparison and renders a new prompt using the supplied replay config.

## Captured Turn Snapshots

New AAR runs write a council snapshot before each council member starts.  The snapshot directory is named `council-turns/turn-%06d-%s`, where the number is the AAR turn number and the suffix is the member id.  The directory contains `input.json`, which is the structured replay input, and `prompt.txt`, which is the exact prompt generated for the original council member.

Snapshot capture belongs to the normal run packet.  A completed run therefore contains the final record and each council turn input that was presented to Pi.  Future replay work should use `snapshot` when the relevant `council-turns/` directory exists, because that basis avoids reconstructing the first council opportunity from the final state.

## Same-Spec Replay

Same-spec replay uses the same member id and request spec recorded in the original `council.json`.  The config passed to `--config` must be a single request-spec JSON object, not the full council member object.  The `persona` field must point to a readable persona file, so temporary configs should use an absolute persona path unless the config file lives next to the persona directory.

From `arb/`, create a same-spec config for member `C1`:

```bash
source="../../aar-attested/aar-ex03-20260613T210952Z/aar-output"
member=C1
arb_dir="$(pwd)"

jq --arg member "$member" --arg arb "$arb_dir" '
  .[] | select(.member_id == $member) |
  .request_spec + {
    persona: (
      if ((.persona_file // .request_spec.persona) | startswith("/"))
      then (.persona_file // .request_spec.persona)
      else $arb + "/" + (.persona_file // .request_spec.persona)
      end
    )
  }
' "$source/council.json" >"/tmp/aar-replay-$member.json"
```

Run the replay:

```bash
.bin/aar council-replay \
  --basis reconstructed_first_round \
  --source-output "$source" \
  --config "/tmp/aar-replay-$member.json" \
  --out-dir "../aar-replays/aar-ex03-$member-same" \
  --member-id "$member" \
  --podman docker \
  --pi-image agentcourt-pi-sandbox:latest
```

The command requires `OPENROUTER_API_KEY` because the current Pi council configs use OpenRouter.  It also requires a local Pi image, such as `agentcourt-pi-sandbox:latest`, and access to the local container daemon.  Use `--podman docker` when Docker runs the Pi image; keep the default `--podman podman` only when Podman can run the Pi image and expose host networking in the local environment.

## Snapshot Replay

Snapshot replay uses the saved turn input from a newer output packet.  The member id comes from the snapshot, so `--member-id` cannot override it in this mode.  The supplied config still controls the replay model and persona, which allows the same captured turn to run under the original model or a different model.

```bash
.bin/aar council-replay \
  --basis snapshot \
  --source-output out/ex03-new \
  --snapshot out/ex03-new/council-turns/turn-000009-C1 \
  --config /tmp/aar-replay-C1.json \
  --out-dir out/replays/ex03-C1-snapshot \
  --podman docker \
  --pi-image agentcourt-pi-sandbox:latest
```

## Juror Replay

`aar juror-replay` runs one fresh deliberation from a saved AAR output packet with a model config and persona chosen at command time.  The command requires `--source-output`, `--model-config`, `--persona`, and `--out-dir`.  It prints one JSON summary to stdout and writes the full replay packet under `--out-dir`.

The command chooses a replay basis from the supplied flags and source output.  An explicit `--snapshot` selects snapshot replay.  A supplied `--member-id` makes the command scan `council-turns/*/input.json` and require exactly one snapshot with that `member_id`.  If the source output has no `council-turns/` directory, the command uses `reconstructed_first_round`; if the source has multiple snapshots and no member is specified, the command fails and requires `--snapshot` or `--member-id`.

The model config file must contain one JSON request-spec record.  A member's `.request_spec` from `council.json` works directly.  A JSONL row from `pool.jsonl` also works when it contains endpoint and model fields accepted by `common/modelrequest`.

From `arb/`, build a one-record model config from an existing council member:

```bash
source="out/local-direct-three-per-ex-only-20260629/ex13/run-03"
member=C1

jq --arg member "$member" '
  .[] | select(.member_id == $member) | .request_spec
' "$source/council.json" >"/tmp/aar-juror-replay-$member-model.json"
```

Or build a model config from `pool.jsonl`:

```bash
jq -c '
  select(.openrouter_model_id == "minimax/minimax-m2.5")
  | select(.provider_name == "Minimax")
' pool.jsonl | head -n 1 >"/tmp/aar-juror-replay-pool-model.json"
```

Run the replay with an experimental persona:

```bash
.bin/aar juror-replay \
  --source-output "$source" \
  --member-id "$member" \
  --model-config "/tmp/aar-juror-replay-$member-model.json" \
  --persona "../evals/model-pool/personas/experiments/attorneys/Brandeis.txt" \
  --out-dir "out/juror-replays/ex13-run-03-$member-brandeis" \
  --podman docker \
  --pi-image agentcourt-pi-sandbox:latest
```

Run from the repository root by passing explicit prompt and instruction paths:

```bash
go run ./arb/runtime/cmd/aar juror-replay \
  --source-output arb/out/local-direct-three-per-ex-only-20260629/ex13/run-03 \
  --member-id C1 \
  --model-config /tmp/aar-juror-replay-C1-model.json \
  --persona evals/model-pool/personas/experiments/attorneys/Brandeis.txt \
  --out-dir arb/out/juror-replays/ex13-run-03-C1-brandeis \
  --prompt-dir arb/prompts \
  --council-instructions arb/agent-instructions/pi-council.md.tmpl \
  --podman docker \
  --pi-image agentcourt-pi-sandbox:latest
```

When running `go run` for `council-replay` from the repository root instead of `.bin/aar` from `arb/`, pass the same prompt and instruction paths explicitly:

```bash
go run ./arb/runtime/cmd/aar council-replay \
  --basis reconstructed_first_round \
  --source-output ../aar-attested/aar-ex03-20260613T210952Z/aar-output \
  --config /tmp/aar-replay-C1.json \
  --out-dir ../aar-replays/aar-ex03-C1-same \
  --member-id C1 \
  --prompt-dir arb/prompts \
  --council-instructions arb/agent-instructions/pi-council.md.tmpl \
  --podman docker \
  --pi-image agentcourt-pi-sandbox:latest
```

## Replay Output

Each replay output directory contains the files needed to inspect the new council turn.  `input.json` records the replay basis, source output directory, member, model, policy, runtime limits, prompt, case view, visible evidence, and evidence manifest.  `prompt.txt` contains the prompt sent to the replay member, `result.json` records the final vote or error, and `tool-calls.ndjson` records each Council API tool call in order.

`aar juror-replay` also writes `juror-replay.json`.  That metadata file records the source output directory, selected snapshot, model config path, persona path, persona SHA-256, vote, rationale, and tool-call count.  Use it as the summary record for persona experiments.

`tool-calls.ndjson` gives the clearest process comparison.  A replay member that calls `read_evidence_range` examined evidence bytes through the frozen API, while a member that calls only `get_case` or `list_evidence` voted from the rendered record and metadata.  Comparing `tool-calls.ndjson`, `result.json`, and the original `run.json` council vote identifies whether the replay changed the outcome, changed the path to the same outcome, or failed before vote submission.

```bash
jq '{status,basis,case_id,member_id,model,vote,tool_call_count,persona_path,snapshot_dir}' \
  out/juror-replays/ex13-run-03-C1-brandeis/juror-replay.json

jq -r '.tool' out/juror-replays/ex13-run-03-C1-brandeis/tool-calls.ndjson

rg -n 'Persona:|You are an attorney' out/juror-replays/ex13-run-03-C1-brandeis/prompt.txt
```

## Troubleshooting

| Message or symptom | Cause | Fix |
| --- | --- | --- |
| `OPENROUTER_API_KEY is required` | The selected Pi model config uses OpenRouter. | Export `OPENROUTER_API_KEY` before running replay. |
| `stat persona ...` or `empty persona text` | `--persona` points to a missing, directory, or empty file. | Pass the intended persona text file, usually under `../evals/model-pool/personas/experiments/` from `arb/`. |
| `source output has multiple council-turn snapshots` | The source run has more than one captured turn and no target turn was specified. | Pass `--member-id MEMBER` or `--snapshot PATH`. |
| `member MEMBER has N council-turn snapshots` | The same member has more than one captured turn. | Pass the exact `--snapshot` directory. |
| `operation not permitted` while binding `127.0.0.1:0` | The process cannot open the local replay HTTP listener in the current environment. | Run the command in a local shell with permission to bind loopback ports. |
| Pi image or container command failure | The configured container command cannot run the Pi image. | Check the local Pi image and use `--podman docker` or `--podman podman` consistently. |
| Missing prompt or instruction template | The command ran from a directory where default relative paths do not exist. | Run from `arb/`, or pass `--prompt-dir` and `--council-instructions`. |

## Limits

Replay calls the model again, so nondeterminism and provider changes can change the vote.  Same-spec replay compares a new call against the original call; it does not prove that the original member would always vote the same way.  The comparison should record the source run id, member id, model, original vote, replay vote, replay status, and replay output directory.

`reconstructed_first_round` is a reconstruction from final output files.  It fits old completed outputs, including the existing `ex*` results, but it cannot recover transient prompt state that was not written before the original council turn.  `snapshot` should be preferred for new runs because AAR writes the exact turn input before the member starts.
