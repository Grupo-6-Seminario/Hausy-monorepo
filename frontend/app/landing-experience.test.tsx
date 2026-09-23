import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { expect, it, vi } from 'vitest';
import { SearchExperience } from './components/search-experience';

it('lets visitors edit an illustrative featured search before submitting it', async () => {
  const user = userEvent.setup();
  const fetchSpy = vi.spyOn(globalThis, 'fetch');
  render(<SearchExperience />);
  expect(
    screen.getByRole('heading', { name: 'Avisos destacados' }),
  ).toBeVisible();
  expect(screen.getByText(/Imágenes ilustrativas/)).toBeVisible();
  await user.click(
    screen.getByRole('button', {
      name: 'Buscar algo así: Un balcón para bajar un cambio',
    }),
  );
  const composer = screen.getByRole('textbox', {
    name: 'Describí cómo querés vivir',
  });
  expect(composer).toHaveValue(
    'Busco un departamento en CABA con balcón y luz natural. Quiero comparar precios, expensas y requisitos de ingreso.',
  );
  expect(composer).toHaveFocus();
  expect(fetchSpy).not.toHaveBeenCalled();
  fetchSpy.mockRestore();
});
