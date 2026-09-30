# Skill routing

Read the selected project-local SKILL.md in full before applying it. Required skills apply at their triggers, not all on every task. Conditional principles guide real decisions without separate mandatory ceremonies. User-invoked skills require an explicit request naming that workflow.

The authoritative shared process is [workflow](workflow.md); task-specific verification is [testing](testing.md). Locate skills and restore missing ones through [skill setup](skill-setup.md). Tool adapters and optional playbooks preserve the same approval and delegation limits.

## Baseline inventory

The audit found 61 distinct original project skills. One is removed and one is added, leaving 61 active skills. Every original skill has a route or exclusion below.

| Skill | Activation | Flow |
| --- | --- | --- |
| `amazon-bedrock` | Conditional | Explicit Bedrock/AgentCore work only; authentication and mutations retain approval gates. |
| `architect` | Conditional | Substantial structural changes or unclear interface ownership. |
| `arena` | Conditional | Materially different designs warrant comparison; bounded sequential alternatives. |
| `automate-me` | User invoked | Capture preferences as a local mode; approval before edits, no publication. |
| `badesign-skill` | Required | Every UI task; approved brief and existing product context, live visual proof. |
| `blast-radius` | Conditional | Shared contracts, persistence, authentication, or search semantics; executable downstream proof. |
| `bro` | Conditional | User asks for a plain-language restatement. |
| `code-review` | Required | Substantial code changes automatically; also requested review. Parent plus one worker; normal local skill, not paid Claude service. |
| `codebase-design` | Required | Understand affected modules, ownership, and verification boundaries at task start. |
| `create-verification-skill` | User invoked | Create a reproducible live verification flow; execute representative journey. |
| `deslop` | Required | Final code-diff cleanup; preserve behavior and useful comments. |
| `diagnosing-bugs` | Required | Reported failure, regression, or slowness; executed reproduction before repair. |
| `domain-modeling` | Conditional | Resolve domain terminology or qualifying ADR decisions; CONTEXT.md remains a glossary. |
| `figure-it-out` | Conditional | Large multi-part work without a narrower suitable workflow. |
| `grill-with-docs` | Conditional | Unresolved feature/design decisions or explicit interview request; grilling plus domain-modeling. |
| `grilling` | Conditional | Decision-tree interview; complete numbered frontier, recommendations, conflict resolution, confirmation. |
| `how` | Conditional | Behavior, ownership, placement, or substantial-flow investigation. |
| `improve-codebase-architecture` | User invoked | Broader architecture investigation; user selects opportunity before implementation. |
| `interrogate` | User invoked | Adversarial scrutiny; preserve disagreements and avoid automatic changes. |
| `maintain-verification-skill` | User invoked | Repair verification docs/harness; report product regressions separately. |
| `no-comments` | User invoked | Focused comment review; retain useful function and key-decision comments. |
| `ponytail` | Required | Every implementation, including fixes and refactors; shortest correct approved solution. |
| `poteto-mode` | User invoked | Optional adapted playbooks for deeper investigation, performance, prototypes, pause/resume, or instruction evaluation. |
| `principle-attack-the-premise` | Conditional | Two failed fixes sharing one premise; gather actor/state evidence before another fix. |
| `principle-boundary-discipline` | Conditional | External input, validation, adapters; validate at boundary and trust domain values. |
| `principle-build-the-lever` | Conditional | Repeated manual work or proof creates a real reliability cost; reuse smallest rerunnable tool. |
| `principle-encode-lessons-in-structure` | Conditional | Recurring correction; prefer an appropriate structural enforcement. |
| `principle-exhaust-the-design-space` | Conditional | Novel interaction/architecture without precedent; compare materially distinct sketches. |
| `principle-experience-first` | Conditional | Product/UX tradeoff within approved scope; finish user outcomes before adding scope. |
| `principle-fix-root-causes` | Conditional | Debugging; trace reproduction to owner and inspect sibling paths. |
| `principle-foundational-thinking` | Conditional | Core data shapes, sequencing, or shared state; establish structure before downstream logic. |
| `principle-guard-the-context-window` | Conditional | Large outputs or context pressure; bound reads and delegate within budget. |
| `principle-laziness-protocol` | Conditional | Refactoring or abstraction pressure; deletion and smallest meaningful change. |
| `principle-make-operations-idempotent` | Conditional | Retry, crash, or lifecycle behavior; repeated execution converges. |
| `principle-migrate-callers-then-delete-legacy-apis` | Conditional | Internal API replacement; migrate callers and remove old path, preserve real external obligations. |
| `principle-minimize-reader-load` | Conditional | Hard-to-trace code; reduce unnecessary layers and hidden state. |
| `principle-model-the-domain` | Conditional | Stateful/branch-heavy logic; encode actual invariants in suitable types or structures. |
| `principle-never-block-on-the-human` | Conditional | Reversible work within approved scope; preserve feature and external-action checkpoints. |
| `principle-outcome-oriented-execution` | Conditional | Planned migration; scoped verified units converge on approved final behavior. |
| `principle-prove-it-works` | Conditional | Before completion; inspect actual artifact and evidence. |
| `principle-redesign-from-first-principles` | Conditional | New requirement changes design assumptions; consider coherent owner/caller changes. |
| `principle-separate-before-serializing-shared-state` | Conditional | Concurrent writers; isolate writable targets before synchronization. |
| `principle-sequence-verifiable-units` | Conditional | Multi-step work; each coherent unit ends in an actual check. |
| `principle-subtract-before-you-add` | Conditional | Addition/refactor; remove relevant dead paths before adding complexity. |
| `principle-test-behavior-not-implementation` | Conditional | Any test decision; real boundary and independent expected result. |
| `principle-type-system-discipline` | Conditional | Types and signatures; practical invariants and boundary parsing without speculative machinery. |
| `recall` | Conditional | Resume work; verify history against current repository state. |
| `reflect` | User invoked | Review session lessons; local recommendations and backlog; approval before skill edits. |
| `research` | Conditional | External facts or primary-source investigation; cited local report. |
| `ship` | User invoked | Only explicit ship invocation commits, pushes, or creates/updates ready PRs into dev. |
| `show-me` | Conditional | Helpful concise visualization; required PR diagram inside explicitly invoked ship. |
| `show-me-your-work` | Conditional | Long, unattended, or multi-phase work; local decision trail. |
| `swarm` | User invoked | Partitioned work under one concurrent worker, no nested delegation. |
| `tdd` | Required | Behavior changes; approved feature boundary or established bug boundary. |
| `teach` | Conditional | User asks to understand a concept or body of work. |
| `technical-writing` | Conditional | Docs, readmes, comments, commit or PR prose; concise operational English. |
| `test-audit` | Required | Touching tests; broader affected-area audit after long tasks. |
| `typescript-best-practices` | Required | Reading or editing TS/TSX; practical strict types and boundary parsing. |
| `unslop` | Required | All prose, including updates, docs, and final reports. |
| `why` | Conditional | Design rationale, history, or regression explanation with evidence. |
| `writing-for-agents` | Required | Writing agent instructions, skills, or linked workflow documents. |

## Excluded workflows

| Skill or service | Decision |
| --- | --- |
| `to-spec` | Removed. Its defined outcome publishes an issue. Feature briefs and local specs use the shared workflow instead. |
| Paid Claude review service | Excluded. Use the normal local `code-review` skill. |
| `setup-matt-pocock-skills` | Not installed. Issue-tracker and triage wizard does not match this workflow. |
| Automatic issue publishing, messaging, PR creation, merging | Removed from adapted skills. Only explicitly invoked `ship` creates PRs. |

The installer briefly added the whole upstream bundle because its parser did not accept the equals-form skill selector. Unrequested additions were removed. The corrected space-form selector installed only `writing-for-agents`; no extra publishing workflows remain in the baseline.

## Flow examples

- A saved-search feature requires an approved brief, then `ponytail`, appropriate TDD, affected UI/backend guidance, artifact proof, `deslop`, and normal review. It ends locally.
- An ambiguous “con amenities” search bug uses diagnosis and glossary/contract evidence. Preserve the hard condition; clarify semantics if the intended behavior is not settled.
- A listing parser correction fixes deterministic normalization. It does not authorize a re-scrape or model snapshot regeneration.
- A failed GPU effect uses diagnosis, BADESIGN, actual fallback inspection, and relevant GPU/browser checks.
- A provider swap preserves the application-owned contract; explicit Bedrock work activates `amazon-bedrock`, while paid runs and authentication retain their gates.
- A performance investigation pins a real workload and baseline. Optional Poteto playbooks add trace analysis and measured improvement without automatic publication.
- An instruction evaluation starts a fresh session with ordinary task prompts and assesses actual behavior. It does not tell the candidate to read the instructions or inherit the authoring interview.
