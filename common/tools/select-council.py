#!/usr/bin/env -S uv run --script
# /// script
# requires-python = ">=3.11"
# dependencies = []
# ///
"""Select a behaviorally diverse council from clustering outputs."""

from __future__ import annotations

import argparse
import csv
import hashlib
import json
import math
import statistics
import sys
from collections import Counter, defaultdict
from dataclasses import dataclass, field
from pathlib import Path
from typing import Any, Iterable

DEFAULT_FAILURE_EXCLUSIONS = (
    "council_vote_tool_noncompliance",
    "context_limit",
    "tool_choice_endpoint_failure",
)
DEFAULT_REQUIRED_PARAMETERS = ("tools", "tool_choice")
OPENROUTER_PREFIX = "openrouter://"
EPSILON = 1e-12


@dataclass(frozen=True)
class Metadata:
    model: str
    provider: str
    top_context: int | None
    provider_context: int | None
    effective_context: int | None
    provider_max_completion_tokens: int | None
    supported_parameters: frozenset[str]


@dataclass
class Candidate:
    model: str
    persona_file: str
    absolute_persona_path: Path
    row_count: int
    gene_counts: Counter[int]
    cluster_counts: Counter[tuple[int, int]]
    pca_rows: list[tuple[int, float, float, float]] = field(default_factory=list)
    metadata: Metadata | None = None
    latency_ms: int | None = None
    latency_tools_supported: str | None = None
    failures: list[str] = field(default_factory=list)
    reasons: list[str] = field(default_factory=list)
    vector: tuple[float, ...] = ()

    @property
    def key(self) -> tuple[str, str]:
        return self.model, self.persona_file

    @property
    def provider(self) -> str:
        if self.metadata is not None:
            return self.metadata.provider
        return provider_from_model(self.model)


def parse_args(argv: list[str]) -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--clusters", required=True, type=Path, help="Headerless MODEL,PERSONA_FILE,GENE,CLUSTER CSV")
    parser.add_argument("--pca", type=Path, help="Optional headerless MODEL,PERSONA_FILE,GENE,PC1,PC2,PC3,CLUSTER CSV")
    parser.add_argument("--metadata", required=True, type=Path, help="OpenRouter metadata JSON, either a data object or raw list")
    parser.add_argument("--latency-csv", type=Path, help="Optional MODEL,ELAPSED_MS,TOOLS_SUPPORTED CSV")
    parser.add_argument("--no-latency-filter", action="store_true", help="Load latency data for reporting but do not filter on it")
    parser.add_argument("--failures", type=Path, help="Optional operational failure ledger CSV")
    parser.add_argument("--failure-exclusions", default=",".join(DEFAULT_FAILURE_EXCLUSIONS), help="Comma-separated failure types to exclude")
    parser.add_argument("--expected-genes", type=int, default=3, help="Expected distinct genes per candidate. Default: %(default)s")
    parser.add_argument("--samples-per-gene", type=int, default=3, help="Expected samples per gene. Default: %(default)s")
    parser.add_argument("--required-parameters", default=",".join(DEFAULT_REQUIRED_PARAMETERS), help="Comma-separated supported_parameters required in metadata. Empty disables this filter")
    parser.add_argument("--min-context", type=int, default=128000, help="Minimum effective context length. Default: %(default)s")
    parser.add_argument("--min-completion-tokens", type=int, default=0, help="Minimum provider max_completion_tokens when that value is present. Default 0 disables")
    parser.add_argument("--max-elapsed-ms", type=int, default=8000, help="Maximum latency when latency filtering is enabled. Use 0 to disable this threshold")
    parser.add_argument("--size", type=int, required=True, help="Number of council rows to select")
    parser.add_argument("--out", required=True, type=Path, help="Output council CSV, no header")
    parser.add_argument("--report", required=True, type=Path, help="Markdown report path")
    parser.add_argument("--tie-break-label", default="", help="Optional deterministic label for tie-breaking")
    return parser.parse_args(argv)


def parse_int(value: Any) -> int | None:
    if value is None:
        return None
    try:
        return int(value)
    except (TypeError, ValueError):
        return None


def comma_set(raw: str) -> set[str]:
    return {item.strip() for item in raw.split(",") if item.strip()}


def provider_from_model(model: str) -> str:
    rest = model.split("://", 1)[1] if "://" in model else model
    return rest.split("/", 1)[0] if rest else "unknown"


def metadata_lookup_key(model: str) -> str:
    return model[len(OPENROUTER_PREFIX) :] if model.startswith(OPENROUTER_PREFIX) else model


def load_metadata(path: Path) -> dict[str, Metadata]:
    payload = json.loads(path.read_text())
    data = payload.get("data") if isinstance(payload, dict) else payload
    if not isinstance(data, list):
        raise ValueError("metadata JSON must be a list or an object with a data list")

    out: dict[str, Metadata] = {}
    for item in data:
        if not isinstance(item, dict):
            raise ValueError("metadata JSON contains a non-object model record")
        raw_id = str(item.get("id", "")).strip()
        if not raw_id:
            continue
        top_provider = item.get("top_provider")
        if not isinstance(top_provider, dict):
            top_provider = {}
        top_context = parse_int(item.get("context_length"))
        provider_context = parse_int(top_provider.get("context_length"))
        effective_context = provider_context if provider_context is not None else top_context
        provider_max_completion_tokens = parse_int(top_provider.get("max_completion_tokens"))
        supported_raw = item.get("supported_parameters")
        supported_parameters = frozenset(
            str(value).strip() for value in supported_raw if str(value).strip()
        ) if isinstance(supported_raw, list) else frozenset()
        model = OPENROUTER_PREFIX + raw_id
        metadata = Metadata(
            model=model,
            provider=provider_from_model(model),
            top_context=top_context,
            provider_context=provider_context,
            effective_context=effective_context,
            provider_max_completion_tokens=provider_max_completion_tokens,
            supported_parameters=supported_parameters,
        )
        out[raw_id] = metadata
        out[model] = metadata
    return out


def resolve_persona_path(repo_root: Path, clusters_path: Path, persona_file: str) -> Path:
    path = Path(persona_file)
    if path.is_absolute():
        return path.resolve()

    by_cluster_path = (clusters_path.parent / path).resolve()
    if by_cluster_path.exists() or "common/etc/personas" in by_cluster_path.as_posix():
        return by_cluster_path

    by_common_etc = (repo_root / "common" / "etc" / path).resolve()
    if by_common_etc.exists() or path.as_posix().startswith("personas/"):
        return by_common_etc

    return by_cluster_path


def load_clusters(path: Path, repo_root: Path) -> dict[tuple[str, str], Candidate]:
    candidates: dict[tuple[str, str], Candidate] = {}
    with path.open(newline="") as handle:
        for line_num, row in enumerate(csv.reader(handle), start=1):
            if not row or all(not cell.strip() for cell in row):
                continue
            if len(row) != 4:
                raise ValueError(f"{path}:{line_num}: expected 4 columns, found {len(row)}")
            model, persona_file, gene_text, cluster_text = [cell.strip() for cell in row]
            try:
                gene_index = int(gene_text)
                cluster_num = int(cluster_text)
            except ValueError as exc:
                raise ValueError(f"{path}:{line_num}: gene and cluster must be integers") from exc
            key = (model, persona_file)
            if key not in candidates:
                candidates[key] = Candidate(
                    model=model,
                    persona_file=persona_file,
                    absolute_persona_path=resolve_persona_path(repo_root, path, persona_file),
                    row_count=0,
                    gene_counts=Counter(),
                    cluster_counts=Counter(),
                )
            candidate = candidates[key]
            candidate.row_count += 1
            candidate.gene_counts[gene_index] += 1
            candidate.cluster_counts[(gene_index, cluster_num)] += 1
    return candidates


def load_pca(path: Path) -> dict[tuple[str, str], list[tuple[int, float, float, float]]]:
    rows: dict[tuple[str, str], list[tuple[int, float, float, float]]] = defaultdict(list)
    with path.open(newline="") as handle:
        for line_num, row in enumerate(csv.reader(handle), start=1):
            if not row or all(not cell.strip() for cell in row):
                continue
            if len(row) != 7:
                raise ValueError(f"{path}:{line_num}: expected 7 columns, found {len(row)}")
            model, persona_file, gene_text, pc1_text, pc2_text, pc3_text, _cluster_text = [cell.strip() for cell in row]
            try:
                rows[(model, persona_file)].append((int(gene_text), float(pc1_text), float(pc2_text), float(pc3_text)))
            except ValueError as exc:
                raise ValueError(f"{path}:{line_num}: invalid PCA row") from exc
    return rows


def load_latency(path: Path) -> dict[str, tuple[str, str]]:
    latency: dict[str, tuple[str, str]] = {}
    with path.open(newline="") as handle:
        for line_num, row in enumerate(csv.reader(handle), start=1):
            if not row or all(not cell.strip() for cell in row):
                continue
            if line_num == 1 and row[0].strip().lower() in {"model", "xproxy_model"}:
                continue
            if len(row) < 3:
                raise ValueError(f"{path}:{line_num}: expected at least 3 columns")
            model = row[0].strip()
            elapsed = row[1].strip()
            tools_supported = row[2].strip().lower()
            latency[model] = (elapsed, tools_supported)
    return latency


def load_failures(path: Path, exclusion_types: set[str]) -> dict[str, list[str]]:
    failures: dict[str, list[str]] = defaultdict(list)
    with path.open(newline="") as handle:
        reader = csv.DictReader(handle)
        required = {"model", "failure_type", "source_run", "observed_at", "notes"}
        if reader.fieldnames is None or not required.issubset(reader.fieldnames):
            raise ValueError(f"{path}: expected header {','.join(sorted(required))}")
        for row in reader:
            model = (row.get("model") or "").strip()
            failure_type = (row.get("failure_type") or "").strip()
            if model and failure_type in exclusion_types:
                failures[model].append(failure_type)
    return failures


def apply_filters(
    candidates: Iterable[Candidate],
    metadata: dict[str, Metadata],
    latency: dict[str, tuple[str, str]] | None,
    failures: dict[str, list[str]] | None,
    args: argparse.Namespace,
) -> tuple[list[Candidate], Counter[str]]:
    required_parameters = comma_set(args.required_parameters)
    expected_total = args.expected_genes * args.samples_per_gene
    exclusion_counts: Counter[str] = Counter()
    eligible: list[Candidate] = []

    for candidate in candidates:
        reasons: list[str] = []
        if candidate.row_count != expected_total:
            reasons.append("coverage_row_count")
        if len(candidate.gene_counts) != args.expected_genes:
            reasons.append("coverage_gene_count")
        for _gene, count in candidate.gene_counts.items():
            if count != args.samples_per_gene:
                reasons.append("coverage_samples_per_gene")
                break

        model_metadata = metadata.get(candidate.model) or metadata.get(metadata_lookup_key(candidate.model))
        candidate.metadata = model_metadata
        if model_metadata is None:
            reasons.append("metadata_missing")
        else:
            missing_parameters = sorted(required_parameters - model_metadata.supported_parameters)
            if missing_parameters:
                reasons.append("required_parameters_missing:" + "+".join(missing_parameters))
            if model_metadata.effective_context is None:
                reasons.append("context_missing")
            elif model_metadata.effective_context < args.min_context:
                reasons.append("context_below_min")
            max_completion = model_metadata.provider_max_completion_tokens
            if args.min_completion_tokens > 0 and max_completion is not None and max_completion < args.min_completion_tokens:
                reasons.append("completion_tokens_below_min")

        if latency is not None and not args.no_latency_filter:
            latency_row = latency.get(candidate.model)
            if latency_row is None:
                reasons.append("latency_missing")
            else:
                elapsed_text, tools_supported = latency_row
                candidate.latency_tools_supported = tools_supported
                if tools_supported != "true":
                    reasons.append("latency_tools_not_supported")
                if elapsed_text.strip().lower() == "timeout":
                    reasons.append("latency_timeout")
                else:
                    try:
                        elapsed_ms = int(float(elapsed_text))
                        candidate.latency_ms = elapsed_ms
                    except ValueError:
                        reasons.append("latency_elapsed_invalid")
                    else:
                        if args.max_elapsed_ms > 0 and elapsed_ms > args.max_elapsed_ms:
                            reasons.append("latency_above_max")

        if failures is not None:
            candidate.failures = failures.get(candidate.model, [])
            for failure_type in candidate.failures:
                reasons.append("operational_failure:" + failure_type)

        candidate.reasons = reasons
        if reasons:
            exclusion_counts.update(reasons)
        else:
            eligible.append(candidate)

    return eligible, exclusion_counts


def attach_pca(candidates: dict[tuple[str, str], Candidate], pca: dict[tuple[str, str], list[tuple[int, float, float, float]]]) -> None:
    for key, rows in pca.items():
        candidate = candidates.get(key)
        if candidate is not None:
            candidate.pca_rows = rows


def build_pca_vectors(candidates: list[Candidate], expected_genes: int) -> None:
    genes = sorted({gene for candidate in candidates for gene in candidate.gene_counts})
    if len(genes) != expected_genes:
        genes = sorted(genes)[:expected_genes]
    for candidate in candidates:
        by_gene: dict[int, list[tuple[float, float, float]]] = defaultdict(list)
        for gene, pc1, pc2, pc3 in candidate.pca_rows:
            by_gene[gene].append((pc1, pc2, pc3))
        vector: list[float] = []
        for gene in genes:
            coords = by_gene.get(gene, [])
            if coords:
                vector.extend(sum(axis) / len(coords) for axis in zip(*coords))
            else:
                vector.extend((0.0, 0.0, 0.0))
        candidate.vector = tuple(vector)


def build_cluster_vectors(candidates: list[Candidate]) -> None:
    feature_keys = sorted({feature for candidate in candidates for feature in candidate.cluster_counts})
    for candidate in candidates:
        total = max(candidate.row_count, 1)
        candidate.vector = tuple(candidate.cluster_counts.get(feature, 0) / total for feature in feature_keys)


def euclidean(left: tuple[float, ...], right: tuple[float, ...]) -> float:
    return math.sqrt(sum((a - b) * (a - b) for a, b in zip(left, right)))


def centroid(vectors: list[tuple[float, ...]]) -> tuple[float, ...]:
    if not vectors:
        return ()
    width = len(vectors[0])
    return tuple(sum(vector[i] for vector in vectors) / len(vectors) for i in range(width))


def tie_key(candidate: Candidate, label: str) -> tuple[str, str, str]:
    digest = hashlib.sha256(f"{label}\0{candidate.model}\0{candidate.persona_file}".encode()).hexdigest() if label else ""
    return digest, candidate.model, candidate.persona_file


def select_farthest_first(candidates: list[Candidate], size: int, label: str) -> list[Candidate]:
    if size <= 0:
        raise ValueError("--size must be positive")
    if len(candidates) < size:
        raise ValueError(f"only {len(candidates)} eligible candidates for requested size {size}")
    if any(not candidate.vector for candidate in candidates):
        raise ValueError("candidate signature vectors are missing")

    remaining = sorted(candidates, key=lambda candidate: tie_key(candidate, label))
    selected: list[Candidate] = []
    provider_counts: Counter[str] = Counter()

    center = centroid([candidate.vector for candidate in remaining])
    seed = best_by_score(remaining, lambda candidate: euclidean(candidate.vector, center), label)
    selected.append(seed)
    provider_counts[seed.provider] += 1
    remaining.remove(seed)

    cap = 1
    while len(selected) < size:
        allowed = [candidate for candidate in remaining if provider_counts[candidate.provider] < cap]
        if not allowed:
            cap += 1
            continue
        chosen = best_by_score(
            allowed,
            lambda candidate: min(euclidean(candidate.vector, selected_candidate.vector) for selected_candidate in selected),
            label,
        )
        selected.append(chosen)
        provider_counts[chosen.provider] += 1
        remaining.remove(chosen)
    return selected


def best_by_score(candidates: list[Candidate], score_fn: Any, label: str) -> Candidate:
    best: Candidate | None = None
    best_score = float("-inf")
    for candidate in sorted(candidates, key=lambda item: tie_key(item, label)):
        score = score_fn(candidate)
        if score > best_score + EPSILON:
            best = candidate
            best_score = score
    if best is None:
        raise ValueError("cannot choose from an empty candidate list")
    return best


def context_stats(candidates: list[Candidate]) -> str:
    values = [candidate.metadata.effective_context for candidate in candidates if candidate.metadata and candidate.metadata.effective_context is not None]
    if not values:
        return "none"
    return f"min={min(values)} median={int(statistics.median(values))} max={max(values)}"


def vector_summary(candidate: Candidate, limit: int = 6) -> str:
    values = candidate.vector[:limit]
    rendered = ", ".join(f"{value:.4g}" for value in values)
    if len(candidate.vector) > limit:
        rendered += ", ..."
    norm = euclidean(candidate.vector, tuple(0.0 for _ in candidate.vector))
    return f"dim={len(candidate.vector)} norm={norm:.4g} [{rendered}]"


def value_or_blank(value: Any) -> str:
    return "" if value is None else str(value)


def output_persona_ref(candidate: Candidate, repo_root: Path) -> str:
    common_etc = (repo_root / "common" / "etc").resolve()
    try:
        return candidate.absolute_persona_path.resolve().relative_to(common_etc).as_posix()
    except ValueError:
        pass
    if not Path(candidate.persona_file).is_absolute():
        return candidate.persona_file
    return str(candidate.absolute_persona_path)


def write_outputs(selected: list[Candidate], out_path: Path, report_path: Path, all_count: int, eligible_count: int, exclusions: Counter[str], used_pca: bool, args: argparse.Namespace, repo_root: Path) -> None:
    out_path.parent.mkdir(parents=True, exist_ok=True)
    report_path.parent.mkdir(parents=True, exist_ok=True)
    with out_path.open("w", newline="") as handle:
        writer = csv.writer(handle, lineterminator="\n")
        for candidate in selected:
            writer.writerow([candidate.model, output_persona_ref(candidate, repo_root)])

    provider_counts = Counter(candidate.provider for candidate in selected)
    lines: list[str] = []
    lines.append("# Council Selection Report")
    lines.append("")
    lines.append("## Inputs")
    lines.append("")
    lines.append(f"- Clusters: `{args.clusters}`")
    lines.append(f"- PCA: `{args.pca}`" if args.pca else "- PCA: not supplied")
    lines.append(f"- Metadata: `{args.metadata}`")
    lines.append(f"- Latency CSV: `{args.latency_csv}`" if args.latency_csv else "- Latency CSV: not supplied")
    lines.append(f"- Failures CSV: `{args.failures}`" if args.failures else "- Failures CSV: not supplied")
    lines.append(f"- Signature source: {'PCA mean vectors' if used_pca else 'cluster signatures'}")
    lines.append("")
    lines.append("## Eligibility")
    lines.append("")
    lines.append(f"- Candidate model/persona pairs: {all_count}")
    lines.append(f"- Eligible candidate pairs: {eligible_count}")
    lines.append(f"- Selected rows: {len(selected)}")
    lines.append(f"- Expected coverage: {args.expected_genes} genes x {args.samples_per_gene} samples")
    lines.append(f"- Minimum context: {args.min_context}")
    lines.append(f"- Required parameters: {args.required_parameters or 'none'}")
    lines.append(f"- Maximum elapsed ms: {args.max_elapsed_ms if args.latency_csv and not args.no_latency_filter else 'not applied'}")
    lines.append("")
    lines.append("## Exclusions")
    lines.append("")
    if exclusions:
        for reason, count in sorted(exclusions.items()):
            lines.append(f"- {reason}: {count}")
    else:
        lines.append("- None")
    lines.append("")
    lines.append("## Selected Provider Counts")
    lines.append("")
    for provider, count in sorted(provider_counts.items()):
        lines.append(f"- {provider}: {count}")
    lines.append("")
    lines.append("## Context Stats")
    lines.append("")
    lines.append(f"- Selected effective context: {context_stats(selected)}")
    lines.append("")
    lines.append("## Selected Rows")
    lines.append("")
    lines.append("| # | Model | Persona | Provider | Top context | Provider context | Effective context | Provider max completion | Latency ms | Signature |")
    lines.append("|---:|---|---|---|---:|---:|---:|---:|---:|---|")
    for index, candidate in enumerate(selected, start=1):
        metadata = candidate.metadata
        lines.append(
            "| "
            + " | ".join(
                [
                    str(index),
                    candidate.model,
                    output_persona_ref(candidate, repo_root),
                    candidate.provider,
                    value_or_blank(metadata.top_context if metadata else None),
                    value_or_blank(metadata.provider_context if metadata else None),
                    value_or_blank(metadata.effective_context if metadata else None),
                    value_or_blank(metadata.provider_max_completion_tokens if metadata else None),
                    value_or_blank(candidate.latency_ms),
                    vector_summary(candidate).replace("|", "\\|"),
                ]
            )
            + " |"
        )
    lines.append("")
    report_path.write_text("\n".join(lines) + "\n")


def main(argv: list[str]) -> int:
    args = parse_args(argv)
    repo_root = Path.cwd().resolve()
    candidates_by_key = load_clusters(args.clusters, repo_root)
    metadata = load_metadata(args.metadata)
    latency = load_latency(args.latency_csv) if args.latency_csv else None
    failure_exclusions = comma_set(args.failure_exclusions)
    failures = load_failures(args.failures, failure_exclusions) if args.failures else None

    used_pca = False
    if args.pca:
        pca_rows = load_pca(args.pca)
        attach_pca(candidates_by_key, pca_rows)
        used_pca = any(candidate.pca_rows for candidate in candidates_by_key.values())

    eligible, exclusions = apply_filters(candidates_by_key.values(), metadata, latency, failures, args)
    if used_pca:
        build_pca_vectors(eligible, args.expected_genes)
    else:
        build_cluster_vectors(eligible)
    selected = select_farthest_first(eligible, args.size, args.tie_break_label)
    write_outputs(selected, args.out, args.report, len(candidates_by_key), len(eligible), exclusions, used_pca, args, repo_root)
    print(f"candidates={len(candidates_by_key)} eligible={len(eligible)} selected={len(selected)} out={args.out}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv[1:]))
