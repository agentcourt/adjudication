# Council Replay

Council replay runs one council member against a saved AAR output packet.  It exists for two related jobs: testing whether a council member reaches the same vote when called again with the same model request spec, and testing how another council model would decide from the same record.  The replay process starts a frozen local Council API server, starts a local MCP server, starts one Pi council container, and records the member's tool calls and vote in a separate replay output directory.

Replay never changes the source AAR output packet.  It reads the completed case record, evidence metadata, evidence bytes, policy, runtime limits, and council model config, then creates a new one-member council opportunity for the replay.  A replay result therefore measures a new model call against saved case material; it is not a cryptographic verification of the original council vote.

## Replay Bases

| Basis | Source material | Use |
| --- | --- | --- |
| `reconstructed_first_round` | Existing output packets with `run.json`, `state.json`, `policy.json`, `runtime.json`, `evidence-manifest.json`, and `evidence-store/`. | Test old completed `ex*` output packets that were produced before turn snapshots existed. |
| `snapshot` | A captured `council-turns/turn-NNNNNN-MEMBER/input.json` plus the source output packet. | Replay a future council turn from the exact saved turn state and opportunity. |

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

When running `go run` from the repository root instead of `.bin/aar` from `arb/`, pass the prompt and instruction paths explicitly:

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

`tool-calls.ndjson` gives the clearest process comparison.  A replay member that calls `read_evidence_range` examined evidence bytes through the frozen API, while a member that calls only `get_case` or `list_evidence` voted from the rendered record and metadata.  Comparing `tool-calls.ndjson`, `result.json`, and the original `run.json` council vote identifies whether the replay changed the outcome, changed the path to the same outcome, or failed before vote submission.

## Limits

Replay calls the model again, so nondeterminism and provider changes can change the vote.  Same-spec replay compares a new call against the original call; it does not prove that the original member would always vote the same way.  The comparison should record the source run id, member id, model, original vote, replay vote, replay status, and replay output directory.

`reconstructed_first_round` is a reconstruction from final output files.  It fits old completed outputs, including the existing `ex*` results, but it cannot recover transient prompt state that was not written before the original council turn.  `snapshot` should be preferred for new runs because AAR writes the exact turn input before the member starts.
