# Hausy frontend design reference

> A personal property search in a cozy green room, with the responsiveness of a modern creative tool.

## Direction

Hausy is decision support for a high-consideration housing choice. It should feel warm, clear, patient, and grounded, while its motion and response quality make the agent feel present.

The search conversation is the primary surface. Property results support it with evidence, trade-offs, and links to the original publication.

Design calibration:

- `DESIGN_VARIANCE: 6/10`
- `MOTION_INTENSITY: 6/10`
- `VISUAL_DENSITY: 4/10`
- Mode: targeted evolution of the existing experience

## Reference synthesis

The interaction language is informed by the four exports in the [GSAP Refero style guide](https://styles.refero.design/style/00537a20-e99e-4ef2-b119-c6f532c44cc9):

1. `DESIGN.md`: humanist display type, confident scale, light controls, deliberate animation.
2. Tailwind v4 theme: named theme tokens mapped to runtime CSS variables.
3. CSS Variables: a single source for color, typography, spacing, radius, and motion values.
4. Design Tokens: portable DTCG-style values in `design-tokens.json`.

Hausy borrows the reference's typography, rhythm, pill controls, motion hierarchy, and internally lit depth. It does not inherit the reference's black canvas, cream surface, or multicolor taxonomy.

## Non-negotiable color rule

Use only the cozy green palette below. Do not introduce, replace, or reinterpret colors without explicit approval from the project owner.

All transparency, shadows, gradients, focus treatments, and shader colors must be derived from these values. Rainbow, RGB, prism, purple, blue, orange, pink, and unrelated neon effects are not part of Hausy.

### Light theme

| Role                | Value     | Token                  |
| ------------------- | --------- | ---------------------- |
| Page background     | `#f5f9e6` | `--background`         |
| Primary text        | `#193724` | `--foreground`         |
| Card surface        | `#f9fbea` | `--card`               |
| Primary action      | `#2a5a3a` | `--primary`            |
| Primary action text | `#f5f9e6` | `--primary-foreground` |
| Secondary surface   | `#d3e0b8` | `--secondary`          |
| Muted surface       | `#e7edcf` | `--muted`              |
| Muted text          | `#46634f` | `--muted-foreground`   |
| Soft accent         | `#6fbe91` | `--accent`             |
| Border              | `#b8c99c` | `--border`             |
| Input border        | `#9eb487` | `--input`              |
| Focus ring          | `#3a7e4f` | `--ring`               |
| Error               | `#a73b3b` | `--destructive`        |

### Dark theme

| Role                | Value     | Token                  |
| ------------------- | --------- | ---------------------- |
| Page background     | `#102218` | `--background`         |
| Primary text        | `#eef4df` | `--foreground`         |
| Card surface        | `#173322` | `--card`               |
| Primary action      | `#6fbe91` | `--primary`            |
| Primary action text | `#102218` | `--primary-foreground` |
| Secondary surface   | `#284a34` | `--secondary`          |
| Muted surface       | `#203d2b` | `--muted`              |
| Muted text          | `#b8c9ad` | `--muted-foreground`   |
| Border              | `#3d5d46` | `--border`             |
| Input border        | `#56745d` | `--input`              |
| Focus ring          | `#6fbe91` | `--ring`               |
| Error               | `#e27d72` | `--destructive`        |

The page follows the system theme. A rendered page stays entirely within that theme.

## Typography

- Use DM Sans for interface and display text. It is the humanist substitute selected from the Refero guide.
- Use Geist Mono only for compact numerical or technical metadata.
- Use weight 600 and tight tracking for display headings; weight 400-500 for body copy.
- Prefer sentence case.
- Body copy uses a line height between 1.45 and 1.6.
- Display type can be large on the welcome view but must contract inside the result workspace.

## Spacing and shapes

Spacing uses a 4 px base and the named runtime tokens:

- Element gap: 16 px.
- Card padding: 24 px.
- Section rhythm: 80 px where the viewport allows it.
- Maximum page width: 1360 px; welcome content is limited to 760 px.
- Welcome headings range from 32 px on small screens to 60 px on desktop.
- Workspace headings are 26–32 px. Regular conversation text stays at 16 px.

Hausy uses one soft shape system:

- Main panels and property cards: 16-22 px radius.
- Inputs: 14-20 px radius.
- Buttons, metadata chips, and counters: fully rounded.
- Shadows are subtle, diffuse, and tinted with approved greens.

Square property cards, sharp text fields, and mixed sharp or rounded controls are not allowed.

## Layout

- Desktop search workspace: approximately 54 percent conversation and 46 percent results.
- The conversation pane can remain sticky on wide screens.
- Below 1180 px, use a single column with the conversation before results.
- Do not trap the full page in nested scroll areas. A bounded conversation history may scroll when necessary.
- The conversation sits directly on the page instead of inside nested cards. A new turn scrolls only the history viewport to that turn's beginning.
- The history is keyboard-focusable; results include a direct link back to the composer.
- Use generous gaps between property cards and clear internal grouping.

## Motion and interaction

Every animation must communicate hierarchy, feedback, or state.

- The header, headline, supporting copy, composer, and examples enter in a short stagger on first load.
- The prompt stays still on hover and focus; its border and diffuse light provide feedback.
- The green luminary beneath the prompt follows the mouse locally and brightens on hover, focus, and active search.
- New conversation turns and property cards rise into place once.
- Buttons compress on press. Directional icons move only when their control is engaged.
- Loading uses a skeletal or linear shimmer that matches the final shape, not a generic spinner.
- Animate opacity and transform for DOM transitions. The luminary is the only continuous canvas animation.
- Honor `prefers-reduced-motion`; render a static green light when motion is reduced.

### vgpu prompt luminary

The vgpu effect is a supported Hausy signature, not a prohibited dependency.

- Render it beneath the prompt, never as a rainbow border.
- Shader colors are limited to `#3a7e4f`, `#6fbe91`, and `#d3e0b8` or their dark-theme equivalents.
- At rest the glow remains quiet. Hover and focus increase local energy near the pointer. Searching raises the overall intensity.
- Track the pointer only inside the prompt stage. Do not attach a permanent page-wide pointer listener.
- Clamp device pixel ratio and keep one GPU context, surface, effect, and frame loop per mounted prompt.
- Render at up to 2× device pixel ratio, but calculate halo distances in CSS pixels so its width stays consistent across screens.
- Use premultiplied alpha and a Gaussian falloff. Every canvas edge must reach zero opacity before clipping; the CSS fallback must not add a second outline.
- Stop the frame loop while the page is hidden. Reduced motion renders a still frame and redraws on resize.
- Dispose listeners, the frame loop, and the GPU context on unmount.
- If WebGPU or shader startup fails, hide the canvas and keep a stable CSS glow derived from `--accent`.

## Conversation and agent communication

The agent response is a decision brief, not a transcript dump and not a duplicate of the cards.

For searches with results, prefer this structure:

1. `Mi lectura`: one or two sentences stating the main decision criterion.
2. `Por qué las elegí`: no more than three properties, each with one fit and one concession.
3. `Qué falta confirmar`: only relevant unknown or inferred facts, with no more than two items.
4. One focused follow-up question when an answer would materially improve the ranking.

Render headings, lists, and emphasis semantically. Raw Markdown markers must never be visible.

## Property results

- Keep the shortlist visible while follow-up requests run.
- Mark a property as `Destacada por Hausy` only when the agent explicitly cites that card's rank.
- Show published facts separately from model inference.
- Do not repeat every card's price, address, and features in the agent response.
- Keep the original publication link visible and secondary to the search conversation.
- Hover movement can reinforce that a card is interactive, but it must not imply a better ranking.

## Brand asset

Use `/hausy_logo.png` as the provisional Hausy mark. Preserve its colors and aspect ratio. It may identify the product but must not be recolored, cropped into another symbol, or used as decoration.

## Accessibility and resilience

- Keep visible labels above form fields.
- Preserve Enter to submit and Shift + Enter for a new line.
- Maintain visible green focus states and WCAG AA contrast.
- Use semantic headings, lists, status messages, and disclosure controls.
- Motion is supplementary. The full search flow works without WebGPU and with reduced motion.
- Continuous visual values stay outside React state to avoid pointer-driven rerenders.

## Verification

- `npm test`: component and API contract tests.
- `npm run test:ui`: Chrome browser tests for welcome, results, long conversations, keyboard return, errors without WebGPU, reduced motion, responsive layouts, and account forms. Chrome must be installed. The runner starts a local server or reuses port 5173.
- `npm run test:gpu`: renders the actual WGSL through a native WebGPU adapter and checks premultiplied alpha, transparent edges, smoothness, visible interaction feedback, and consistent size at 1×/2× DPR.
- `npm run lint`, `npx tsc --noEmit --incremental false`, and `npm run build` complete the checks.

Browser tests use explicit listing and response fixtures. They verify the frontend, not live model responses or listing accuracy. Browser screenshots and traces are written to ignored `test-results/` output.
