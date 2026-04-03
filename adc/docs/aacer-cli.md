# AACER CLI Guide

The AACER CLI provides read-only access to court records.  It supports two operations: list available documents for a case and fetch one document by document id.

The CLI reads from a run database that contains completed case state.  It does not create, update, or delete case records.

## Command forms

Use the `adc pacer` subcommand:

```bash
.bin/adc pacer --db out/adc-run.db
.bin/adc pacer --db out/adc-run.db --case-id CASE-0001
.bin/adc pacer --db out/adc-run.db --document-id filing-0001
```

If you want help text:

```bash
.bin/adc help pacer
```

## Flags

The CLI accepts four flags.

| Flag | Meaning | Default |
|---|---|---|
| `--db` | SQLite path | `out/adc-run.db` |
| `--case-id` | Select one case; if omitted, use the latest case with final state | empty |
| `--document-id` | Fetch one document; if omitted, list documents | empty |
| `--json` | Output mode | `true` |

With `--json=true`, output is indented JSON.  With `--json=false`, output is a one-line summary.

## What “latest case” means

If `--case-id` is omitted, AACER scans runs in reverse completion order and selects the first run that has a final state with a case object.  If `--case-id` is provided, AACER selects the newest run whose final-state case id matches.

## Output model

A list operation returns:

- `case`: summary metadata (`run_id`, `case_id`, caption, judge, status, trial mode, phase, and run metadata)
- `documents`: array of document objects

A fetch operation returns:

- `case`: the same case summary metadata
- `document`: one document object

Document objects include:

- `document_id`
- `source`
- `title`
- `document_type`
- optional fields such as `filed_at`, `description`, `body`, and `metadata`

## Document ids

AACER document ids are deterministic within each listed case.

- Docket entries use `docket-0001`, `docket-0002`, and so on.
- Filing documents use `filing-0001`, `filing-0002`, and so on.

To fetch a document, first list documents and copy the `document_id` value you need.

## Typical workflow

1. Run a case flow that writes a SQLite run database.
2. List documents with `adc pacer`.
3. Select a `document_id`.
4. Fetch that document.
5. Pipe JSON output into review or automation tools.

## Errors you will see

If the database path is wrong or unreadable, the CLI exits with the SQLite open error.

If no completed run has final state, the CLI returns: `no completed run with final state in sqlite`.

If `--case-id` does not match any final-state case, the CLI returns: `case_id "<id>" not found in sqlite final states`.

If `--document-id` is not present in the selected case, the CLI returns: `document_id "<id>" not found`.
