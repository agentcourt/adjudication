#!/usr/bin/env -S uv run --script
# /// script
# requires-python = ">=3.11"
# dependencies = []
# ///
import argparse
import json
import random
from collections import Counter, defaultdict
from pathlib import Path
from typing import Any


ClusterTuple = tuple[int, ...]


def load_jsonl(path: Path) -> list[dict[str, Any]]:
    rows: list[dict[str, Any]] = []
    with path.open() as handle:
        for line_num, line in enumerate(handle, start=1):
            line = line.strip()
            if not line:
                continue
            try:
                row = json.loads(line)
            except json.JSONDecodeError as exc:
                raise RuntimeError(f"{path}:{line_num}: invalid JSON") from exc
            if not isinstance(row, dict):
                raise RuntimeError(f"{path}:{line_num}: expected JSON object")
            row["_source_row"] = len(rows) + 1
            rows.append(row)
    if not rows:
        raise RuntimeError(f"{path}: no rows found")
    return rows


def validate_rows(rows: list[dict[str, Any]]) -> int:
    cluster_count: int | None = None
    for index, row in enumerate(rows, start=1):
        clusters = row.get("clusters")
        if not isinstance(clusters, list) or not clusters:
            raise RuntimeError(f"row {index}: clusters must be a non-empty array")
        if not all(isinstance(value, int) for value in clusters):
            raise RuntimeError(f"row {index}: clusters must contain only integers")
        if cluster_count is None:
            cluster_count = len(clusters)
        elif len(clusters) != cluster_count:
            raise RuntimeError(f"row {index}: clusters length {len(clusters)} differs from expected {cluster_count}")
    assert cluster_count is not None
    return cluster_count


def group_by_tuple(rows: list[dict[str, Any]]) -> dict[ClusterTuple, list[dict[str, Any]]]:
    grouped: dict[ClusterTuple, list[dict[str, Any]]] = defaultdict(list)
    for row in rows:
        grouped[tuple(row["clusters"])].append(row)
    return dict(grouped)


def clean_row(row: dict[str, Any]) -> dict[str, Any]:
    return {key: value for key, value in row.items() if key != "_source_row"}


def endpoint_key(row: dict[str, Any]) -> str:
    endpoint_variant_id = row.get("endpoint_variant_id")
    if endpoint_variant_id:
        return str(endpoint_variant_id)
    return "|".join(
        str(row.get(key, ""))
        for key in ("openrouter_model_id", "provider_name", "endpoint_tag", "quantization")
    )


def main() -> int:
    parser = argparse.ArgumentParser(description="Sample rows by uniformly selected cluster-assignment tuples.")
    parser.add_argument("input", type=Path, help="Input variant-persona cluster JSONL file")
    parser.add_argument("--out", required=True, type=Path, help="Output sampled JSONL file")
    parser.add_argument("--diagnostics-out", type=Path, help="Optional JSONL diagnostics file")
    parser.add_argument("--pool-size", type=int, default=20, help="Number of rows to emit. Default: %(default)s")
    parser.add_argument("--seed", type=int, help="Optional deterministic random seed")
    args = parser.parse_args()

    if args.pool_size <= 0:
        raise RuntimeError("--pool-size must be positive")

    rows = load_jsonl(args.input)
    cluster_count = validate_rows(rows)
    grouped = group_by_tuple(rows)
    tuples = sorted(grouped)
    rng = random.Random(args.seed)

    tuple_counts: Counter[ClusterTuple] = Counter()
    row_counts: Counter[int] = Counter()
    model_counts: Counter[str] = Counter()
    provider_counts: Counter[str] = Counter()
    endpoint_counts: Counter[str] = Counter()

    args.out.parent.mkdir(parents=True, exist_ok=True)
    diagnostics_handle = None
    if args.diagnostics_out:
        args.diagnostics_out.parent.mkdir(parents=True, exist_ok=True)
        diagnostics_handle = args.diagnostics_out.open("w")

    try:
        with args.out.open("w") as output_handle:
            for step in range(1, args.pool_size + 1):
                cluster_tuple = tuples[rng.randrange(len(tuples))]
                candidates = grouped[cluster_tuple]
                row = candidates[rng.randrange(len(candidates))]

                tuple_counts[cluster_tuple] += 1
                row_counts[row["_source_row"]] += 1
                model_id = str(row.get("openrouter_model_id", ""))
                provider_name = str(row.get("provider_name", ""))
                endpoint = endpoint_key(row)
                model_counts[model_id] += 1
                provider_counts[provider_name] += 1
                endpoint_counts[endpoint] += 1

                print(
                    f"{step}: tuple={list(cluster_tuple)} tuple_size={len(candidates)} "
                    f"tuple_count={tuple_counts[cluster_tuple]} row={row['_source_row']} "
                    f"row_count={row_counts[row['_source_row']]} model={model_id} "
                    f"provider={provider_name} endpoint={row.get('endpoint_tag', '')} "
                    f"quantization={row.get('quantization', '')}"
                )
                output_handle.write(json.dumps(clean_row(row), ensure_ascii=False) + "\n")

                if diagnostics_handle:
                    diagnostics_handle.write(json.dumps({
                        "step": step,
                        "cluster_tuple": list(cluster_tuple),
                        "cluster_tuple_count": tuple_counts[cluster_tuple],
                        "cluster_tuple_size": len(candidates),
                        "source_row": row["_source_row"],
                        "source_row_count": row_counts[row["_source_row"]],
                        "openrouter_model_id": model_id,
                        "provider_name": provider_name,
                        "endpoint_tag": row.get("endpoint_tag"),
                        "quantization": row.get("quantization"),
                        "endpoint_identifier": endpoint,
                    }, ensure_ascii=False, sort_keys=True) + "\n")
    finally:
        if diagnostics_handle:
            diagnostics_handle.close()

    print(
        f"summary: input_rows={len(rows)} gene_count={cluster_count} "
        f"unique_tuples={len(tuples)} emitted={args.pool_size} "
        f"emitted_unique_tuples={len(tuple_counts)} unique_rows={len(row_counts)} "
        f"unique_models={len(model_counts)} unique_providers={len(provider_counts)} "
        f"unique_endpoints={len(endpoint_counts)}"
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
