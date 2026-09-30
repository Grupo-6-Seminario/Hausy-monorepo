---
name: wizard
description: Set up a teammate's Hausy checkout, project skills, and local development environment when they request onboarding or invoke /wizard.
disable-model-invocation: true
---

# Teammate setup

Read `AGENTS.md`, `docs/SETUP.md`, and `docs/agents/skill-setup.md` from the repository root. Use those documents as the command and prerequisite reference.

1. Inspect the operating system, available tools, Git status, and existing configuration. Preserve local changes and secrets. Ask whether the teammate wants agent tooling only, local development, or Bedrock/AWS development when their request does not establish a scope. Finish when the scope and missing prerequisites are known.
2. Restore project skills with the documented setup command and run the instruction checker with `--installed`. Read the restored project-local skills for subsequent work. If Python is missing, use the documented uv fallback when uv is available; otherwise explain the required installation. Restart the agent session if its skill catalog needs to reload. Finish when the checker passes or report its exact blocker.
3. For local development, follow the setup guide for environment configuration, Postgres, committed seed data, backend, and frontend. Inspect existing files before copying templates. Confirm before installing dependencies, starting services, or loading data unless the teammate already authorized those actions. Require a configured model endpoint before claiming conversational search works. Finish when the requested components start and the documented checks pass, or list each blocked component.
4. For Bedrock/AWS development, follow `infra/AGENTS.md`. Obtain explicit authentication authorization, then verify account identity. Infrastructure changes, paid model calls, and deployment require their applicable authorization. A local setup request alone does not authorize them.
5. Report checks executed, results, remaining prerequisites, and how to start the requested components. Claim setup complete only for components actually verified. Keep credentials out of output. Leave Git commits, pushes, PRs, and external messages to separately authorized workflows.
