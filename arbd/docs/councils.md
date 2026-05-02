# Council Constitution

`arbd` constitutes its deciding body during `aard case`.  The procedure uses a council.  The code and the run artifacts use that term throughout.  Council constitution begins after the complaint has been parsed and the case policy has been loaded, and it finishes before the Lean engine accepts the case and opens the first attorney turn.

The runtime draws council members from a pool file, converts that draw into Lean input, and then asks the Lean engine to initialize the case with that exact list.  The engine performs a second set of checks on the incoming council and rewrites each member into a seated state before the case becomes active.  Once initialization succeeds, the constituted council is recorded immediately in the event stream and later in the final artifacts.

| Stage | Code path | Effect |
|---|---|---|
| CLI configuration | [case CLI](../runtime/cli/case.go) | Loads policy, applies `--council-size` and `--council-pool`, and builds `runner.Config`. |
| Pool loading | [persona loader](../../common/persona/persona.go) | Reads the pool file, validates model ids, resolves persona files, and loads persona text. |
| Sampling | [runner helpers](../runtime/runner/helpers.go) | Draws `council_size` records without replacement and assigns seat ids in draw order. |
| Engine initialization | [Lean engine](../engine/Main.lean) | Requires exact council length, requires unique member ids, rewrites all members to `seated`, and opens the case. |
| Recording | [main runner](../runtime/runner/run.go) and [renderer](../runtime/runner/render.go) | Writes the constituted council into the initialization event and the final run artifacts. |
| Deliberation order | [Lean engine](../engine/Main.lean) | Selects the first seated member who has not yet answered in the current round. |

## Entry Point

`aard case` loads the complaint, resolves the shared `common` tree, loads the arbitration policy, and applies the explicit CLI overrides before it touches the council pool.  The policy controls council size, so that value must be fixed before sampling begins.  The default policy in [the runtime policy layer](../runtime/runner/policy.go) and [the repository policy file](../etc/policy.json) sets `council_size` to `5`.

The runner accepts `--council-size` as a direct override, and then validates the resulting policy before it starts the run.  [Policy validation](../runtime/runner/policy.go) requires a positive council size and a non-empty judgment standard.  That validation determines the shape of the deciding body before the runtime reads a single council record from the pool.

## Pool File

The pool file comes from `--council-pool` when the caller supplies it.  Otherwise the CLI uses [the shared default pool](../../common/data/personas/pool.csv).  That file is a flat list of `MODEL,PERSONA_FILE` records, one per line, with blank lines and `#` comments ignored by [the persona loader](../../common/persona/persona.go).

Each usable line becomes one independent sampleable record.  The loader validates the model id with xproxy parsing, resolves the persona filename relative to the pool file, reads the persona text immediately, and requires that text to be non-empty.  If the pool file repeats a line, the runtime treats each repeated line as a separate entry in the sampling pool, because the loader preserves record multiplicity rather than collapsing identical records.

## Sampling

[The council sampler](../runtime/runner/helpers.go) receives the parsed pool and the already-validated `council_size`.  It requires `council_size <= len(specs)`, builds an index list over the pool records, and draws from that index list with `crypto/rand`.  Each successful draw removes one index from the remaining set, so sampling proceeds without replacement across the pool records for that run.

The draw order determines the seat ids.  The first sampled record becomes `C1`, the second becomes `C2`, and the sequence continues until the runtime has drawn `council_size` records.  Each drawn seat carries four runtime values at this stage: the synthetic `member_id`, the selected model id, the persona filename, and the loaded persona text.

The runtime keeps the persona text for prompting, but the public council metadata carries only the seat id, model id, and persona filename.  That separation appears directly in [the council seat type](../runtime/runner/types.go), where `PersonaText` is excluded from JSON output.  The draw therefore produces both the public description of the council and the private prompt material that the council runtime will later feed to each model.

## Lean Initialization

After sampling, the runtime converts the drawn seats into Lean input with [the council mapper](../runtime/runner/helpers.go).  Each mapped entry includes `member_id`, `model`, `persona_filename`, and `status`, with `status` set to `seated` before the request is sent.  The Go bridge in [the Lean engine wrapper](../runtime/lean/engine.go) packages that list into an `initialize_case` request together with the question and the current policy state.

[The Lean initializer](../engine/Main.lean) then performs its own constitution checks.  It requires a non-empty council, requires the incoming list length to match `policy.council_size`, and requires unique `member_id` values.  When those checks pass, it rewrites every incoming member to `status := "seated"`, resets `deliberation_round` to `1`, clears `council_answers`, sets case status to `active`, and moves case phase to `openings`.

That second initialization step matters because it fixes the authoritative council state inside the engine.  The Go runtime may have sampled and labeled the seats, but the Lean state becomes the source of truth for who is seated and which round is current.  From that point forward, attorney turns, council turns, answer recording, and removal all operate against the initialized Lean state rather than against the original pool file or the pre-init draw structure.

## Recording

The constituted council is recorded as soon as initialization succeeds.  [The main runner](../runtime/runner/run.go) appends a `run_initialized` event that includes the complaint, the judgment standard, the attorney model configuration, and the full council list.  That event is the first durable record of which members were seated in that run.

The same council list appears again in the completion artifacts written by [the renderer](../runtime/runner/render.go).  `run.json` carries the council in the top-level `council` field, and `council.json` writes the same list on its own.  Those artifacts preserve the constituted body exactly as it existed at initialization, which means the council metadata is not reconstructed later from the answer log.

## Answer Order

When the case reaches deliberation, the Lean engine chooses the next answering member from the initialized `council_members` list.  [The selection function](../engine/Main.lean) filters that list to members whose status remains `seated` and then chooses the first seated member who has not yet answered in the current round.  Because the underlying list order is the original sampling order, the first round calls members in sampled seat order: `C1`, then `C2`, then `C3`, and so on.

The current `arbd` design closes after one complete round.  [The deliberation continuation rule](../engine/Main.lean) compares the current-round answer count to the number of seated members, and when every seated member has answered, it closes the case without any aggregation step.  The final result is therefore the set of member answers, not a derived council-wide number.

## Status Changes

The council can shrink during deliberation through explicit status change.  The current runtime path for that change is timeout handling in [the council runtime](../runtime/runner/council.go), which calls Lean `remove_council_member` with a new status such as `timed_out`.  Lean permits that transition only during deliberation, only for a known seated member, and only before that member has answered in the current round.

After removal, the member remains present in `council_members`, but the member no longer counts as seated.  Later calls to the next-member selector therefore skip that seat, and the closure rule uses the smaller seated body.  The runtime records that change as a `council_member_removed` event, so the event stream shows both the originally constituted council and the later status transition.

Another council failure path leaves the council composition untouched and ends the run.  If a council model keeps returning malformed or invalid answers until it exceeds the runtime invalid-attempt limit, [the council executor](../runtime/runner/council.go) returns an error and the run stops.  In that case the constituted council remains the same body that initialization created, and the failure appears as a run error rather than as a change in council membership.
