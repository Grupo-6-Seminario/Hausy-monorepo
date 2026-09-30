# Hausy agent instructions

Hausy helps people search CABA rentals using property evidence and declared rental qualifications.

## Working agreement

Read [the workflow](docs/agents/workflow.md) before starting a task. For a new feature, present the intended behavior, exclusions, acceptance criteria, stopping condition, affected boundaries, and verification plan. Wait for the user's approval before implementing, including when the request already appears complete. Restore established behavior autonomously for bugs; clarify ambiguous behavior.

Use `ponytail` for implementation and `unslop` for prose. Challenge decisions with evidence and keep explanations short. Read [skill routing](docs/agents/skills.md) to select required and conditional skills, not every installed skill.

Allow one concurrent subagent across the task. Subagents do not delegate. The parent owns integration and verification. Only an explicit user invocation of `ship` authorizes commits, pushes, and PR creation. Completion itself is not authorization. Preserve unrelated changes.

## Read by task

| Task | Instructions |
| --- | --- |
| Frontend, UI, TS/TSX | [Frontend](frontend/AGENTS.md) |
| Go, HTTP, providers, persistence | [Backend](docs/agents/backend.md) |
| Package design metrics, complexity, benchmarks | [Metrics](docs/metrics/README.md) (`cmd/metrics`) |
| Tests, verification, completion review | [Testing and review](docs/agents/testing.md) |
| Listings, search, eligibility, model output | [Data and search](docs/agents/data.md), [domain glossary](CONTEXT.md) |
| Terraform, AWS, authentication, deployment | [Infrastructure](infra/AGENTS.md) |
| Installation, missing skills, skill edits | [Skill setup](docs/agents/skill-setup.md) |
| Product scope, hypotheses, milestones | [Product context](docs/agents/product-context.md) |

Root rules govern shared behavior; scoped files add domain guidance. Project-local skill copies implement these rules. If a tool exposes an upstream or global copy instead, follow the project workflow and report the mismatch. Fresh setup is in [SETUP](docs/SETUP.md).
