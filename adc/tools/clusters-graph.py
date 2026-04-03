#!/usr/bin/env -S uv run --script
# /// script
# requires-python = ">=3.11"
# dependencies = [
#   "matplotlib>=3.9",
# ]
# ///

from __future__ import annotations

import argparse
import csv
from dataclasses import dataclass
from pathlib import Path

import matplotlib.pyplot as plt
from matplotlib.lines import Line2D


@dataclass(frozen=True)
class Point:
    model: str
    source: str
    gene: int
    x1: float
    x2: float
    cluster: int


MARKERS = ["o", "s", "^", "D", "P", "X", "v", "<", ">", "h", "8", "p"]


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser()
    parser.add_argument(
        "--pca",
        default="etc/personas-pca.csv",
        help="Path to the PCA CSV file.",
    )
    parser.add_argument(
        "--out",
        default="clusters.png",
        help="Output PNG path.",
    )
    return parser.parse_args()


def load_points(path: Path) -> list[Point]:
    points: list[Point] = []
    with path.open(encoding="utf-8", newline="") as f:
        reader = csv.reader(f)
        for row_num, row in enumerate(reader, start=1):
            if len(row) < 6:
                raise SystemExit(f"{path}:{row_num}: expected at least 6 columns")
            try:
                model = row[0]
                gene = int(row[2])
                x1 = float(row[3])
                x2 = float(row[4])
                cluster = int(row[-1])
            except ValueError as exc:
                raise SystemExit(f"{path}:{row_num}: {exc}") from exc
            points.append(
                Point(
                    model=model,
                    source=model_source(model),
                    gene=gene,
                    x1=x1,
                    x2=x2,
                    cluster=cluster,
                )
            )
    if not points:
        raise SystemExit(f"no PCA rows found in {path}")
    return points


def model_source(model: str) -> str:
    rest = model.split("://", 1)[1] if "://" in model else model
    return rest.split("/", 1)[0]


def model_name(model: str) -> str:
    rest = model.split("://", 1)[1] if "://" in model else model
    parts = rest.split("/", 1)
    if len(parts) == 1:
        return parts[0]
    return parts[1]


def cluster_colors(points: list[Point]) -> dict[int, tuple[float, float, float, float]]:
    clusters = sorted({point.cluster for point in points})
    cmap = plt.get_cmap("tab20", max(len(clusters), 1))
    return {cluster: cmap(index) for index, cluster in enumerate(clusters)}


def source_model_markers(points: list[Point]) -> dict[str, dict[str, str]]:
    by_source: dict[str, dict[str, str]] = {}
    sources = sorted({point.source for point in points})
    return {
        source: {
            model: MARKERS[index % len(MARKERS)]
            for index, model in enumerate(
                sorted({point.model for point in points if point.source == source})
            )
        }
        for source in sources
    }


def plot_points(points: list[Point], out_path: Path) -> None:
    genes = sorted({point.gene for point in points})
    sources = sorted({point.source for point in points})
    colors = cluster_colors(points)
    markers = source_model_markers(points)
    cols = len(genes)
    rows = len(sources)
    fig, axes = plt.subplots(
        rows,
        cols + 1,
        figsize=(4.2 * cols + 3.0, 3.2 * rows),
        constrained_layout=True,
        squeeze=False,
        gridspec_kw={"width_ratios": [1] * cols + [0.72]},
    )

    for row_index, source in enumerate(sources):
        for col_index, gene in enumerate(genes):
            ax = axes[row_index, col_index]
            facet_points = [
                point
                for point in points
                if point.source == source and point.gene == gene
            ]
            combos = sorted(
                {(point.model, point.cluster) for point in facet_points},
                key=lambda value: (value[1], value[0]),
            )
            for model, cluster in combos:
                cluster_points = [
                    point
                    for point in facet_points
                    if point.model == model and point.cluster == cluster
                ]
                ax.scatter(
                    [point.x1 for point in cluster_points],
                    [point.x2 for point in cluster_points],
                    s=22,
                    alpha=0.9,
                    color=colors[cluster],
                    marker=markers[source][model],
                    edgecolors="none",
                )
            if row_index == 0:
                ax.set_title(f"Gene {gene}")
            if col_index == 0:
                ax.set_ylabel(f"{source}\nX2")
            else:
                ax.set_ylabel("X2")
            if row_index == rows - 1:
                ax.set_xlabel("X1")
            else:
                ax.set_xlabel("")
            ax.grid(True, color="#d1d5db", linewidth=0.6, alpha=0.6)
            ax.set_axisbelow(True)

        legend_ax = axes[row_index, cols]
        legend_ax.axis("off")
        source_handles = [
            Line2D(
                [0],
                [0],
                marker=marker,
                linestyle="",
                markersize=6,
                markerfacecolor="#6b7280",
                markeredgewidth=0,
                color="#6b7280",
                label=model_name(model),
            )
            for model, marker in sorted(markers[source].items())
        ]
        legend_ax.legend(
            handles=source_handles,
            title=source,
            loc="center left",
            frameon=False,
        )
    fig.savefig(out_path, dpi=180)
    plt.close(fig)


def main() -> int:
    args = parse_args()
    pca_path = Path(args.pca)
    out_path = Path(args.out)
    points = load_points(pca_path)
    plot_points(points, out_path)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
