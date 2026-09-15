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
| Docker | runs the Postgres the listing store uses | `winget install Docker.DockerDesktop` | `brew install --cask docker` |
| PostgreSQL client 14+ | optional, for `psql` against that database | `winget install PostgreSQL.PostgreSQL.17` | `brew install postgresql@17` |

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

For the local Hausy flow, set `LOCAL_LLM_TOKEN` when your OpenAI-compatible model requires
authentication. `AGENTCORE_HARNESS_ARN` is only needed for the separate AgentCore harness path;
it remains deliberately absent from the repo because it is account-specific.

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

## 6. Postgres

```bash
docker compose up -d
```

That starts Postgres 14 on 5432 with database, user and password all `hausy`, matching the
`DATABASE_URI` default in `.env.example`. Override it by setting `DATABASE_URI` in `.env`.
No `createdb` is needed, and migrations apply themselves on the first `listings load` or
`go run ./cmd/hausy`, whichever runs first.

To populate it with the committed seed data:

```bash
go run ./cmd/listings load
```

That is the whole thing — no scraping and no local model required. See
[DATA_MODEL.md](./DATA_MODEL.md) for the schema, and `AGENTS.md` for how the seed files are
produced when they need refreshing.

The **`postgres` MCP server** (a separate thing from the database itself) runs in
`--access-mode=restricted` (read-only) so a stray query cannot mutate anyone's database. If you
are doing schema work, change that flag to `--access-mode=unrestricted` in `.mcp.json` — as a
deliberate, reviewed edit rather than a reflex when a write fails. Note that `cmd/listings`
connects directly with `DATABASE_URI` and is unaffected by that flag.

The Postgres-backed tests (`internal/store/postgres`) truncate their target on every run.
To protect development data, they require designating a disposable database ending with
`_test` via `HAUSY_TEST_DATABASE_URI` (e.g. `postgresql://hausy:hausy@localhost:5432/hausy_test`).
Without it, those tests skip safely.

## Verify

Each command below should produce the stated result.

| Check | Expected |
| --- | --- |
| `go build ./...` | exits 0, no output |
| `go vet ./...` | exits 0, no output |
| `go test ./...` | passes (Postgres-backed tests skip safely when no disposable test database is designated) |
| `HAUSY_TEST_DATABASE_URI="postgresql://hausy:hausy@localhost:5432/hausy_test" go test ./internal/store/postgres` | passes against a disposable database; note that these tests truncate their target |
| `aws sts get-caller-identity` | prints the project account id |
| `claude mcp list` | `aws-mcp` and `postgres` both listed |
| `docker compose up -d` then `go run ./cmd/listings load` | loads the seed listings and prints the row count |
| `git status --short` | empty — a correct clone has nothing untracked |

That last row is the one that catches platform problems: if `git status` shows modifications
straight after cloning, stop and report it rather than committing them.

## Running it

Load the local configuration and start the Hausy API:

```bash
set -a
source .env
set +a
go run ./cmd/hausy
```

By default, the API listens at `http://127.0.0.1:8080` and sends messages to the
OpenAI-compatible model at `http://127.0.0.1:8000`. Set `LOCAL_LLM_TOKEN` when the local model
requires authentication.

In a second terminal, start the frontend:

```bash
cd frontend
npm install
npm run dev
```

The frontend's `/api/agent` route forwards prompt messages to the Hausy API and displays the
returned agent reply in a modal.

Accounts are optional: searching never requires one. `/ingresar` signs searchers and realtors in
through `/api/auth/*`, which keeps the backend's session token in an HttpOnly `hausy_session`
cookie. Without a reachable database the API keeps accounts in memory, so they disappear on
restart.
