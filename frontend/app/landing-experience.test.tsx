import { render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { expect, it, vi } from 'vitest';
import { SearchExperience } from './components/search-experience';

it('lets visitors edit an example search before submitting it', async () => {
  const user = userEvent.setup();
  const fetchSpy = vi.spyOn(globalThis, 'fetch');
  render(<SearchExperience />);
  await user.click(
    screen.getByRole('button', { name: '2 amb en Palermo, con luz' }),
  );
  const composer = screen.getByRole('textbox', {
    name: '¿Qué estás buscando?',
  });
  expect(composer).toHaveValue(
    'Busco 2 ambientes en Palermo con mucha luz natural. Puedo estirar un poco el presupuesto si vale la pena.',
  );
  expect(composer).toHaveFocus();
  expect(fetchSpy).not.toHaveBeenCalled();
  fetchSpy.mockRestore();
});

it('brings the reader back to the prompt from the closing call to action', async () => {
  const user = userEvent.setup();
  // jsdom lays nothing out, so it has no scrolling to perform.
  const scrollIntoView = vi.fn();
  Element.prototype.scrollIntoView = scrollIntoView;
  render(<SearchExperience />);
  const composer = screen.getByRole('textbox', {
    name: '¿Qué estás buscando?',
  });
  const closing = screen.getByRole('region', {
    name: 'Empezá contando qué buscás. Tarda un minuto.',
  });
  await user.click(
    within(closing).getByRole('button', { name: 'Empezar búsqueda' }),
  );
  expect(composer).toHaveFocus();
  expect(scrollIntoView).toHaveBeenCalled();
});
