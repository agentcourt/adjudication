# Evidence Handling

This note documents the AAR runtime artifact layer used by `aar case`.

## Model

AAR owns record custody. It stores admitted bytes, assigns stable artifact identifiers, records provenance metadata, enforces policy limits, and logs access. Attorneys and later juror agents inspect artifacts through media-agnostic methods. AAR does not parse, render, OCR, transcribe, extract, execute, or otherwise interpret evidence formats.

`artifact_id` is the record identity for an artifact. It is deterministic from the stored SHA-256 and a normalized source name. It is not a local path, workspace path, or content-addressed storage path. Filings cite visible `artifact_id` values in `offered_artifacts`.

Local paths, workspace paths, and content-addressed storage names are implementation details. Use `artifact_id` plus SHA-256 when exact byte custody matters.

## Runtime storage

Each run writes artifact state under `--out-dir`:

```text
artifact-manifest.json
artifact-store/<sha-prefix>/<sha256>
submitted-evidence/
events.ndjson
run.json
state.json
```

`artifact-store/` is content-addressed by SHA-256. Repeated identical bytes may share the same stored object. `artifact-manifest.json` records the AAR view of each visible artifact:

- `artifact_id`
- `sha256`
- `size_bytes`
- `mime_type`
- `storage_name`
- `created_at`
- `admissibility_status`
- `record_visibility`
- optional title, original filename, provenance, parent artifact, derivation, and readability fields

Initial case materials are registered as `case_packet` artifacts. Accepted attorney submissions are registered as `submitted_artifacts` artifacts.

## Attorney methods

AAR exposes these methods over the existing ACP custom-method channel during arguments and rebuttals:

- `aar_get_case` returns the visible arbitration record.
- `aar_list_artifacts` lists visible artifact metadata. It returns metadata only, not bytes.
- `aar_stat_artifact` returns metadata, allowed operations, and remaining limits for one artifact.
- `aar_read_artifact_range` returns a bounded byte range as base64. It never mutates the record. Successful reads are logged as `artifact_read` events.
- `aar_materialize_artifact` copies exact artifact bytes into the managed attorney workspace and returns a workspace path. The returned path is not the record identity. Successful materializations are logged as `artifact_materialized` events.
- `aar_submit_artifact` submits small source evidence in one JSON request using `content` or `content_base64`.
- `aar_submit_decision` submits the legal act for the current opportunity.

## Chunked upload methods

Chunked upload is for evidence too large or unsuitable for single-request `aar_submit_artifact`.

- `aar_begin_artifact_upload` starts an upload session. It requires title, MIME type, expected size, relevance, and either source URL or source description. Nothing is admitted at this step.
- `aar_write_artifact_chunk` writes one base64 chunk at the next expected offset. Chunks must be sequential. The runtime enforces chunk and total upload limits.
- `aar_commit_artifact_upload` verifies size and SHA-256, admits the artifact through the Lean `submit_artifact` state transition, moves the uploaded bytes into `submitted-evidence/`, registers the artifact in `artifact-store/`, and returns `artifact_id`.

A failed or incomplete upload session is not evidence. A completed upload becomes record evidence only after commit succeeds and the Lean engine accepts the corresponding `submit_artifact` action.

## Policy limits

The policy has three artifact-size limits:

- `max_submitted_artifacts_bytes` is the authoritative record limit enforced by the Lean engine for each submitted artifact.
- `max_direct_submitted_artifacts_bytes` is the smaller direct JSON/base64 limit for `aar_submit_artifact`.
- `max_artifact_upload_bytes` is the chunked-upload limit. It must not exceed `max_submitted_artifacts_bytes`.

Artifact read policy:

- `max_artifact_chunk_bytes` caps each uploaded chunk.
- `max_artifact_read_bytes` caps each artifact range read.
- `max_artifact_reads_per_opportunity` caps read count per opportunity.
- `max_artifact_read_bytes_per_opportunity` caps returned artifact bytes per opportunity.

The runtime rejects invalid policies at startup. Artifact access is enforced server-side by phase; it is allowed only during arguments and rebuttals.

## Custody invariants

The implementation must preserve these invariants:

1. AAR stores exact bytes before exposing an artifact as accepted evidence.
2. `artifact_id` and SHA-256 identify record bytes. Paths do not.
3. Upload commit does not bypass the Lean `submit_artifact` transition.
4. `offered_artifacts` uses visible `artifact_id` values.
5. Artifact reads and materializations are logged.
6. AAR remains media-agnostic. Agents examine bytes with their own tools.
7. Juror-facing artifact access, when added, must be read-only and narrower than attorney access.

## Inspection checklist

After a run that uses evidence artifacts:

```bash
jq '.artifacts | length' "$out_dir/run.json"
jq '.artifact_count' "$out_dir/artifact-manifest.json"
jq '.artifacts[] | {artifact_id,sha256,size_bytes,mime_type,admissibility_status}' "$out_dir/artifact-manifest.json"
grep -n 'artifact_read\|artifact_materialized\|submitted_artifacts' "$out_dir/events.ndjson"
```

For each important exhibit, verify that:

- the `offered_artifacts` entry uses a visible `artifact_id`;
- the corresponding artifact has the expected SHA-256 and size;
- any derived artifact names its source artifact and derivation method;
- the attorney's filing distinguishes source evidence from analysis or work product.
