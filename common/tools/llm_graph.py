#!/usr/bin/env -S uv run --script
# /// script
# requires-python = ">=3.11"
# dependencies = [
#   "matplotlib>=3.9",
# ]
# ///

from __future__ import annotations

import argparse
import re
from dataclasses import dataclass
from datetime import datetime
from pathlib import Path

AGENT_CALL_RE = re.compile(r"^agent call turn=\d+ .* phase=([a-z_]+) ")
REPO_ROOT = Path(__file__).resolve().parents[2]


@dataclass(frozen=True)
class Sample:
    ts: datetime
    phase: str
    model: str
    bytes_in: int
    bytes_out: int
    elapsed_ms: int


PHASE_STYLE = {
    "setup": "#6b7280",
    "pretrial": "#2563eb",
    "jury_selection": "#7c3aed",
    "trial": "#dc2626",
    "instructions": "#d97706",
    "deliberation": "#059669",
    "post_verdict": "#111827",
}


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser()
    parser.add_argument(
        "--log",
        required=True,
        help="Path to the xproxy log with llm_csv lines.",
    )
    parser.add_argument(
        "--out",
        default=str(REPO_ROOT / "adc/llm.png"),
        help="Output PNG path.",
    )
    return parser.parse_args()


def parse_samples(log_path: Path) -> list[Sample]:
    current_phase = "setup"
    samples: list[Sample] = []
    for raw_line in log_path.read_text().splitlines():
        line = raw_line.strip()
        match = AGENT_CALL_RE.match(line)
        if match:
            current_phase = phase_group(match.group(1))
            continue
        if line.startswith("autopilot stop"):
            current_phase = "setup"
            continue
        if not line.startswith("llm_csv,"):
            continue
        parts = line.split(",")
        if len(parts) != 6:
            continue
        _, ts_s, model, bytes_in_s, bytes_out_s, elapsed_ms_s = parts
        samples.append(
            Sample(
                ts=datetime.strptime(ts_s, "%Y-%m-%d %H:%M:%S.%f"),
                phase=current_phase,
                model=model,
                bytes_in=int(bytes_in_s),
                bytes_out=int(bytes_out_s),
                elapsed_ms=int(elapsed_ms_s),
            )
        )
    return samples


def phase_group(raw_phase: str) -> str:
    if raw_phase == "pretrial":
        return "pretrial"
    if raw_phase == "voir_dire":
        return "jury_selection"
    if raw_phase in {
        "openings",
        "plaintiff_case",
        "plaintiff_evidence",
        "defense_case",
        "defense_evidence",
        "plaintiff_rebuttal",
        "plaintiff_rebuttal_evidence",
        "defense_surrebuttal",
        "defense_surrebuttal_evidence",
        "closings",
    }:
        return "trial"
    if raw_phase in {"charge_conference", "jury_charge"}:
        return "instructions"
    if raw_phase == "deliberation":
        return "deliberation"
    if raw_phase == "post_verdict":
        return "post_verdict"
    return "setup"


def plot_samples(samples: list[Sample], out_path: Path) -> None:
    import matplotlib.dates as mdates
    import matplotlib.pyplot as plt

    fig, ax = plt.subplots(figsize=(14, 8), constrained_layout=True)
    ax.set_title(f"LLM request latency over time ({len(samples)} requests)")
    ax.set_xlabel("Time")
    ax.set_ylabel("Milliseconds")
    ax.grid(True, axis="both", color="#d1d5db", linewidth=0.6, alpha=0.6)
    ax.set_axisbelow(True)
    for phase, color in PHASE_STYLE.items():
        phase_samples = [sample for sample in samples if sample.phase == phase]
        if not phase_samples:
            continue
        ax.scatter(
            [sample.ts for sample in phase_samples],
            [sample.elapsed_ms for sample in phase_samples],
            label=phase,
            c=color,
            marker="o",
            s=54,
            alpha=0.85,
            edgecolors="none",
        )
    max_latency = max(sample.elapsed_ms for sample in samples)
    top = max(1, int(max_latency * 1.05))
    ax.set_ylim(0, top)

    ax.xaxis.set_major_formatter(mdates.DateFormatter("%H:%M:%S"))
    fig.autofmt_xdate(rotation=30, ha="right")
    ax.legend(title="Phase", frameon=False)
    fig.savefig(out_path, dpi=180)
    plt.close(fig)


def main() -> int:
    args = parse_args()
    log_path = Path(args.log)
    out_path = Path(args.out)
    samples = parse_samples(log_path)
    if not samples:
        raise SystemExit(f"no llm_csv samples found in {log_path}")
    mean_latency = sum(sample.elapsed_ms for sample in samples) / len(samples)
    max_latency = max(sample.elapsed_ms for sample in samples)
    plot_samples(samples, out_path)
    print(f"points={len(samples)}")
    print(f"max_latency_ms={max_latency}")
    print(f"mean_latency_ms={mean_latency:.1f}")
    print(out_path)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
