# Review Notes

## 2026-03-24

This review covered the Lean engine, the Go runner, the policy path, and the current docs.

| Severity | Area | Observation |
|---|---|---|
| High | complaint parsing | The complaint parser accepts old complaint files that still include a `Standard of Evidence` section and silently ignores that section.  A user can therefore believe the complaint controls the burden while the case actually runs under policy. |
| Medium | Lean policy enforcement | The Lean engine carries `max_report_title_bytes` and `max_report_summary_bytes`, but does not enforce them when it accepts technical reports.  The Go runner blocks oversize reports, but a direct engine caller can still store them in state. |
| Medium | Lean vote enforcement | The Lean engine accepts blank council rationales.  The Go council path requires a rationale, but the engine itself does not. |
| Medium | docs | The main docs had drifted after the move of `evidence_standard` into policy and after the move from simple majority to configurable `required_votes_for_decision`.  `ARAP.md`, `practice.md`, and `README.md` described the older procedure. |
