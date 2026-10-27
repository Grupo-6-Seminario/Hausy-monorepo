---
name: verify
description: "Prove a change in the running Hausy app: start the whole stack on disposable data, drive the real UI as a searcher or realtor, capture screenshots, and tear down what you started. Use after a change a user can see, or when asked to run, drive, or screenshot the app."
---

# Verify in the running app

You drive the stack the journeys use: Postgres, the seeded `hausy_e2e_test` database, the fake model, the Go API, and the production frontend build. It runs on ports 8710–8713 and leaves a dev server on 3000 or an API on 8080 alone. `frontend/playwright.journeys.config.ts` is the single source of truth for those servers. `scripts/stack.mjs` imports it.

## Prerequisites

- Docker running.
- Go.
- Google Chrome. The drives use `channel: 'chrome'`.
- `frontend/node_modules`, installed with `npm ci` in `frontend/`.
- Node 22.18 or later. It imports the TypeScript config and fixture directly.

## 1. Choose the features

Read [features/README.md](features/README.md) and open the feature files your change touches. Each file names what the user sees, the source behind it, and the route to drive. You're done with this step when you can name the screens and the text that will prove the change.

## 2. Launch

```sh
node .claude/skills/verify/scripts/stack.mjs up
node .claude/skills/verify/scripts/stack.mjs doctor
```

- `up` does the following:
  - Starts Postgres with `docker compose` if nothing listens on 5432.
  - Recreates `hausy_e2e_test` from `e2e/journeys/seed`. The seed refuses any database whose name doesn't end in `_test`.
  - Starts each server that isn't already answering.
  - Records what it started in `frontend/test-results/verify/stack.json`.
- The first `up` on a machine pulls the Postgres image and builds the frontend, which takes about 3 minutes. Later runs take 15 seconds to a minute. Server logs are in `frontend/test-results/verify/logs/`, and each `up` overwrites them.
- `doctor` must print `ok` for all four lines before you drive. Run it again after any drive fails. When a server is `DOWN` or `DEAD`, read its log, run `down`, then `up`.
- The fake model answers only the messages in `plans` in `e2e/journeys/fakemodel/main.go`. Any other searcher message fails with "no canned answer". To drive a new message, add its plan there, the same way the journeys do, and restart with `down` and `up`.
- The frontend is a build, so `up` serves the code as of its own start. After a frontend edit, run `down` and `up`.

## 3. Drive

`scripts/journey.mjs` is the representative drive and the template for new ones. It signs up a new searcher, declares a qualification, finds Soler 5818 showing "Cumplís lo que pide" in Palermo, and opens the three listings in Monserrat that refuse a garantía propietaria.

```sh
node .claude/skills/verify/scripts/journey.mjs
```

To drive your own route:
1. Copy `journey.mjs` to `frontend/test-results/verify/drive.mjs`. That directory is ignored, and `@playwright/test` resolves from `frontend/`.
2. Replace its steps with yours. Each `step` saves a screenshot even when it fails.
3. Run your copy.

Reuse `hydrated` and `search` from `frontend/journeys/fixture.ts`. The prompt can be visible before it works, and `hydrated` waits for that.

`search` takes the prompt's label. That label is `/¿Qué estás buscando\?/` before the first search and `/Sumá a tu búsqueda/` after it.

A route starts anonymously at `/` unless its feature file says to sign in. Full-page screenshots miss content inside the side panel's own scroll area. For the card or button that proves a step, use the template's `capture`: it scrolls the element into view and saves it on its own.

An agent with its own browser tool can open `http://127.0.0.1:8712` instead.

The seeded accounts and listings are in `frontend/journeys/fixture.ts`; the realtor is `inmobiliaria@journeys.test` with password `journeys-realtor`. Sign up searchers with a fresh `@journeys.test` email each run.

The drive is done when every step prints `ok` and you've opened the screenshots and seen the expected text in them. A step that prints `ok` is not enough evidence on its own.

## 4. Evidence

Each run writes `frontend/test-results/verify/<timestamp>/` with one numbered PNG per step. Report the directory, the screenshots that prove the change, and every limitation: a fake-model reply instead of a real one, the seeded fixture instead of the full catalog, Chrome at 1440×900.

## 5. Tear down

```sh
node .claude/skills/verify/scripts/stack.mjs down
```

`down` stops the servers `up` started, along with their child processes. It returns Docker to its previous state, using what `up` recorded:
- If the Postgres container already existed, `down` stops it.
- If `up` created the container, `down` removes it.
- If `up` also created the volume, `down` removes the volume too.

Reused servers keep running. Screenshots stay, and logs stay until the next `up`. The Postgres image stays cached. Run `down` after the last drive, including any reruns.

## Drift or a product bug?

- **The app works but a feature file or a step describes it wrongly:** that's drift. Fix the feature file or the drive.
- **`up` or `doctor` fails because the config, a port or a command changed:** that's harness drift. Fix `stack.mjs`, then run `up` and `doctor` again.
- **The app does something its spec, `CONTEXT.md` or a passing journey says it shouldn't:** that's a product bug. Report it with the screenshot. Leave the feature file describing the intended behavior.
