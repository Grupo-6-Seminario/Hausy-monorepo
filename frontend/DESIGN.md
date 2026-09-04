# Hausy frontend design reference

> A calm property search conversation in a cozy green room.

## Direction

Hausy is a decision-support product for people making a high-consideration housing choice. The interface should feel warm, clear, patient, and grounded. It is not a dark-tech product and it should not resemble an analytics dashboard.

The search conversation is the primary surface. Property results support that conversation with evidence, trade-offs, and links to the original publication.

## Non-negotiable color rule

Use only the cozy green palette documented below. Do not introduce, replace, or reinterpret colors without explicit approval from the project owner.

Rainbow lighting, RGB effects, prism treatments, neon borders, and multicolor gradients are not part of the Hausy visual language. Focus states should use the existing green ring token and must remain stable when WebGPU is unavailable.

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

The page follows the system theme. A single rendered page must remain entirely within one theme.

## Typography

- Use Geist Sans for interface and display text.
- Use Geist Mono only for compact numerical or technical metadata.
- Prefer sentence case.
- Use weight and spacing for hierarchy. Avoid oversized display typography inside the result workspace.
- Body copy should have a line height between 1.45 and 1.6.

## Shapes and depth

Hausy uses one soft shape system:

- Main panels and property cards: 16-22px radius.
- Inputs and buttons: 12-16px radius.
- Metadata chips and counters: fully rounded.
- Shadows are subtle and tinted green. Never use pure black shadows.

Square property cards, sharp text fields, and mixed sharp/rounded controls are not allowed.

## Layout

- Maximum page width: 1560px.
- Desktop search workspace: two columns with the conversation as the larger column.
- Target desktop balance: approximately 54 percent conversation and 46 percent results.
- The conversation pane can remain sticky on wide screens.
- Below 1180px, use a single column with the conversation before results.
- Do not trap the complete page in nested scroll areas. A bounded conversation history may scroll when necessary.
- Use generous gaps between property cards and clear internal grouping.

## Conversation and agent communication

The agent response is a decision brief, not a transcript dump and not a duplicate of the cards.

For searches with results, prefer this structure:

1. `Mi lectura`: one or two sentences stating the main decision criterion.
2. `Por qué las elegí`: no more than three properties, each with one fit and one concession.
3. `Qué falta confirmar`: only relevant unknown or inferred facts, with no more than two items.
4. One focused follow-up question when an answer would materially improve the ranking.

The interface should render headings, lists, and emphasis semantically. Raw Markdown markers must never be visible.

## Property results

- Keep the shortlist visible while follow-up requests run.
- Mark a property as `Destacada por Hausy` only when the agent's latest decision brief explicitly cites that card's rank. Never infer a recommendation from its array position.
- Show published facts separately from model inference.
- Do not repeat every card's price, address, and features in the agent response.
- Keep the link to the original publication visible and clearly secondary to the search conversation.

## Motion

- Motion intensity is low.
- Use motion only for state feedback, such as a short loading progress sweep or button press.
- Animate transform and opacity only.
- Honor `prefers-reduced-motion`.
- Do not use persistent decorative animation, pointer-following light, WebGPU borders, rainbow lighting, or glow effects.

## Brand asset

Use `/hausy_logo.png` as the provisional Hausy mark. Preserve its colors and aspect ratio. It may appear in the header and other restrained brand-identification contexts, but should not be recolored, cropped into another symbol, or used as decoration.

## Accessibility

- Keep visible labels above form fields.
- Preserve keyboard submission with Enter and multiline entry with Shift + Enter.
- Maintain visible focus states with the green focus-ring token.
- Use semantic headings, lists, status messages, and disclosure controls.
- All interactive controls must meet WCAG AA contrast.
