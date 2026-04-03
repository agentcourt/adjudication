#!/usr/bin/env -S uv run --script
# /// script
# requires-python = ">=3.11"
# dependencies = [
#   "numpy>=2.1.0",
#   "openai>=1.68.0",
#   "scikit-learn>=1.5.0",
# ]
# ///

"""
Sample model/persona behavior over a gene set and emit cluster assignments.

Usage:

1. Start local xproxy from the repository root:

   adc/.bin/adc xproxy

2. Run the script:

   uv run common/tools/cluster-personas.py --personas-file adc/etc/some-personas.csv --num-samples 3 --num-genes 5 --num-personas 25

The shared persona pool is `common/etc/personas.csv`.  The checked-in sampled
subset is `adc/etc/some-personas.csv`.

Design:

- Completions go through local xproxy because the personas file stores xproxy
  model ids, and this repository already routes provider access through xproxy.
- Embeddings go directly through the OpenAI Python SDK because the local xproxy
  implementation exposes `/v1/responses` but not an embeddings endpoint.
- Embeddings run one sampled response at a time.  That avoids oversized batch
  requests and keeps one bad embedding response from aborting the whole run.
- PCA runs once per gene over the full embedding set for that gene, across all
  sampled completions for all model/persona pairs.  That matches the task.
- The script keeps the cluster CSV on stdout and also writes per-sample PCA
  rows to `adc/etc/personas-pca.csv` by default.
- K-means chooses `k` by maximizing silhouette score across all admissible
  cluster counts in the fixed range `3..10`.  If the data are too small or too
  degenerate to score, the script assigns cluster `0` to every point for that
  gene.
- The requested PCA dimension can exceed what the sample count permits.  In that
  case the script computes the largest valid PCA basis and pads the remaining
  coordinates with zeroes so the downstream clustering still receives a fixed
  per-gene dimension.
- `--num-genes` controls how many prompts are sampled from the genes file
  without replacement.  The default is `5`.  `all` uses the entire file.
- `--num-personas` controls how many model/persona pairs are sampled from the
  personas file without replacement.  The default is `25`.  `all` uses the
  entire file.
- The task asks for `N` samples per model/persona pair and notes that multiple
  completions in one request would be preferable.  This script issues repeated
  Responses API calls instead.  That is the direct and reliable path through the
  current xproxy surface.
- If a model times out or returns an unusable response, the script logs that
  failure to stderr, removes that model from the remaining run, and continues
  with the remaining models.
- Each completion attempt writes one stderr line in the form
  `MODEL,I,BYTES,LATENCY_MS`.  Failures use `timeout,timeout` or `error,error`
  for the last two fields.
- Gene sampling runs in parallel.  The script uses one worker thread per
  selected gene.

Operational requirements:

- xproxy must already be running and reachable on `127.0.0.1`.
- `OPENAI_API_KEY` must be set for embeddings.
- The script resolves relative persona paths relative to the personas CSV file,
  matching the existing Go runtime behavior.
"""

from __future__ import annotations

import argparse
import concurrent.futures
import csv
import json
import os
import random
from dataclasses import dataclass
from pathlib import Path
import sys
import time
from typing import Any
from urllib.error import URLError
from urllib.request import urlopen

import numpy as np
from openai import APIConnectionError, APIStatusError, APITimeoutError, OpenAI
from sklearn.cluster import KMeans
from sklearn.decomposition import PCA
from sklearn.metrics import silhouette_score


REPO_ROOT = Path(__file__).resolve().parents[2]
DEFAULT_XPROXY_PORT = 18459
DEFAULT_EMBEDDING_MODEL = "text-embedding-3-small"
DEFAULT_EMBEDDING_BASE_URL = "https://api.openai.com/v1"
DEFAULT_TIMEOUT_SECONDS = 120.0


@dataclass(frozen=True)
class PersonaSpec:
    model: str
    file_ref: str
    text: str


@dataclass(frozen=True)
class Sample:
    model: str
    persona_file: str
    gene_index: int
    text: str


@dataclass(frozen=True)
class CompletionSample:
    text: str
    bytes_out: int
    latency_ms: int


@dataclass(frozen=True)
class ClusteredSample:
    model: str
    persona_file: str
    gene_index: int
    coords: tuple[float, ...]
    cluster_num: int


class ModelFailure(Exception):
    def __init__(self, model: str, reason: str) -> None:
        super().__init__(reason)
        self.model = model
        self.reason = reason


def parse_args(argv: list[str]) -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description=(
            "Sample completions for model/persona pairs over a gene set, embed the "
            "responses, cluster them per gene, and write CSV rows M,P,G,C to stdout."
        ),
        formatter_class=argparse.RawTextHelpFormatter,
    )
    parser.add_argument(
        "--personas-file",
        default="common/etc/personas.csv",
        help=(
            "CSV-like file of model/persona records.\n"
            "Default: %(default)s"
        ),
    )
    parser.add_argument(
        "--genes-file",
        default="adc/etc/genes.json",
        help=(
            "JSON array of prompt strings.\n"
            "Default: %(default)s"
        ),
    )
    parser.add_argument(
        "--num-samples",
        type=int,
        default=5,
        help=(
            "Number of completions to sample per selected model/persona pair per gene.\n"
            "Default: %(default)s"
        ),
    )
    parser.add_argument(
        "--num-genes",
        default="5",
        help=(
            "Number of gene prompts to sample without replacement, or 'all'.\n"
            "Default: %(default)s"
        ),
    )
    parser.add_argument(
        "--num-personas",
        default="25",
        help=(
            "Number of model/persona records to sample without replacement, or 'all'.\n"
            "Default: %(default)s"
        ),
    )
    parser.add_argument(
        "--gene-dim",
        type=int,
        default=3,
        help=(
            "PCA dimensions per gene before clustering.\n"
            "Default: %(default)s"
        ),
    )
    parser.add_argument(
        "--pca-out",
        default="adc/etc/personas-pca.csv",
        help=(
            "Path for per-sample PCA output. Empty disables the file.\n"
            "Default: %(default)s"
        ),
    )
    return parser.parse_args(argv)


def resolve_num_genes(raw: str) -> int | None:
    text = raw.strip().lower()
    if text == "all":
        return None
    try:
        value = int(text)
    except ValueError as exc:
        raise SystemExit("--num-genes must be a positive integer or 'all'") from exc
    if value <= 0:
        raise SystemExit("--num-genes must be a positive integer or 'all'")
    return value


def resolve_num_personas(raw: str) -> int | None:
    text = raw.strip().lower()
    if text == "all":
        return None
    try:
        value = int(text)
    except ValueError as exc:
        raise SystemExit("--num-personas must be a positive integer or 'all'") from exc
    if value <= 0:
        raise SystemExit("--num-personas must be a positive integer or 'all'")
    return value


def resolve_path(path_text: str) -> Path:
    path = Path(path_text)
    if path.is_absolute():
        return path
    return (REPO_ROOT / path).resolve()


def parse_xproxy_model(model: str) -> None:
    if "://" not in model:
        raise SystemExit(f"invalid persona model {model!r}: expected ENDPOINT://MODEL")
    endpoint, rest = model.split("://", 1)
    if not endpoint.strip() or not rest.strip():
        raise SystemExit(f"invalid persona model {model!r}: expected ENDPOINT://MODEL")


def juror_prompt(persona_text: str) -> str:
    persona_text = persona_text.strip()
    if not persona_text:
        raise SystemExit("persona text is empty")
    return (
        "This juror identity is yours for this prompt. "
        "Treat it as true of yourself, including any bias, skepticism, hardship, "
        "or limits it implies:\n"
        f"{persona_text}"
    )


def load_personas(path_text: str) -> list[PersonaSpec]:
    path = resolve_path(path_text)
    try:
        raw = path.read_text(encoding="utf-8")
    except OSError as exc:
        raise SystemExit(f"read persona records {path_text}: {exc}") from exc
    specs: list[PersonaSpec] = []
    for raw_line in raw.splitlines():
        line = raw_line.strip()
        if not line or line.startswith("#"):
            continue
        model, sep, file_ref = line.partition(",")
        if not sep:
            raise SystemExit(f"invalid persona record: {line}")
        model = model.strip()
        file_ref = file_ref.strip()
        if not model or not file_ref:
            raise SystemExit(f"invalid persona record: {line}")
        parse_xproxy_model(model)
        file_path = Path(file_ref)
        if not file_path.is_absolute():
            file_path = (path.parent / file_ref).resolve()
        try:
            text = file_path.read_text(encoding="utf-8").strip()
        except OSError as exc:
            raise SystemExit(f"read persona text {file_ref}: {exc}") from exc
        if not text:
            raise SystemExit(f"empty persona text: {file_ref}")
        specs.append(PersonaSpec(model=model, file_ref=file_ref, text=text))
    if not specs:
        raise SystemExit(f"persona records file contains no usable entries: {path_text}")
    return specs


def load_genes(path_text: str) -> list[str]:
    path = resolve_path(path_text)
    try:
        raw = path.read_text(encoding="utf-8")
    except OSError as exc:
        raise SystemExit(f"read genes file {path_text}: {exc}") from exc
    try:
        payload = json.loads(raw)
    except json.JSONDecodeError as exc:
        raise SystemExit(f"parse genes file {path_text}: {exc}") from exc
    if not isinstance(payload, list) or not payload:
        raise SystemExit(f"genes file must be a non-empty JSON array: {path_text}")
    genes: list[str] = []
    for index, item in enumerate(payload):
        if not isinstance(item, str) or not item.strip():
            raise SystemExit(f"gene {index} must be a non-empty string")
        genes.append(item)
    return genes


def select_genes(genes: list[str], num_genes: int | None) -> list[tuple[int, str]]:
    indexed = list(enumerate(genes))
    if num_genes is None:
        return indexed
    if num_genes > len(indexed):
        raise SystemExit(
            f"--num-genes={num_genes} exceeds available gene prompts ({len(indexed)})"
        )
    return random.sample(indexed, num_genes)


def select_personas(
    personas: list[PersonaSpec], num_personas: int | None
) -> list[PersonaSpec]:
    if num_personas is None:
        return personas
    if num_personas > len(personas):
        raise SystemExit(
            f"--num-personas={num_personas} exceeds available persona records ({len(personas)})"
        )
    return random.sample(personas, num_personas)


def resolve_xproxy_port() -> int:
    raw = os.environ.get("PI_CONTAINER_XPROXY_PORT", "").strip()
    if not raw:
        return DEFAULT_XPROXY_PORT
    try:
        port = int(raw)
    except ValueError as exc:
        raise SystemExit("PI_CONTAINER_XPROXY_PORT must be a positive integer") from exc
    if port <= 0:
        raise SystemExit("PI_CONTAINER_XPROXY_PORT must be a positive integer")
    return port


def resolve_timeout_seconds() -> float:
    raw = os.environ.get("PERSONA_SAMPLE_TIMEOUT_SECONDS", "").strip()
    if not raw:
        return DEFAULT_TIMEOUT_SECONDS
    try:
        value = float(raw)
    except ValueError as exc:
        raise SystemExit("PERSONA_SAMPLE_TIMEOUT_SECONDS must be a positive number") from exc
    if value <= 0:
        raise SystemExit("PERSONA_SAMPLE_TIMEOUT_SECONDS must be a positive number")
    return value


def ensure_xproxy_healthy(port: int, timeout_seconds: float) -> None:
    url = f"http://127.0.0.1:{port}/healthz"
    try:
        with urlopen(url, timeout=min(timeout_seconds, 2.0)) as response:
            if response.status != 200:
                raise SystemExit(
                    f"xproxy health check failed at {url}: status {response.status}"
                )
    except URLError as exc:
        raise SystemExit(
            f"xproxy is not reachable at {url}. Run `adc/.bin/adc xproxy` first."
        ) from exc


def build_xproxy_client(port: int, timeout_seconds: float) -> OpenAI:
    return OpenAI(
        api_key=os.environ.get("PERSONA_SAMPLE_XPROXY_API_KEY", "xproxy"),
        base_url=f"http://127.0.0.1:{port}/v1",
        timeout=timeout_seconds,
        max_retries=0,
    )


def build_embeddings_client(timeout_seconds: float) -> OpenAI:
    api_key = os.environ.get("OPENAI_API_KEY", "").strip()
    if not api_key:
        raise SystemExit("OPENAI_API_KEY is required for embeddings")
    base_url = os.environ.get(
        "PERSONA_SAMPLE_EMBEDDING_BASE_URL",
        DEFAULT_EMBEDDING_BASE_URL,
    ).strip()
    if not base_url:
        raise SystemExit("PERSONA_SAMPLE_EMBEDDING_BASE_URL must be non-empty")
    return OpenAI(
        api_key=api_key,
        base_url=base_url,
        timeout=timeout_seconds,
        max_retries=0,
    )


def log_completion_attempt(model: str, gene_index: int, bytes_out: str, latency: str) -> None:
    print(f"{model},{gene_index},{bytes_out},{latency}", file=sys.stderr)


def log_gene_error(gene_index: int, message: str) -> None:
    print(f"gene {gene_index}: {message}", file=sys.stderr)


def sample_completion(client: OpenAI, spec: PersonaSpec, gene: str) -> CompletionSample:
    started = time.monotonic()
    try:
        response = client.responses.create(
            model=spec.model,
            input=[
                {"role": "system", "content": juror_prompt(spec.text)},
                {"role": "user", "content": gene},
            ],
        )
    except APITimeoutError as exc:
        raise ModelFailure(spec.model, "timeout") from exc
    except APIConnectionError as exc:
        raise ModelFailure(spec.model, f"connection error: {exc}") from exc
    except APIStatusError as exc:
        raise ModelFailure(spec.model, f"status {exc.status_code}") from exc
    except json.JSONDecodeError as exc:
        raise ModelFailure(spec.model, f"invalid JSON response: {exc}") from exc
    except Exception as exc:
        raise ModelFailure(spec.model, f"unexpected error: {exc}") from exc
    text = (response.output_text or "").strip()
    if not text:
        raise ModelFailure(spec.model, "empty response text")
    return CompletionSample(
        text=text,
        bytes_out=len(text.encode("utf-8")),
        latency_ms=int((time.monotonic() - started) * 1000),
    )


def embed_text(client: OpenAI, text: str) -> np.ndarray:
    model = os.environ.get(
        "PERSONA_SAMPLE_EMBEDDING_MODEL",
        DEFAULT_EMBEDDING_MODEL,
    ).strip()
    if not model:
        raise SystemExit("PERSONA_SAMPLE_EMBEDDING_MODEL must be non-empty")
    response = client.embeddings.create(model=model, input=[text])
    if not response.data:
        raise RuntimeError("embedding response was empty")
    return np.asarray(response.data[0].embedding, dtype=np.float64)


def reduce_gene_vectors(matrix: np.ndarray, gene_dim: int) -> np.ndarray:
    if matrix.ndim != 2:
        raise SystemExit(f"embedding matrix must be two-dimensional, got {matrix.ndim}")
    rows, cols = matrix.shape
    if rows == 0 or cols == 0:
        raise SystemExit("embedding matrix must be non-empty")
    if rows < 2:
        return np.zeros((rows, gene_dim), dtype=np.float64)
    n_components = min(gene_dim, rows, cols)
    reduced = PCA(n_components=n_components).fit_transform(matrix)
    if n_components == gene_dim:
        return reduced
    out = np.zeros((rows, gene_dim), dtype=np.float64)
    out[:, :n_components] = reduced
    return out


def cluster_gene_vectors(matrix: np.ndarray) -> np.ndarray:
    rows = matrix.shape[0]
    if rows < 4:
        return np.zeros(rows, dtype=np.int64)
    best_labels: np.ndarray | None = None
    best_score: float | None = None
    for clusters in range(3, min(10, rows-1) + 1):
        try:
            model = KMeans(n_clusters=clusters, n_init=10, random_state=0)
            labels = model.fit_predict(matrix)
        except Exception:
            continue
        if len(set(int(value) for value in labels)) < 2:
            continue
        try:
            score = float(silhouette_score(matrix, labels))
        except Exception:
            continue
        if best_score is None or score > best_score:
            best_score = score
            best_labels = labels.astype(np.int64, copy=False)
    if best_labels is None:
        return np.zeros(rows, dtype=np.int64)
    return best_labels


def collect_gene_samples(
    client: OpenAI,
    personas: list[PersonaSpec],
    gene: str,
    gene_index: int,
    num_samples: int,
    disabled_models: set[str],
) -> list[Sample]:
    samples: list[Sample] = []
    for spec in personas:
        if spec.model in disabled_models:
            continue
        for _ in range(num_samples):
            try:
                completion = sample_completion(client, spec, gene)
            except ModelFailure as exc:
                marker = "timeout" if exc.reason == "timeout" else "error"
                log_completion_attempt(exc.model, gene_index, marker, marker)
                if exc.model not in disabled_models:
                    disabled_models.add(exc.model)
                break
            log_completion_attempt(
                spec.model,
                gene_index,
                str(completion.bytes_out),
                str(completion.latency_ms),
            )
            samples.append(
                Sample(
                    model=spec.model,
                    persona_file=spec.file_ref,
                    gene_index=gene_index,
                    text=completion.text,
                )
            )
    return samples


def collect_gene_rows(
    personas: list[PersonaSpec],
    gene: str,
    gene_index: int,
    num_samples: int,
    gene_dim: int,
    timeout_seconds: float,
    xproxy_port: int,
) -> list[ClusteredSample]:
    responses_client = build_xproxy_client(xproxy_port, timeout_seconds)
    embeddings_client = build_embeddings_client(timeout_seconds)
    disabled_models: set[str] = set()
    gene_samples = collect_gene_samples(
        responses_client,
        personas,
        gene,
        gene_index,
        num_samples,
        disabled_models,
    )
    if not gene_samples:
        return []
    embedded_samples: list[Sample] = []
    vectors: list[np.ndarray] = []
    for sample in gene_samples:
        try:
            vector = embed_text(embeddings_client, sample.text)
        except Exception as exc:
            log_gene_error(
                gene_index,
                f"embedding failed for {sample.model},{sample.persona_file}: {exc}",
            )
            continue
        embedded_samples.append(sample)
        vectors.append(vector)
    if not embedded_samples:
        return []
    embeddings = np.vstack(vectors)
    reduced = reduce_gene_vectors(embeddings, gene_dim)
    labels = cluster_gene_vectors(reduced)
    return [
        ClusteredSample(
            model=sample.model,
            persona_file=sample.persona_file,
            gene_index=sample.gene_index,
            coords=tuple(float(value) for value in coords_row),
            cluster_num=int(label),
        )
        for sample, coords_row, label in zip(
            embedded_samples,
            reduced,
            labels,
            strict=True,
        )
    ]


def write_cluster_csv(rows: list[ClusteredSample]) -> None:
    writer = csv.writer(sys.stdout, lineterminator="\n")
    writer.writerows(
        [
            (row.model, row.persona_file, row.gene_index, row.cluster_num)
            for row in rows
        ]
    )


def write_pca_csv(path_text: str, rows: list[ClusteredSample]) -> None:
    if path_text == "":
        return
    path = resolve_path(path_text)
    path.parent.mkdir(parents=True, exist_ok=True)
    with path.open("w", encoding="utf-8", newline="") as f:
        writer = csv.writer(f, lineterminator="\n")
        for row in rows:
            writer.writerow(
                [
                    row.model,
                    row.persona_file,
                    row.gene_index,
                    *[f"{value:.12g}" for value in row.coords],
                    row.cluster_num,
                ]
            )


def main(argv: list[str]) -> int:
    args = parse_args(argv)
    if args.num_samples <= 0:
        raise SystemExit("--num-samples must be > 0")
    if args.gene_dim <= 0:
        raise SystemExit("--gene-dim must be > 0")
    num_genes = resolve_num_genes(args.num_genes)
    num_personas = resolve_num_personas(args.num_personas)
    personas = load_personas(args.personas_file)
    selected_personas = select_personas(personas, num_personas)
    genes = load_genes(args.genes_file)
    selected_genes = select_genes(genes, num_genes)
    timeout_seconds = resolve_timeout_seconds()
    xproxy_port = resolve_xproxy_port()
    ensure_xproxy_healthy(xproxy_port, timeout_seconds)
    rows: list[ClusteredSample] = []
    gene_rows: dict[int, list[ClusteredSample]] = {}
    with concurrent.futures.ThreadPoolExecutor(max_workers=len(selected_genes)) as executor:
        future_map = {
            executor.submit(
                collect_gene_rows,
                selected_personas,
                gene,
                gene_index,
                args.num_samples,
                args.gene_dim,
                timeout_seconds,
                xproxy_port,
            ): gene_index
            for gene_index, gene in selected_genes
        }
        for future in concurrent.futures.as_completed(future_map):
            gene_index = future_map[future]
            try:
                gene_rows[gene_index] = future.result()
            except Exception as exc:
                log_gene_error(gene_index, f"worker failed: {exc}")
                gene_rows[gene_index] = []
    for gene_index, _ in selected_genes:
        rows.extend(gene_rows.get(gene_index, []))
    write_pca_csv(args.pca_out.strip(), rows)
    write_cluster_csv(rows)
    return 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv[1:]))
