# Repo setup

Hand this file to your coding agent — "set up this repo following docs/SETUP.md" — or run it
yourself. Works the same on Windows, macOS and Linux: the repo contains **no symlinks**, so a
plain `git clone` is enough on every platform.

Setup is done when every check in [Verify](#verify) passes.

## 1. Prerequisites

| Tool | Why | Install (Windows, PowerShell) | Install (macOS) |
| --- | --- | --- | --- |
| Go 1.26.4+ | builds the module | `winget install GoLang.Go` | `brew install go` |
| uv | runs MCP servers and portable Python commands | `winget install astral-sh.uv` | `brew install uv` |
| Node 22.13+ | runs the optional skill installer | `winget install OpenJS.NodeJS.LTS` | `brew install node` |
| Python 3.11+ | runs project agent scripts | `uv python install 3.11` | `uv python install 3.11` |
| AWS CLI v2 | credentials for Bedrock and the aws-mcp server | `winget install Amazon.AWSCLI` | `brew install awscli` |
| Docker | runs the Postgres the listing store uses | `winget install Docker.DockerDesktop` | `brew install --cask docker` |
| PostgreSQL client 14+ | optional, for `psql` against that database | `winget install PostgreSQL.PostgreSQL.17` | `brew install postgresql@17` |
| Terraform 1.15+ | optional, only to change AWS resources ([section 7](#7-terraform)) | `winget install Hashicorp.Terraform` | `brew install hashicorp/tap/terraform` |

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
authentication. To use Claude on Bedrock instead, set `HAUSY_LLM=bedrock` and your own
`AWS_PROFILE` (section 4). To run only the reply writer on Bedrock and keep planning on the
local model, set `HAUSY_WRITER_LLM=bedrock` instead.

## 4. AWS credentials

The `aws-mcp` server and the Go program both use your ambient AWS credentials. Sign in with
your own IAM user, never the account root:

```bash
aws login --profile <your-profile>
AWS_PROFILE=<your-profile> aws sts get-caller-identity   # must print arn:aws:iam::<account>:user/<you>
```

If the browser is signed in to the console as root, `aws login` asks whether to switch the
profile to root: sign out and sign back in as your IAM user instead. A command run through
Claude Code's `!` prefix cannot answer that prompt.

Bedrock calls are recorded in CloudTrail under your IAM user name. To check that your
credentials can reach the model:

```bash
HAUSY_BEDROCK_LIVE=1 AWS_PROFILE=<your-profile> go test ./internal/bedrock -run Live
```

`get-caller-identity` must print the **project** account. Confirm the account id with a
teammate before creating anything — several people on this project have more than one AWS
account configured, and the MCP server uses whichever profile is active.

To pin a specific profile, set `AWS_PROFILE` in `.env`.

## 5. Agent tooling

Restore the pinned skills with `python3 scripts/setup-agent-skills.py`, then run `python3 scripts/check-agent-instructions.py --installed`. When `python3` is not on PATH, including Windows, use `uv run --python 3.11 --no-project scripts/setup-agent-skills.py` and the same prefix for the checker or verification script. See [skill setup](agents/skill-setup.md). This setup does not enable a paid review service or an issue-tracker workflow.

For Claude Code, approve project-scoped MCP servers only when you intend to use them. The local Matt Pocock review skill is separate from paid review services and marketplace plugins. Other agents use the root instructions and the same restored project-local baseline.

## 6. Postgres

```bash
docker compose up -d
```

That starts Postgres 14 on 5432 with database, user and password all `hausy`, matching the
`DATABASE_URI` default in `.env.example`. Override it by setting `DATABASE_URI` in `.env`.
No `createdb` is needed, and migrations apply themselves on the first `listings load` or
`go run ./cmd/hausy`, whichever runs first. `go run ./cmd/hausy` refuses to start while
Postgres is down: without it there is nothing to search.

Every worktree shares the one `hausy-db` container (the compose project is named `hausy`).
If `docker compose up -d` reports `The container name "/hausy-db" is already in use`, that
container predates the fixed name. Keep your accounts with a dump, then recreate it:

```bash
docker exec hausy-db pg_dump -U hausy -d hausy --clean --if-exists > hausy.sql
docker rm -f hausy-db && docker compose up -d --wait
docker exec -i hausy-db psql -U hausy -d hausy < hausy.sql
```

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

## 7. Terraform

AWS infrastructure is Terraform under `infra/` ([ADR 0002](./adr/0002-infrastructure-as-terraform-in-this-repo.md)).
You only need this section to change AWS resources. The provider pins no profile: Terraform uses
your `aws login` session through `AWS_PROFILE` (section 4).

```bash
aws login --profile <your-profile>
export AWS_PROFILE=<your-profile>
terraform -chdir=infra init
terraform -chdir=infra plan
```

There is no CI, so the plan is reviewed in the PR:

1. Change `infra/` on a feature branch and run `terraform -chdir=infra plan`.
2. Put the plan in the PR body inside a collapsible `<details>` section. Paste **only** the
   resource address lines (`# aws_... will be created`) and the `Plan:` summary. Plans print
   remote attributes, which can include agent prompts, environment variables and the account id.
3. After merge into `dev`, obtain explicit apply authorization for the reviewed plan before running `terraform -chdir=infra apply` with
   their own session. A plan from a feature branch is only a preview; never apply from one.

State lives in the S3 bucket `hausy-tfstate-csofja`, locked with a lock file next to the state
(`use_lockfile`), so two people cannot apply at once. The bucket was created once with the
commands below and is never managed by Terraform. They are recorded here for audit; do not run
them again.

```bash
BUCKET=hausy-tfstate-csofja
aws s3api create-bucket --bucket "$BUCKET" --region us-east-1
aws s3api put-bucket-versioning --bucket "$BUCKET" --versioning-configuration Status=Enabled
aws s3api put-public-access-block --bucket "$BUCKET" --public-access-block-configuration BlockPublicAcls=true,IgnorePublicAcls=true,BlockPublicPolicy=true,RestrictPublicBuckets=true
aws s3api get-bucket-encryption --bucket "$BUCKET"   # confirms default SSE-S3 (AES256)
```

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
| `terraform -chdir=infra plan` (section 7 only) | on an up-to-date `dev`: `No changes.` |
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
requires authentication. With `HAUSY_LLM=bedrock`, it sends them to `BEDROCK_MODEL_ID` on
Bedrock instead, signed with `AWS_PROFILE`; Jev still plans when `HAUSY_PLANNER=jev`.
Either Bedrock setting makes the API refuse to start until those credentials resolve: a
`startup_failed` log with `"stage":"aws_credentials"` means `aws login` again (section 4).

The API writes one JSON log line per event to stdout. Set `HAUSY_LOG_LEVEL` to `debug`,
`info` (default), `warn`, or `error`. Each request has an `X-Request-ID` response header
and a matching `request_id` in its `http_request` log; the frontend proxy passes that header
through. A search turn also logs `buyer_plan`, `buyer_candidates`, `buyer_eligibility`,
`buyer_qualitative` (when requested), `buyer_search`, `buyer_writer`, and `buyer_turn`
with stage durations and outcomes. Fallbacks have their own events. These application events
record counts and error classes while omitting messages, qualification values, tokens, provider
response bodies, and the database URI.

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
cookie. Accounts live in Postgres.
