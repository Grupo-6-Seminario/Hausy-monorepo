# Frontend instructions

Before UI or TS/TSX work, read [the shared workflow](../docs/agents/workflow.md). Apply `badesign-skill` to every UI task and `typescript-best-practices` to TS/TSX. Use the current interface, theme tokens, components, and approved brief as context. Broader redesigns need explicit approval; there is no separate DESIGN.md authority.

## Scope and behavior

Keep conversational search usable as the primary interaction unless an approved feature changes that behavior. Preserve existing navigation, contact and consent behavior, and evidence distinctions. Published facts and model interpretations must remain distinguishable. Generated photos or invented examples are not evidence of a real available property.

Keep hard search conditions in the backend contract; UI presentation does not silently remove them. Confirm public API changes before implementation. Preserve resilient contact navigation when contact-intent recording fails.

For visible changes, design actual loading, empty, error, success, disabled, hover, focus, and pressed states as applicable. Keep the existing shortlist usable during follow-up requests. Preserve keyboard operation, visible labels and focus, reduced motion, and fallback behavior when WebGPU or optional effects fail.

## Code and types

Follow local component and styling conventions. Reuse installed components and dependencies before adding anything. Use theme tokens for coherent changes; palette, typography, and motion choices are governed by BADESIGN, the existing interface, and the approved scope.

Use strict types, `unknown` at untrusted boundaries, validation before narrowing, and discriminated unions for distinct states. Add branded types only when they prevent a concrete mistake. Reuse existing validation patterns before considering a schema dependency. Use casts only where the runtime claim is justified; avoid casts that hide a type problem.

Keep continuous pointer or animation values outside React state where appropriate. Clean up listeners, observers, loops, and GPU resources. Keep useful short comments for function purpose or important decisions.

## Verify the result

Read [testing and review](../docs/agents/testing.md). Inspect the running interface and capture screenshots for visible changes. Check the affected journey at desktop and mobile sizes, plus the 320px minimum width from BADESIGN. Check keyboard interaction, focus, contrast, overflow, reduced motion, and affected loading/error states. Verify affected theme variants and GPU fallback when relevant.

Use package scripts as the command source. The existing browser suite covers 1440px desktop and 390px mobile; add a focused 320px inspection for changed layouts. Screenshots and traces belong in ignored test output. Report which states and sizes were inspected, and distinguish fixture-backed frontend proof from live backend/model proof.
