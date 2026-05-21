#!/usr/bin/env -S uv run --script
# /// script
# requires-python = ">=3.11"
# dependencies = []
# ///
"""Generate a council CSV from OpenRouter metadata, probes, personas, clustering, and PCA."""

from __future__ import annotations

import argparse
import csv
import os
import random
import shutil
import subprocess
import sys
import tempfile
import time
from pathlib import Path
from urllib.error import URLError
from urllib.request import urlopen

OPENROUTER_MODELS_URL = "https://openrouter.ai/api/v1/models"
DEFAULT_XPROXY_PORT = 18459
SECONDS_PER_DAY = 86400.0


def parse_args(argv: list[str]) -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--repo-root", type=Path, default=Path.cwd(), help="Repository root. Default: current directory")
    parser.add_argument("--metadata-url", default=OPENROUTER_MODELS_URL, help="OpenRouter metadata URL")
    parser.add_argument("--common-etc", default="common/etc", help="common/etc directory")
    parser.add_argument("--persona-dir", default="common/etc/personas/persons", help="Directory containing persona text files")
    parser.add_argument("--probe-persona", default="common/etc/personas/persons/d715074-0.txt", help="Persona file used for the tool/latency probe")
    parser.add_argument("--metadata", default="common/data/personas/openrouter-models.json", help="OpenRouter metadata JSON output/input")
    parser.add_argument("--models-prefiltered", default="common/data/personas/models-prefiltered.csv", help="Metadata-prefiltered model list")
    parser.add_argument("--filter-decisions", default="common/data/personas/model-filter-decisions.csv", help="Metadata-filter audit CSV")
    parser.add_argument("--model-latency", default="common/data/personas/model-latency.csv", help="MODEL,ELAPSED_MS,TOOLS_SUPPORTED CSV")
    parser.add_argument("--models", default="common/data/personas/models.csv", help="Retained model list after latency/tool filtering")
    parser.add_argument("--source-pool", default="common/etc/personas.csv", help="Model/persona cross-product CSV")
    parser.add_argument("--cluster-input", default="common/data/personas/cluster-input.csv", help="Sampled model/persona input for clustering")
    parser.add_argument("--genes", default="common/data/personas/genes.json", help="Gene prompt JSON file")
    parser.add_argument("--clusters", default="common/data/personas/clusters.csv", help="Cluster assignment CSV output/input")
    parser.add_argument("--pca", default="common/data/personas/pca-cluster.csv", help="PCA cluster CSV output/input")
    parser.add_argument("--failures", default="common/data/personas/model-operational-failures.csv", help="Operational failure ledger CSV")
    parser.add_argument("--council", default="common/data/personas/council.csv", help="Selected council CSV output")
    parser.add_argument("--report", default="", help="Council selection report output. Default: a temporary file outside the repository")
    parser.add_argument("--model-speed-log", default="common/data/personas/model-latency.log", help="stderr log for model-speed.sh")
    parser.add_argument("--cluster-log", default="common/data/personas/clusters.log", help="stderr log for cluster-personas.py")
    parser.add_argument("--min-context", type=int, default=200000, help="Minimum effective context for selected council candidates")
    parser.add_argument("--max-elapsed-ms", type=int, default=8000, help="Maximum model-latency elapsed time. Use 0 to disable")
    parser.add_argument("--size", type=int, default=20, help="Number of council rows to select")
    parser.add_argument("--sample-size", default="512", help="Rows to sample from source pool for clustering, or 'all'")
    parser.add_argument("--sample-seed", type=int, default=0, help="Deterministic sampling seed for cluster input")
    parser.add_argument("--num-samples", type=int, default=3, help="cluster-personas completions per model/persona/gene")
    parser.add_argument("--num-genes", default="3", help="cluster-personas genes to sample, or 'all'")
    parser.add_argument("--num-personas", default="all", help="cluster-personas rows to sample from --cluster-input, or 'all'")
    parser.add_argument("--gene-dim", type=int, default=3, help="PCA dimensions per gene")
    parser.add_argument("--expires", type=float, default=7.0, help="Reuse intermediate files younger than this many days. Default: %(default)s. Use 0 to regenerate all intermediates")
    parser.add_argument("--xproxy-port", type=int, default=0, help="xproxy port for health checks. Default uses PI_CONTAINER_XPROXY_PORT or 18459")
    parser.add_argument("--use-existing-metadata", action="store_true", help="Do not fetch OpenRouter metadata")
    parser.add_argument("--use-existing-latency", action="store_true", help="Do not run model-speed.sh")
    parser.add_argument("--use-existing-clusters", action="store_true", help="Do not run cluster-personas.py")
    parser.add_argument("--no-xproxy-check", action="store_true", help="Skip xproxy health check before live probe/clustering")
    parser.add_argument("--dry-run", action="store_true", help="Print planned actions without changing files")
    return parser.parse_args(argv)


def rel_path(root: Path, value: str | Path) -> Path:
    path = Path(value)
    if path.is_absolute():
        return path
    return root / path


def display(root: Path, path: Path) -> str:
    try:
        return path.relative_to(root).as_posix()
    except ValueError:
        return path.as_posix()


def ensure_parent(path: Path, dry_run: bool) -> None:
    if dry_run:
        return
    path.parent.mkdir(parents=True, exist_ok=True)


def expires_seconds(days: float) -> float:
    if days < 0:
        raise SystemExit("--expires must be non-negative")
    return days * SECONDS_PER_DAY


def is_fresh(path: Path, now: float, expires: float) -> bool:
    if expires <= 0 or not path.is_file():
        return False
    return now - path.stat().st_mtime <= expires


def fresh_all(paths: list[Path], now: float, expires: float) -> bool:
    return bool(paths) and all(is_fresh(path, now, expires) for path in paths)


def fresh_some(paths: list[Path], now: float, expires: float) -> bool:
    return any(is_fresh(path, now, expires) for path in paths)


def describe_age(path: Path, now: float) -> str:
    if not path.exists():
        return "missing"
    age_seconds = max(0.0, now - path.stat().st_mtime)
    return f"{age_seconds / SECONDS_PER_DAY:.2f} days old"


def use_fresh(label: str, paths: list[Path], root: Path, now: float) -> None:
    rendered = ", ".join(f"{display(root, path)} ({describe_age(path, now)})" for path in paths)
    print(f"use fresh {label}: {rendered}")


def stop_on_partial_fresh(label: str, paths: list[Path], root: Path, now: float, expires: float) -> None:
    if fresh_some(paths, now, expires) and not fresh_all(paths, now, expires):
        rendered = ", ".join(f"{display(root, path)} ({describe_age(path, now)})" for path in paths)
        raise SystemExit(f"partial fresh {label} outputs; refusing to mix or overwrite: {rendered}")


def run_command(cmd: list[str], *, cwd: Path, dry_run: bool, stdin_path: Path | None = None, stdout_path: Path | None = None, stderr_path: Path | None = None) -> None:
    rendered = " ".join(cmd)
    if stdin_path is not None:
        rendered += f" < {display(cwd, stdin_path)}"
    if stdout_path is not None:
        rendered += f" > {display(cwd, stdout_path)}"
    if stderr_path is not None:
        rendered += f" 2> {display(cwd, stderr_path)}"
    print(f"$ {rendered}")
    if dry_run:
        return
    stdin_handle = stdin_path.open("rb") if stdin_path else None
    stdout_handle = stdout_path.open("wb") if stdout_path else None
    stderr_handle = stderr_path.open("wb") if stderr_path else None
    try:
        completed = subprocess.run(
            cmd,
            cwd=cwd,
            stdin=stdin_handle,
            stdout=stdout_handle,
            stderr=stderr_handle,
            check=False,
        )
    finally:
        for handle in (stdin_handle, stdout_handle, stderr_handle):
            if handle is not None:
                handle.close()
    if completed.returncode != 0:
        raise SystemExit(f"command failed with exit {completed.returncode}: {rendered}")


def fetch_metadata(url: str, out_path: Path, dry_run: bool) -> None:
    print(f"fetch {url} -> {out_path}")
    if dry_run:
        return
    ensure_parent(out_path, dry_run=False)
    with urlopen(url, timeout=60) as response:
        if response.status != 200:
            raise SystemExit(f"metadata fetch failed: HTTP {response.status}")
        payload = response.read()
    out_path.write_bytes(payload)


def xproxy_port(args: argparse.Namespace) -> int:
    if args.xproxy_port > 0:
        return args.xproxy_port
    raw = os.environ.get("PI_CONTAINER_XPROXY_PORT", "").strip()
    if raw:
        try:
            port = int(raw)
        except ValueError as exc:
            raise SystemExit("PI_CONTAINER_XPROXY_PORT must be an integer") from exc
        if port <= 0:
            raise SystemExit("PI_CONTAINER_XPROXY_PORT must be positive")
        return port
    return DEFAULT_XPROXY_PORT


def ensure_xproxy(port: int, dry_run: bool) -> None:
    url = f"http://127.0.0.1:{port}/healthz"
    print(f"check xproxy {url}")
    if dry_run:
        return
    try:
        with urlopen(url, timeout=2.0) as response:
            if response.status != 200:
                raise SystemExit(f"xproxy health check failed: HTTP {response.status}")
    except URLError as exc:
        raise SystemExit(f"xproxy is not reachable at {url}; start adc/.bin/adc xproxy first") from exc


def read_noncomment_lines(path: Path) -> list[str]:
    rows: list[str] = []
    for raw in path.read_text().splitlines():
        line = raw.strip()
        if not line or line.startswith("#"):
            continue
        rows.append(line)
    return rows


def build_models(latency_path: Path, models_path: Path, max_elapsed_ms: int, dry_run: bool) -> None:
    print(f"build {models_path} from {latency_path}")
    if dry_run:
        return
    keep: set[str] = set()
    with latency_path.open(newline="") as handle:
        for row in csv.reader(handle):
            if not row or all(not cell.strip() for cell in row):
                continue
            if row[0].strip().lower() in {"model", "xproxy_model"}:
                continue
            if len(row) < 3:
                raise SystemExit(f"invalid latency row: {row}")
            model = row[0].strip()
            elapsed = row[1].strip()
            tools_supported = row[2].strip().lower()
            if tools_supported != "true" or elapsed.lower() == "timeout":
                continue
            try:
                elapsed_ms = int(float(elapsed))
            except ValueError:
                continue
            if max_elapsed_ms > 0 and elapsed_ms > max_elapsed_ms:
                continue
            keep.add(model)
    ensure_parent(models_path, dry_run=False)
    models_path.write_text("".join(model + "\n" for model in sorted(keep)))
    print(f"models={len(keep)}")


def build_source_pool(models_path: Path, persona_dir: Path, common_etc: Path, source_pool_path: Path, dry_run: bool) -> None:
    print(f"build {source_pool_path} from {models_path} x {persona_dir}")
    if dry_run:
        return
    models = read_noncomment_lines(models_path)
    persona_files = sorted(path for path in persona_dir.rglob("*.txt") if path.is_file())
    if not persona_files:
        raise SystemExit(f"no persona files found in {persona_dir}")
    rows: list[str] = []
    for model in models:
        for persona_file in persona_files:
            try:
                persona_ref = persona_file.relative_to(common_etc).as_posix()
            except ValueError as exc:
                raise SystemExit(f"persona file is not under {common_etc}: {persona_file}") from exc
            rows.append(f"{model},{persona_ref}\n")
    ensure_parent(source_pool_path, dry_run=False)
    source_pool_path.write_text("".join(rows))
    print(f"source_pool_rows={len(rows)}")


def parse_sample_size(raw: str, total: int) -> int:
    text = raw.strip().lower()
    if text == "all":
        return total
    try:
        size = int(text)
    except ValueError as exc:
        raise SystemExit("--sample-size must be a positive integer or 'all'") from exc
    if size <= 0:
        raise SystemExit("--sample-size must be positive")
    if size > total:
        raise SystemExit(f"--sample-size={size} exceeds source pool rows ({total})")
    return size


def cluster_input_record(line: str, common_etc: Path, cluster_input_dir: Path) -> str:
    model, sep, persona_ref = line.partition(",")
    if not sep or not model.strip() or not persona_ref.strip():
        raise SystemExit(f"invalid source-pool row: {line}")
    persona_ref = persona_ref.strip()
    persona_path = Path(persona_ref)
    if not persona_path.is_absolute():
        persona_path = common_etc / persona_ref
    try:
        cluster_ref = persona_path.resolve().relative_to(cluster_input_dir.resolve()).as_posix()
    except ValueError:
        cluster_ref = os.path.relpath(persona_path.resolve(), cluster_input_dir.resolve())
    return f"{model.strip()},{cluster_ref}"


def sample_source_pool(source_pool_path: Path, cluster_input_path: Path, common_etc: Path, sample_size_raw: str, seed: int, dry_run: bool) -> None:
    print(f"sample {source_pool_path} -> {cluster_input_path}")
    if dry_run:
        return
    rows = read_noncomment_lines(source_pool_path)
    if not rows:
        raise SystemExit(f"source pool has no usable rows: {source_pool_path}")
    sample_size = parse_sample_size(sample_size_raw, len(rows))
    selected = rows if sample_size == len(rows) else random.Random(seed).sample(rows, sample_size)
    output_rows = [cluster_input_record(row, common_etc, cluster_input_path.parent) for row in selected]
    ensure_parent(cluster_input_path, dry_run=False)
    cluster_input_path.write_text("".join(row + "\n" for row in output_rows))
    print(f"cluster_input_rows={len(selected)} source_pool_rows={len(rows)} sample_seed={seed}")


def validate_council(council_path: Path, common_etc: Path) -> None:
    rows = read_noncomment_lines(council_path)
    if not rows:
        raise SystemExit(f"council file has no usable rows: {council_path}")
    absolute = 0
    bad_relative = 0
    missing = 0
    seen: set[str] = set()
    for line in rows:
        model, sep, persona_ref = line.partition(",")
        if not sep or not model.strip() or not persona_ref.strip():
            raise SystemExit(f"invalid council row: {line}")
        persona_ref = persona_ref.strip()
        if Path(persona_ref).is_absolute():
            absolute += 1
            persona_path = Path(persona_ref)
        else:
            if not persona_ref.startswith("personas/persons/"):
                bad_relative += 1
            persona_path = common_etc / persona_ref
        if not persona_path.is_file():
            missing += 1
        seen.add(line)
    print(
        "council_validation "
        f"rows={len(rows)} unique={len(seen)} absolute={absolute} "
        f"bad_relative={bad_relative} missing_persona_files={missing}"
    )
    if absolute or bad_relative or missing:
        raise SystemExit("council validation failed")


def require_file(path: Path, label: str) -> None:
    if not path.is_file():
        raise SystemExit(f"missing {label}: {path}")


def main(argv: list[str]) -> int:
    args = parse_args(argv)
    root = args.repo_root.resolve()
    if not root.is_dir():
        raise SystemExit(f"repository root does not exist: {root}")
    now = time.time()
    expires = expires_seconds(args.expires)

    metadata = rel_path(root, args.metadata)
    models_prefiltered = rel_path(root, args.models_prefiltered)
    filter_decisions = rel_path(root, args.filter_decisions)
    model_latency = rel_path(root, args.model_latency)
    models = rel_path(root, args.models)
    source_pool = rel_path(root, args.source_pool)
    cluster_input = rel_path(root, args.cluster_input)
    genes = rel_path(root, args.genes)
    clusters = rel_path(root, args.clusters)
    pca = rel_path(root, args.pca)
    failures = rel_path(root, args.failures)
    council = rel_path(root, args.council)
    report = Path(tempfile.gettempdir()) / "generate-council-report.md" if args.report == "" else rel_path(root, args.report)
    model_speed_log = rel_path(root, args.model_speed_log)
    cluster_log = rel_path(root, args.cluster_log)
    common_etc = rel_path(root, args.common_etc)
    persona_dir = rel_path(root, args.persona_dir)
    probe_persona = rel_path(root, args.probe_persona)

    require_file(rel_path(root, "common/tools/filter-models.py"), "filter-models.py")
    require_file(rel_path(root, "common/tools/model-speed.sh"), "model-speed.sh")
    require_file(rel_path(root, "common/tools/cluster-personas.py"), "cluster-personas.py")
    require_file(rel_path(root, "common/tools/select-council.py"), "select-council.py")
    require_file(genes, "genes file")
    require_file(failures, "operational failure ledger")
    require_file(probe_persona, "probe persona")
    if shutil.which("uv") is None:
        raise SystemExit("uv is required")

    if args.use_existing_metadata:
        require_file(metadata, "metadata")
    elif is_fresh(metadata, now, expires):
        use_fresh("metadata", [metadata], root, now)
    else:
        fetch_metadata(args.metadata_url, metadata, args.dry_run)

    filter_outputs = [models_prefiltered, filter_decisions]
    stop_on_partial_fresh("metadata filter", filter_outputs, root, now, expires)
    if fresh_all(filter_outputs, now, expires):
        use_fresh("metadata filter", filter_outputs, root, now)
    else:
        run_command(
            [
                sys.executable,
                "common/tools/filter-models.py",
                "--metadata",
                display(root, metadata),
                "--out",
                display(root, models_prefiltered),
                "--decisions",
                display(root, filter_decisions),
            ],
            cwd=root,
            dry_run=args.dry_run,
        )

    if args.use_existing_latency:
        require_file(model_latency, "model latency CSV")
    elif is_fresh(model_latency, now, expires):
        use_fresh("model latency", [model_latency], root, now)
    else:
        if not args.no_xproxy_check:
            ensure_xproxy(xproxy_port(args), args.dry_run)
        require_file(rel_path(root, "adc/.bin/adc"), "adc/.bin/adc")
        ensure_parent(model_latency, args.dry_run)
        ensure_parent(model_speed_log, args.dry_run)
        run_command(
            ["bash", "common/tools/model-speed.sh", display(root, probe_persona)],
            cwd=root,
            dry_run=args.dry_run,
            stdin_path=models_prefiltered,
            stdout_path=model_latency,
            stderr_path=model_speed_log,
        )

    if is_fresh(models, now, expires):
        use_fresh("retained models", [models], root, now)
    else:
        build_models(model_latency, models, args.max_elapsed_ms, args.dry_run)

    if is_fresh(source_pool, now, expires):
        use_fresh("source pool", [source_pool], root, now)
    else:
        build_source_pool(models, persona_dir, common_etc, source_pool, args.dry_run)

    if is_fresh(cluster_input, now, expires):
        use_fresh("cluster input", [cluster_input], root, now)
    else:
        sample_source_pool(source_pool, cluster_input, common_etc, args.sample_size, args.sample_seed, args.dry_run)

    cluster_outputs = [clusters, pca]
    if args.use_existing_clusters:
        require_file(clusters, "clusters CSV")
        require_file(pca, "PCA CSV")
    else:
        stop_on_partial_fresh("cluster", cluster_outputs, root, now, expires)
    if args.use_existing_clusters or fresh_all(cluster_outputs, now, expires):
        use_fresh("cluster outputs", cluster_outputs, root, now)
    else:
        if not args.no_xproxy_check:
            ensure_xproxy(xproxy_port(args), args.dry_run)
        ensure_parent(clusters, args.dry_run)
        ensure_parent(pca, args.dry_run)
        ensure_parent(cluster_log, args.dry_run)
        run_command(
            [
                "uv",
                "run",
                "--script",
                "common/tools/cluster-personas.py",
                "--personas-file",
                display(root, cluster_input),
                "--genes-file",
                display(root, genes),
                "--pca-out",
                display(root, pca),
                "--num-personas",
                args.num_personas,
                "--num-samples",
                str(args.num_samples),
                "--num-genes",
                args.num_genes,
                "--gene-dim",
                str(args.gene_dim),
            ],
            cwd=root,
            dry_run=args.dry_run,
            stdout_path=clusters,
            stderr_path=cluster_log,
        )

    run_command(
        [
            "uv",
            "run",
            "--script",
            "common/tools/select-council.py",
            "--clusters",
            display(root, clusters),
            "--pca",
            display(root, pca),
            "--metadata",
            display(root, metadata),
            "--latency-csv",
            display(root, model_latency),
            "--failures",
            display(root, failures),
            "--min-context",
            str(args.min_context),
            "--max-elapsed-ms",
            str(args.max_elapsed_ms),
            "--size",
            str(args.size),
            "--out",
            display(root, council),
            "--report",
            display(root, report),
        ],
        cwd=root,
        dry_run=args.dry_run,
    )

    if not args.dry_run:
        validate_council(council, common_etc)
    return 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv[1:]))
