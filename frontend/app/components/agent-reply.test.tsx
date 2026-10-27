import { render } from '@testing-library/react';
import { describe, expect, it } from 'vitest';

import { AgentReply } from './agent-reply';

describe('AgentReply', () => {
  it('renders bold and italic markers instead of printing the asterisks', () => {
    const { container } = render(
      <AgentReply
        reply={
          '- El #1 dice: *"APTO FINAER y GARANTÍA YA"*\n' +
          '**Tu garantía.** Precio 3 * 2 ambientes.'
        }
      />,
    );

    expect(container.querySelector('em')?.textContent).toBe(
      '"APTO FINAER y GARANTÍA YA"',
    );
    expect(container.querySelector('strong')?.textContent).toBe('Tu garantía.');
    expect(container.textContent).toBe(
      'El #1 dice: "APTO FINAER y GARANTÍA YA"Tu garantía. Precio 3 * 2 ambientes.',
    );
  });
});
