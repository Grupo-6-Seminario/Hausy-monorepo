# Implementation Plan: Natural-language property search

**Branch**: `Nick2611/frontend-query-ui-refactor` | **Date**: 2026-09-04 | **Spec**: [spec.md](./spec.md)

## Summary

Keep the existing Vinext-to-Go request boundary, replace the one-response result surface with a persistent multi-turn conversation and stable property shortlist, and apply the exact dark palette and typographic hierarchy from `frontend/DESIGN.md`. The progressively enhanced vgpu border remains independent from request handling and is the only chromatic artifact.

## Technical Context

**Language/Version**: TypeScript 5.9, React 19

**Primary Dependencies**: Vinext, Tailwind CSS 4, shadcn-owned components, vgpu

**Storage**: N/A for this slice

**Testing**: Vitest, Testing Library, jsdom

**Target Platform**: Modern browsers with graceful no-WebGPU fallback

**Project Type**: Web application inside an existing Go repository

**Performance Goals**: Keep the primary form interactive without waiting for WebGPU initialization; cap the analytic LED rendering at 1100 by 420 pixels, use 60 frames per second while interactive and 30 while idle

**Constraints**: First viewport exposes the query; local model credentials stay server-side; agent failures are recoverable; accessible reduced-motion fallback

**Scale/Scope**: One route and one primary search flow

## Constitution Check

- Red-green TDD at the public form seam: pass.
- Existing buyer-agent boundary is reused instead of duplicating interpretation in the UI: pass.
- LLM/A2A value is not claimed or simulated as validated: pass.
- Visual effects cannot block product behavior: pass.
- Follow-up loading preserves prior evidence and results instead of replacing them: pass.
- Color usage is limited to the exact `DESIGN.md` palette, with prism channels confined to the vgpu artifact: pass.

## Project Structure

```text
frontend/
├── app/
│   ├── components/
│   ├── globals.css
│   ├── layout.tsx
│   └── page.tsx
├── public/
└── package.json

specs/001-search-ui/
├── spec.md
├── plan.md
└── tasks.md
```

**Structure Decision**: Keep the prototype frontend isolated under `frontend/` so the existing Go module and future API boundaries remain explicit.
