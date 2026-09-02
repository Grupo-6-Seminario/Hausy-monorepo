import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { SearchExperience } from './components/search-experience';

afterEach(() => {
  Reflect.deleteProperty(document, 'modelContext');
});

describe('SearchExperience', () => {
  it('makes a natural-language property query the primary action', () => {
    render(<SearchExperience />);

    expect(
      screen.getByRole('textbox', { name: /describí cómo querés vivir/i }),
    ).toBeVisible();
    expect(screen.getByRole('button', { name: /buscar hogares/i })).toBeVisible();
  });

  it('keeps an empty query in place and explains what is missing', async () => {
    const user = userEvent.setup();
    render(<SearchExperience />);

    await user.click(screen.getByRole('button', { name: /buscar hogares/i }));

    expect(screen.getByRole('alert')).toHaveTextContent(
      'Contanos al menos una necesidad o preferencia.',
    );
    expect(screen.getByRole('textbox')).toHaveFocus();
  });

  it('turns a nuanced query into an explicit preference summary', async () => {
    const user = userEvent.setup();
    render(<SearchExperience />);
    const query =
      'Busco dos dormitorios en Palermo, hasta USD 1.000. Priorizo luz natural y poco ruido por encima del balcón.';

    await user.type(screen.getByRole('textbox'), query);
    await user.click(screen.getByRole('button', { name: /buscar hogares/i }));

    expect(await screen.findByText('Entendimos lo importante')).toBeVisible();
    expect(screen.getByText('Hasta USD 1.000')).toBeVisible();
    expect(screen.getByText('2 dormitorios')).toBeVisible();
    expect(screen.getByText('Luz natural primero')).toBeVisible();
    expect(screen.getByText('Poco ruido')).toBeVisible();
  });

  it('exposes the same search journey as a structured browser tool', async () => {
    const registerTool = vi.fn();
    Object.defineProperty(document, 'modelContext', {
      configurable: true,
      value: { registerTool },
    });
    render(<SearchExperience />);

    expect(registerTool).toHaveBeenCalledOnce();
    const tool = registerTool.mock.calls[0][0];
    expect(tool.name).toBe('search_properties');

    await tool.execute({ query: 'Dos dormitorios con luz natural' });

    expect(await screen.findByText('Entendimos lo importante')).toBeVisible();
  });
});
