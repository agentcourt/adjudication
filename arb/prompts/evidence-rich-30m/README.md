# Evidence-rich attorney prompts

Alternative attorney prompts for open-record, fact-intensive arbitrations with longer attorney time limits.

Use with explicit longer limits, for example:

```bash
export AAR_OPENCLAW_ATTORNEY_TIMEOUT_SECONDS=1800
.bin/aar case \
  --prompt-dir prompts/evidence-rich-30m \
  --acp-timeout-seconds 1800 \
  ...
```

These prompts preserve the same evidence-discipline rules as the default prompts, but tell attorneys to use the larger budget for staged source retrieval, source-chain reconstruction, transcript/OCR work, metadata checks, authenticity checks, private journals, optional question queues, and richer evidence-search ledgers. Missing prompt files fall back to `./prompts`.

## Private journals and question queues

The prompts are check-in safe and do not name local machines, private local files, helper scripts, or media tools. They refer generically to any private work root or question queue supplied by the runtime, work-product section, environment, or operator instructions.

A supervised run may provide a private work root containing files such as:

```text
<private-work-root>/<role>/journal.md
<private-work-root>/<role>/questions.md
<private-work-root>/<role>/answers.md
```

The attorney instructions treat those files as private work product outside the AAR record. Attorneys may use them to plan, ask the supervisor for suggestions, record search paths, and preserve self-audits. They must not submit, cite, quote at length, offer, or attach those files in the AAR record. Public source material discovered through the private work area must still be submitted through `aar_submit_evidence` before it can support a filing.

A supervisor who monitors the queue should answer by question id, keep answers bounded and evidence-oriented, avoid private auth or account material, and remind attorneys to preserve public source artifacts before relying on them.

## Evidence-harvest emphasis

For social, video, audio, screenshot, article-embed, or repost evidence, attorneys are instructed to reconstruct the source chain rather than rely on captions or snippets. They should preserve canonical URLs or IDs, author and timestamp metadata, attached-media metadata when visible, page or media captures, transcripts, OCR, frame observations, hashes when available, archive/cache/mirror checks, and capture-failure ledgers. If a short clip may be misleading, they should attempt to preserve the fuller source context.
