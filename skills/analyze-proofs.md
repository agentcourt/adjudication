# Analyze Proofs

Run the proof-analysis tools from the project directory you want to update, not from the repository root.  In this repository that means `adc/` for the district-court system or `arb/` for arbitration.  The shared tools live in `../common/tools/`, but they derive their default inputs and outputs from the current working directory.

For either project, change into the project directory and run `../common/tools/proofstats.sh`, then run `uv run --script ../common/tools/gentheorems.py`.  The proof-stats script reads `engine/Proofs/*.lean` and rewrites `docs/proofstats.md`.  The theorem tool sorts `theorems.tsv` and rewrites `docs/theorems.md`.

The default path assumptions are deliberate.  `proofstats.sh` detects the proof profile from the files under `engine/Proofs`, so it uses the arbitration categories when run from `arb/` and the district-court categories when run from `adc/`.  `gentheorems.py` uses `theorems.tsv` and `docs/theorems.md` under the current project unless you pass explicit paths.

After the run, inspect the regenerated files rather than trusting the command exit status alone.  Use `git status --short -- docs/proofstats.md docs/theorems.md theorems.tsv` to see what changed, then open the updated Markdown files and confirm that the counts, categories, and theorem table still make sense.  If a project ever needs a different output path, pass it explicitly to the shared tool instead of editing the tool for one project.

For `adc/`, the exact commands are these:

```bash
cd /home/somebody/src/adjudication/adc
../common/tools/proofstats.sh
uv run --script ../common/tools/gentheorems.py
git status --short -- docs/proofstats.md docs/theorems.md theorems.tsv
```

For `arb/`, the exact commands are these:

```bash
cd /home/somebody/src/adjudication/arb
../common/tools/proofstats.sh
uv run --script ../common/tools/gentheorems.py
git status --short -- docs/proofstats.md docs/theorems.md theorems.tsv
```
