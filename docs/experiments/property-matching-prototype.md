# Property matching prototype

Status: the user reviewed and accepted the plan and selected the local stack playbook. The standalone prototype and independent review are complete; see the [results](property-matching-results.md). Broader cleanup and application adoption retain their separate scope approvals.

Branch: `refactor/property-matching-explanations`. Baseline: `7c7012072fc37476a2806b9776771a3bf58d4110`, equal to the locally available `origin/dev` when the branch was created. [Decision trail](property-matching-decisions.tsv).

The team needs a working comparison of the current buyer ranking and a design that retains criterion assessments, computes an auditable preference score, and explains the actual ordering concisely. Adopting the design depends on the comparison evidence, not on completing the prototype.

## Agreed behavior

- Published numeric and location constraints remain mandatory gates. A wrong neighborhood cannot earn its way into exact results through other matches.
- Requirements and preferences remain distinct. An explicitly mandatory qualitative requirement is not silently converted into a preference because evidence is missing.
- Supported mandatory qualities pass. Explicit contradictions fail. Unknown cases may appear as clearly identified alternatives, after confirmed matches.
- Supported preferences contribute positively; contradictions negatively. Unknown evidence and inferred hints contribute zero. Retain the evidence and its provenance for every assessment.
- A preference has weight 1 by default, or 2 when the searcher explicitly prioritizes it. Retain and validate the source phrase. Do not infer priorities from the criterion type or model confidence.
- Compute `(supported weights - contradicted weights) / all requested preference weights`. Unknown preferences remain in the denominator. A search with no preferences has score zero and an explicit indication that there is no preference-based advantage.
- Scores are internal and do not appear in user prose as percentages or probabilities.
- Group confirmed mandatory matches before unconfirmed alternatives. Within each group, order eligible, conditional, and unknown rental eligibility separately. Hide ineligible listings. Use descending preference score within these groups.
- Explicit price or area sorting replaces score ordering within the groups. Preserve missing-value behavior and deterministic ties. A stable tie does not imply a substantive property advantage.
- Explain consecutive ranks using the actual deciding ordering rule and source-backed facts. A lone result receives a fit explanation without an invented comparison.
- Preserve branch-specific criteria. Scores compare coverage of each branch's requested preferences, not evidence that differing branches share an identical rubric.

## Prototype architecture

The flow is fixed structured search -> candidate retrieval -> criterion assessment -> eligibility -> score and grouping -> recorded ranking reasons -> explanation -> comparison report.

The reusable matching operation belongs in `internal/matching`. It reuses the existing classifier and evidence types but has its own request and result for the proposed fit policy. Existing `Evaluator.Evaluate` behavior and its callers remain intact during the experiment.

Two shapes were considered against the same prototype behavior:

| Shape | Benefit | Tradeoff |
| --- | --- | --- |
| Fit operation in `internal/matching`, orchestration in the experiment | Retains assessment ownership and existing provider substitution; useful for later integration | Adds a distinct experimental operation beside the existing ranking policy |
| All proposed assessment and scoring in the experiment | Limits reusable-package changes during exploration | Mixes orchestration with matching ownership and requires moving that implementation during integration |

Use the first shape, with a small interface and no new provider abstraction. Keep retrieval, rental eligibility, presentation grouping, and baseline comparison outside the preference scorer. The scorer cannot change which neighborhood the user requested or decide rental eligibility.

The proposed request identifies criteria, their requirement or preference strength, and any explicit priority source phrase. The result retains each criterion outcome, evaluation status, evidence references, contribution, aggregate score, and missing evidence. Candidate-level ordering records identify the compared ranks and the first deciding rule.

Priority metadata lives in the prototype request. Fixed examples supply the structured search and traceable priorities. This does not claim that the current conversational planner extracts preference weights; that integration is outside this comparison.

Missing, conflicting, and unavailable assessments remain distinguishable. Conflicting or unavailable evidence earns no support points and cannot confirm a mandatory quality. Provider failure must not become a false claim that the listing omitted information. Preserve the existing publication gate for inconsistent listings.

## Scope and exclusions

Implement a standalone comparison command under `experiments/`, with focused reusable matching code, fixtures, tests, benchmarks, and a team-readable report. It compares the real current buyer policy with the proposed policy on the same structured queries, candidates, qualifications, and recorded evidence. The existing experimental `matching.Evaluate` policy is not a substitute for the current buyer baseline.

Reuse the frozen listing and eligibility snapshots, current intake shapes, provider adapters, and published-field semantics. Add direct support and contradiction rubrics for light and quietness where the prototype evaluates prose. Existing exact attributes continue to provide deterministic evidence where appropriate. Inferred attributes alone do not establish supported qualities.

The application path, frontend, public HTTP contracts, database schema, scraping, and committed model-derived snapshots stay outside implementation scope. Do not change live planner behavior or replace the existing buyer writer. The prototype has its own concise explanation instructions and deterministic fallback.

Nearby suggestions are future work. Keep retrieval separate from scoring so future candidates can preserve the requested location and explicitly identify a deviation. Include a synthetic boundary example showing that a nearby candidate cannot pass an exact-neighborhood gate. No geocoder, coordinates, distance estimates, or proximity retrieval are implemented now.

## Explanation behavior

Use plain Rioplatense Spanish. Target one short overview and two or three short sentences per explained property, covering at most the existing three-property writer shortlist. Keep necessary eligibility conditions and unknowns beside the relevant rank. Mention each warning once. Avoid mandatory section headings, repeated card facts, unrelated amenities, and technical score terminology.

The original example could read as follows. This is a style example based on the user's pasted explanation, not a newly verified search result:

> Encontré dos opciones para mostrarte. La primera queda arriba porque el aviso acepta la garantía que tenés; en la segunda falta confirmar ese punto.
>
> **#1, Migueletes al 900.** El aviso acepta garantía propietaria de CABA o Finaer. Cumple con los dos ambientes, la zona y el presupuesto de tu búsqueda.
>
> **#2, Sinclair al 3100.** También cumple esos filtros, pero no publica qué garantías acepta. Consultalo con la inmobiliaria antes de avanzar.
>
> Hay otras dos propiedades excluidas por garantía. Si conseguís una caución, vuelven a aparecer.

Two local development trials showed that free rewriting could reverse a ranking fact even after internal data was removed. The prototype now lets the LLM return references to backend-approved explanation blocks only. The backend validates coverage and order and renders exact text; invalid references use the deterministic fallback. This is a fidelity repair within the explanation boundary, not a change to the ranking policy. The comparison must identify this restriction and assess whether retaining the LLM adds value.

Other examples must explain the score difference when score actually decides the order. Required-property uncertainty, eligibility, and explicit sort can decide earlier. Preserve exact counts, empty branches, ranks, listing-claim versus inference distinctions, and relaxation facts. A targeted listing question is answered directly without a ranking recap. Fallback wording follows the same evidence and brevity rules.

## Examples and benchmarks

Freeze independently specified examples before tuning. Include real snapshot rows plus synthetic controls for supported preferences, contradictions, missing evidence, hints, explicit priorities, mandatory-quality alternatives, all eligibility states, multiple branches, missing numeric values, explicit sorts, ties, no preferences, a lone result, empty results, provider failure, and a nearby deviation. Keep a small held-out subset for final comparison. Record snapshot hashes, baseline revision, case IDs, policies, machine details, and actual commands.

Freeze classifier assessments when comparing ranking policies so model variation cannot masquerade as a scoring improvement. Report the new light and noise assessment behavior separately from the scoring-policy comparison. Recorded or simulated assessments establish downstream behavior, not live classifier accuracy.

Report these dimensions separately:

- Mandatory-gate violations, eligibility leakage, expected ordering, and each order change with its cause.
- Explanation fidelity to the recorded ordering and evidence, unsupported claims, omitted conditions, repeated warnings, and word counts.
- Backend time per operation and allocations on the same machine, with repeated baseline and proposed runs.
- Actual writer latency and generated outputs when a local model is available. Report token use and cost only if observed; do not label unavailable telemetry as zero cost.

Offline checks cover matching policy, contribution accounting, evidence retention, grouping, ties, explanation packets, and deterministic fallback. They do not prove model interpretation or generated prose quality. The default local endpoint at `127.0.0.1:8000` was unavailable during brief preparation. Live prose verification remains required before claiming the explanation-quality criteria passed. Paid provider evaluation is outside this brief.

The current `BenchmarkTurn` stubs planner and writer and uses a neighborhood-only inventory. It is a compute baseline, not proof of Postgres filters or end-to-end quality. The new comparison must identify its retrieval boundary honestly and protect gates with independent literal cases. Existing relevant store tests protect SQL behavior. No database mutation is needed for the standalone comparison.

## Acceptance and stopping condition

Each independently labeled deterministic case passes for the proposed path. Every score can be reconstructed from its reported contributions and denominator. The proposed path introduces no mandatory contradiction into exact matches, known ineligibility into shown results, unsupported quality claim, incorrect rank reference, or fabricated tie advantage. Baseline defects remain visible as comparison findings.

The comparison command produces both rankings, evidence and reasons, concise fallback examples, executed benchmark results, and a report that separates implemented, measured, and unverified behavior. Real generated prose is reviewed against the same evidence when the local model is available. Shorter output must preserve relevant conditions and actual ordering reasons.

Use TDD at the matching operation and comparison-command boundaries. Run focused checks, `go test ./...`, `gofmt`, and `git diff --check`. Apply the required independent code review before completion. Preserve unrelated work.

Stop when the approved prototype and review artifacts are reproducible and every required acceptance check has evidence. If live model verification remains unavailable, report the offline result as partial and leave that acceptance check open. Team adoption and app integration require separate approval. The user subsequently authorized a local commit of this branch; pushes, PR creation, merging and deployment remain separate actions.

## Agent-friendly repository architecture audit

The operator additionally requested a Project agent to investigate whether the repository's architecture can prevent mistakes agents repeatedly make. This is an investigation and plan for review, not approval to execute a repository-wide refactor. It extends the review material without replacing the matching prototype decisions above.

Use the project-local `poteto-mode`, `correct`, and `architect` skills. Read relevant past commits and workarounds to identify recurring code-quality, performance, and bug classes. Use `recall` for relevant prior conversations if they add evidence. Count a recurring class only after two independent incidents, and do not assume a historical fix was caused by an agent.

Answer technical open questions through small isolated prototypes and actual measurements. Compare alternatives against the same criteria, identify already-repaired issues, and prefer structural enforcement over repeated prose instructions. Return a high-level phased plan with evidence, ownership, verification, dependencies, and stopping conditions. Keep broader refactor proposals distinguishable from the matching work.

The [audit task](agent-friendly-audit-task.md) defines the worker's writable files and acceptance criteria. The parent reviews the findings and merges the evidence-backed plan here. The operator permits at most three active subagents, with no more than two Claude workers and one OpenAI worker. Workers do not delegate. Start with one GPT-6.1 worker at high reasoning and add bounded help only if the investigation needs it.

After the user approves the resulting plan, ask whether to execute the local stack or full autopilot playbook. Do not select or start that playbook before the answer. Both adapted playbooks keep work local and retain the separate authorization boundaries for Git publication, merge, and deployment.

The [completed audit](agent-friendly-audit.md) found three recurring classes in the reviewed history: model-generated facts exceeding authoritative input, unused alternative application paths, and startup accepting unresolved prerequisites. Six behavioral regressions failed before their repairs and passed after repair and at current HEAD. Those repairs remain present. Repair coauthors do not establish who introduced the defects, so the audit does not claim an agent-caused failure rate.

Two current findings guide the plan. Buyer discards criterion assessments and deciding ranking keys before constructing the writer packet. Useful SQL regressions call `Store.Search`, which has no tracked production caller, while the app uses `Candidates`. Preserve those tests' invariants through the production interface before deleting the unused method.

Prefer one retained record of fit, evidence and ordering reasons over public score records that callers independently sort and explain. An isolated ownership prototype demonstrated that copied projections preserve the authoritative record when a consumer changes its output. Caller-owned score records allowed the numerator to diverge from their contributions. This proves mutation isolation only; it does not establish full matching correctness or generated prose quality.

During the matching experiment, `internal/matching` owns assessments and preference accounting. The experiment owns final grouping, explicit sort and comparison reasons outside the preference scorer. Its final ordering function constructs one record for report and explanation projections. If the team adopts the design, migrate that ordering owner into the application once and remove superseded paths. Do not add another unused production interface for the experiment.

| Proposed phase | Scope and owner | Verification and stopping condition |
| --- | --- | --- |
| Define the retained record | Matching and buyer owners. Resolve criterion IDs, evidence references, source-backed priorities, contributions, branch IDs, eligibility inputs and deciding ordering reasons against the accepted policy. | Independent examples cover ties and reason sources. Stop when each field has an owner and the proposed path has one final comparator. |
| Build the matching comparison | Matching and experiment owners. Execute the standalone prototype described above. Keep application behavior intact. | Frozen assessments, the full acceptance cases and repeated baseline/proposed measurements on this machine. Stop at the reproducible comparison and review artifacts; leave live prose checks open if inference remains unavailable. |
| Align retrieval tests and remove the unused path | Persistence owner. This is separately approved broader cleanup and does not block the standalone comparison. Move meaningful currency, conjunction, exclusion, publication and archive proofs to `Candidates`, then remove unused `Search` helpers. | Run those proofs against a designated disposable test database and check affected agency/preview callers. Stop only after preserving the useful coverage and removing callers of the retired path. |
| Integrate an adopted policy | Buyer and writer owners, after a separate team adoption decision. Use the retained record for app ranking, cards, writer and fallback. Remove superseded ranking and experiment adapters in the same migration. | Verify gates, groups, score reconstruction, ties, consistent ranks, failures and actual generated prose. Public contract or frontend changes require their approved scope. |
| Correct stale guidance | Documentation owner. Update retired symbol/file references in `MATCHING_CONTRACT.md` alongside the relevant code decisions. | Check anchors against the tracked tree. Stop when current-state guidance describes real owners and callers, with old and proposed policies labeled. |

Preserve existing startup guards, typed clarification errors, batched retrieval and publication fingerprints. The evidence does not justify a repository-wide rewrite, new provider framework, caching layer or generic call-graph lint. The full audit contains the historical commit references, rejected alternatives and phase dependencies.

The [baseline checks](agent-friendly-baseline.md), [historical replay results](agent-friendly-audit/replay-results.json) and [ownership probe](agent-friendly-audit/shape-probe-results.json) are reviewable artifacts. The current deterministic buyer benchmark has a median of about 0.209 ms, approximately 650 KB and 572 allocations per turn on this Mac. It excludes model calls and SQL retrieval. This audit measured no proposed performance improvement. The later standalone comparison in the [results](property-matching-results.md) measures the proposed operation separately and records live writer failures. These are different workloads. Database behavior and live classifier accuracy remain unverified; the final explanation is backend-rendered. Application integration and publication have not started.

### Execution models and responsibilities

The user requires both OpenAI GPT-6.1-Sol and Anthropic Claude Opus 5.5 at high effort after playbook selection. Verify the actual model and effort when each worker starts; do not silently substitute another model. The execution used GPT-6.1-Sol at high reasoning and authenticated Claude Opus 5.5 at high effort. The [results](property-matching-results.md) record launch and verification evidence.

- The OpenAI worker implements the agreed fit operation, comparison command, retained ordering evidence and concise explanation path. It measures baseline and proposed computation against the same frozen inputs.
- A Claude worker independently specifies expected results from the brief, checks the implementation's gates, contributions, groups, ties and explanation fidelity, and challenges unsupported conclusions. Assign its fixtures and verification artifacts separately from implementation files.
- A second Claude worker may perform a bounded architecture and code review when useful. It checks ownership, affected callers and unnecessary complexity. It does not edit implementation files while another worker owns them.
- The parent owns task boundaries, integration, final verification and progress reports. Workers cannot delegate. The maximum remains three active subagents, two Claude and one OpenAI.

The user selected the adapted stack playbook. Execute the retained fit operation, final ordering and comparison, concise explanation path, then benchmarks and independent review as dependent verified units. The adapted full autopilot playbook would organize an approved queue of local units. Neither changes approved scope, model requirements or publication authorization.
