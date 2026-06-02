#!/usr/bin/env python3
"""Conservatively prefilter provider model metadata before live tool probing."""

from __future__ import annotations

import argparse
import csv
import json
import re
import sys
from pathlib import Path
from typing import Any

NO_TEXT_TASK_RE = re.compile(
    r"\b(embedding|embed|rerank|moderation|text[-_ ]?to[-_ ]?speech|tts|speech[-_ ]?to[-_ ]?text|asr|transcri(?:be|ption)|whisper|video generation|image generation|diffusion)\b",
    re.IGNORECASE,
)
OCR_RE = re.compile(r"\bocr\b", re.IGNORECASE)
TOOL_PARAMS = {"tools", "tool_choice"}


def text_list(value: Any) -> list[str] | None:
    if value is None:
        return None
    if not isinstance(value, list):
        return None
    return [str(item).strip().lower() for item in value if str(item).strip()]


def joined_model_text(model: dict[str, Any]) -> str:
    fields = [model.get("id"), model.get("name"), model.get("description")]
    return " ".join(str(field) for field in fields if field)


def evidence_value(value: Any) -> str:
    if value is None:
        return "unknown"
    if isinstance(value, list):
        return "+".join(str(item) for item in value)
    return str(value)


def decide(model: dict[str, Any]) -> tuple[str, str, str]:
    architecture = model.get("architecture")
    if not isinstance(architecture, dict):
        architecture = {}

    input_modalities = text_list(architecture.get("input_modalities"))
    output_modalities = text_list(architecture.get("output_modalities"))
    modality = architecture.get("modality")
    supported_parameters = text_list(model.get("supported_parameters"))
    model_text = joined_model_text(model)

    if output_modalities is not None and "text" not in output_modalities:
        return "skip", "no_text_output", "output_modalities=" + evidence_value(output_modalities)
    if input_modalities is not None and "text" not in input_modalities:
        return "skip", "no_text_input", "input_modalities=" + evidence_value(input_modalities)

    if supported_parameters is not None and TOOL_PARAMS.isdisjoint(supported_parameters):
        return "skip", "no_tool_support_metadata", "supported_parameters=" + evidence_value(supported_parameters)

    task_match = NO_TEXT_TASK_RE.search(model_text)
    if task_match and supported_parameters is not None and TOOL_PARAMS.isdisjoint(supported_parameters):
        return "skip", "special_task_no_tool_support", "term=" + task_match.group(0)
    if task_match and output_modalities is not None and "text" not in output_modalities:
        return "skip", "special_task_no_text_output", "term=" + task_match.group(0)

    ocr_match = OCR_RE.search(model_text)
    if ocr_match and supported_parameters is not None and TOOL_PARAMS.isdisjoint(supported_parameters):
        return "skip", "ocr_no_tool_support", "term=" + ocr_match.group(0)

    if supported_parameters is None:
        return "keep", "unknown_tool_metadata", "probe_required"
    if input_modalities is None or output_modalities is None:
        return "keep", "unknown_modality_metadata", "probe_required"

    return "keep", "metadata_allows_probe", "modality=" + evidence_value(modality)


def load_models(path: Path) -> list[dict[str, Any]]:
    payload = json.loads(path.read_text())
    data = payload.get("data") if isinstance(payload, dict) else payload
    if not isinstance(data, list):
        raise ValueError("model metadata JSON must be a list or an object with a data list")
    models = [item for item in data if isinstance(item, dict)]
    if len(models) != len(data):
        raise ValueError("model metadata contains non-object entries")
    return models


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--metadata", required=True, type=Path, help="Provider model metadata JSON")
    parser.add_argument("--out", required=True, type=Path, help="Output file containing kept runtime model ids")
    parser.add_argument("--decisions", required=True, type=Path, help="CSV audit log of keep/skip decisions")
    parser.add_argument("--prefix", default="openrouter://", help="Runtime model id prefix")
    args = parser.parse_args()

    models = load_models(args.metadata)
    args.out.parent.mkdir(parents=True, exist_ok=True)
    args.decisions.parent.mkdir(parents=True, exist_ok=True)

    kept: list[str] = []
    with args.decisions.open("w", newline="") as decision_file:
        writer = csv.writer(decision_file)
        writer.writerow(["model", "decision", "reason", "evidence"])
        for model in models:
            model_id = str(model.get("id", "")).strip()
            if not model_id:
                continue
            decision, reason, evidence = decide(model)
            runtime_model = args.prefix + model_id
            writer.writerow([runtime_model, decision, reason, evidence])
            if decision == "keep":
                kept.append(runtime_model)

    args.out.write_text("".join(model + "\n" for model in kept))
    print(f"kept={len(kept)} skipped={len(models) - len(kept)} total={len(models)}", file=sys.stderr)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
