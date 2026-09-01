# Repo setup

Hand this file to your coding agent — "set up this repo following docs/SETUP.md" — or run it
yourself. Works the same on Windows, macOS and Linux: the repo contains **no symlinks**, so a
plain `git clone` is enough on every platform.

Setup is done when every check in [Verify](#verify) passes.

## 1. Prerequisites

| Tool | Why | Install (Windows, PowerShell) | Install (macOS) |
| --- | --- | --- | --- |
| Go 1.26.4+ | builds the module | `winget install GoLang.Go` | `brew install go` |
| uv | runs both MCP servers via `uvx` | `winget install astral-sh.uv` | `brew install uv` |
| Node 18+ | runs `npx skills` for skill updates | `winget install OpenJS.NodeJS.LTS` | `brew install node` |
| AWS CLI v2 | credentials for Bedrock and the aws-mcp server | `winget install Amazon.AWSCLI` | `brew install awscli` |
| PostgreSQL 14+ | only if you are working on persistence | `winget install PostgreSQL.PostgreSQL.17` | `brew install postgresql@17` |

Close and reopen the terminal after installing, so the new tools land on `PATH`.

## 2. Clone and build

```bash
git clone https://github.com/Grupo-6-Seminario/proyecto-angus-back.git
cd proyecto-angus-back
go build ./...
```

`go build ./...` exits 0 with no output. That is the whole build.

## 3. Environment

Copy the template and fill it in:

```powershell
# Windows PowerShell
Copy-Item .env.example .env
```

```bash
# macOS / Linux
cp .env.example .env
```

`AGENTCORE_HARNESS_ARN` is the only value with no default. It is account-specific and is
deliberately absent from the repo — ask a teammate for it, or read it from the AgentCore
console. Everything else in `.env.example` has a working default.

## 4. AWS credentials

The `aws-mcp` server and the Go program both use your ambient AWS credentials.

```bash
aws configure sso        # or: aws configure
aws sts get-caller-identity
```

`get-caller-identity` must print the **project** account. Confirm the account id with a
teammate before creating anything — several people on this project have more than one AWS
account configured, and the MCP server uses whichever profile is active.

To pin a specific profile, set `AWS_PROFILE` in `.env`.

## 5. Agent tooling

Both MCP servers and the shared skill are committed, so there is nothing to install — but each
one needs a one-time local approval the first time you open the repo in Claude Code:

1. Run `claude` in the repo root.
2. Approve the `claude-plugins-official` marketplace when prompted. This enables the
   `mattpocock-skills` plugin declared in `.claude/settings.json`.
3. Approve the `aws-mcp` and `postgres` MCP servers when prompted. Project-scoped servers
   always require explicit local approval; that is the trust gate, not a bug.

Other agents need no setup: Codex, Cursor and Copilot read `AGENTS.md` (or the pointer file
placed for them) straight from the clone.

## 6. Postgres (only for persistence work)

```bash
createdb angus
```

The `postgres` MCP server defaults to `postgresql://localhost:5432/angus`. Override it by
setting `DATABASE_URI` in `.env`.

The server runs in `--access-mode=restricted` (read-only) so a stray query cannot mutate
anyone's database. If you are doing schema work, change that flag to `--access-mode=unrestricted`
in `.mcp.json` — as a deliberate, reviewed edit rather than a reflex when a write fails.

## Verify

Each command below should produce the stated result.

| Check | Expected |
| --- | --- |
| `go build ./...` | exits 0, no output |
| `go vet ./...` | exits 0, no output |
| `go test ./...` | passes (`no test files` is fine until the first test lands) |
| `aws sts get-caller-identity` | prints the project account id |
| `claude mcp list` | `aws-mcp` and `postgres` both listed |
| `git status --short` | empty — a correct clone has nothing untracked |

That last row is the one that catches platform problems: if `git status` shows modifications
straight after cloning, stop and report it rather than committing them.

## Running it

```bash
go run ./cmd/angus
```

Prints the harness reply. Without `AGENTCORE_HARNESS_ARN` set it exits with a message naming
the missing variable.
