# Implementation Plan: Natural-language property search

**Branch**: `feat/ui-prototype` | **Date**: 2026-09-02 | **Spec**: [spec.md](./spec.md)

## Summary

Add a standalone Vinext/React frontend under `frontend/`. Make the natural-language query the first-viewport working surface, use deterministic demo interpretation, and isolate a progressively enhanced vgpu canvas behind a static CSS fallback.

## Technical Context

**Language/Version**: TypeScript 5.9, React 19

**Primary Dependencies**: Vinext, Tailwind CSS 4, shadcn-owned components, vgpu

**Storage**: N/A for this slice

**Testing**: Vitest, Testing Library, jsdom

**Target Platform**: Modern browsers with graceful no-WebGPU fallback

**Project Type**: Web application inside an existing Go repository

**Performance Goals**: Keep the primary form interactive without waiting for WebGPU initialization

**Constraints**: First viewport exposes the query; no backend or real-listing claims; accessible reduced-motion fallback

**Scale/Scope**: One route and one primary search flow

## Constitution Check

- Red-green TDD at the public form seam: pass.
- Deterministic software for explicit constraints: pass.
- LLM/A2A value is not claimed or simulated as validated: pass.
- Visual effects cannot block product behavior: pass.

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
