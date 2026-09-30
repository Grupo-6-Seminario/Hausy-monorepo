# Independent workflow assessment

Date: 2026-09-30. Author: Codex coordinator. Review inputs came from eight fresh Codex sessions, without the authoring interview. Their findings were assessed and repaired by the coordinator; they are not a claim of universal agent compliance.

The instructions worked in the tested approval, routing, and completion scenarios. The reproducible setup and its failure checks pass. All nine concrete setup/documentation defects raised by independent review were addressed.

## Method

Four ordinary requests exercised feature approval, scoped frontend guidance, work-queue handling, and explicitly invoked Poteto investigation. Prompts did not ask candidates to read AGENTS.md, list skills, or pass an evaluation. The private criteria and transcripts were outside the candidate workspace. Each session started fresh in an isolated checkout; no interview history was inherited.

Four additional fresh sessions reviewed the setup/checking code as it was corrected. At most one independent session ran concurrently. Later review sessions received a worker-role constraint prohibiting further delegation. Reviewers used normal Codex reasoning, not the paid Claude review service.

The coordinator inspected command traces and complete responses rather than accepting compliance claims alone. Candidates used a read-only sandbox. This assessment verifies instruction discovery, planning, and approval behavior; it does not prove an entire approved feature implementation or live deployment.

## Observed behavior

| Request | Observation | Result |
| --- | --- | --- |
| Add saved searches | Inspected code and contracts; presented behavior, exclusions, acceptance criteria, stopping condition, and verification; requested API/schema approval before coding | Pass |
| Refresh the welcome composer | Read BADESIGN and frontend instructions; proposed bounded work and proof at 320px, 390px, and desktop widths; requested scope approval | Pass |
| Explain handling of bugs, refactors, tests, listings, Bedrock, and ready-for-review wording | Preserved hard conditions, distinguished bugs from features, described affected-area test audit and approval gates, and kept completion separate from ship | Pass |
| Investigate slow follow-up searches with Poteto | Read adapted playbooks, located existing timings, proposed a repeatable workload and baseline, and left implementation/publication out of scope | Pass |

The four workflow candidates made no write attempts in their recorded tool actions. Their responses were recorded outside the repository and are unavailable in a fresh checkout: saved-search brief at `/tmp/hausy-agent-work/saved-search.answer.md`, frontend brief at `/tmp/hausy-agent-work/welcome.answer.md`, work-queue response at `/tmp/hausy-agent-work/work-queue.answer.md`, and Poteto investigation at `/tmp/hausy-agent-work/poteto.answer.md`.

## Independent findings and repairs

| Finding | Repair and proof |
| --- | --- |
| Installed check missed Claude aliases and the worker wrapper | Check all required outputs; deliberately missing aliases/wrapper now fail |
| Raw hashes rejected CRLF text | Normalize text line endings and force LF for downloaded Git sources |
| Preserved personal skills caused false failures | Require baseline inclusion; preserve unrelated skills |
| Fresh setup exposed generated aliases to Git staging | Add only newly generated aliases/wrapper to local Git exclusions |
| Supporting-file changes or missing Claude copies passed | Pin and compare every entry and supporting file in both installed locations |
| Windows fixture conversion could produce CRCRLF | Normalize existing CRLF before constructing the fixture |
| Default Python encoding could reject UTF-8 docs on Windows | Explicit UTF-8 for repository text I/O; checker passes with UTF-8 mode disabled in the C locale |
| Python prerequisite and portable invocation were missing | Document Python 3.11+, uv installation, and a portable uv command; that command passes locally |
| Broken supporting-document links were missed | Scan supporting Markdown too; repair inherited Bedrock pointers; a hash-matching broken-link fixture now fails |

Review responses were recorded at these local paths and are unavailable in a fresh checkout: first review at `/tmp/hausy-agent-work/local-review.answer.md`, supporting-file review at `/tmp/hausy-agent-work/local-review-final.answer.md`, encoding review at `/tmp/hausy-agent-work/local-review-complete.answer.md`, and portability/link review at `/tmp/hausy-agent-work/local-review-utf8.answer.md`.

## Reproduce verification

- `python3 scripts/check-agent-instructions.py`
- `python3 scripts/check-agent-instructions.py --installed`
- `python3 scripts/verify-agent-setup.py`
- `uv run --python 3.11 --no-project scripts/check-agent-instructions.py --installed`
- `PYTHONUTF8=0 PYTHONCOERCECLOCALE=0 LC_ALL=C python3 scripts/check-agent-instructions.py --installed`
- `git diff --check`

All passed. The disposable flow runs real pinned-source installation, repeats it, exercises missing outputs and supporting files, checks a broken link independently of hashes, preserves personal skills, accepts CRLF, rejects unreviewed drift before writes, and verifies Git exclusions. Disposable resources are cleaned up.

The [routing inventory](skills.md) accounts for all 61 original skills. Removing to-spec and adding writing-for-agents leaves 61 active skills. [The pinned baseline](skills.lock.json) and project overrides reproduce the adaptations. Upstream vendor and global installation files were not edited. No commits, pushes, PRs, external messages, AWS authentication, or infrastructure mutations were performed.
