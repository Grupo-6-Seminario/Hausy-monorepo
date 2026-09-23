import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { QualificationPanel } from './qualification-panel';

afterEach(() => vi.unstubAllGlobals());

function open() {
  return userEvent.click(screen.getByText(/¿Qué garantía tenés\?/));
}

describe('QualificationPanel', () => {
  it('turns the three answers into a qualification, without saving when signed out', async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValue(new Response('{}', { status: 401 }));
    vi.stubGlobal('fetch', fetchMock);
    const onChange = vi.fn();
    render(<QualificationPanel onChange={onChange} />);
    await open();

    await userEvent.click(screen.getByLabelText('Garantía propietaria'));
    await userEvent.selectOptions(
      screen.getByLabelText('Ingresos mensuales'),
      '2000000-3000000',
    );
    await userEvent.click(screen.getByLabelText('No'));
    await userEvent.click(
      screen.getByRole('button', { name: 'Usar estos datos' }),
    );

    expect(onChange).toHaveBeenLastCalledWith({
      guarantee: ['propietaria'],
      income_band: ['2000000-3000000'],
      caucion_quoted: ['no'],
    });
    expect(fetchMock).toHaveBeenCalledTimes(1); // the profile lookup only
  });

  it('prefills a signed-in profile and saves changes back to the account', async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(Response.json({ guarantee: ['caucion'] }))
      .mockResolvedValueOnce(new Response(null, { status: 204 }));
    vi.stubGlobal('fetch', fetchMock);
    const onChange = vi.fn();
    render(<QualificationPanel onChange={onChange} />);
    await open();

    await waitFor(() =>
      expect(screen.getByLabelText('Seguro de caución')).toBeChecked(),
    );
    await userEvent.click(screen.getByLabelText('Garantía propietaria'));
    await userEvent.click(
      screen.getByRole('button', { name: 'Usar estos datos' }),
    );

    expect(fetchMock).toHaveBeenLastCalledWith(
      '/api/me/qualification',
      expect.objectContaining({
        method: 'PUT',
        body: JSON.stringify({ guarantee: ['propietaria', 'caucion'] }),
      }),
    );
  });

  it('can be skipped', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue(new Response('{}', { status: 401 })),
    );
    const onChange = vi.fn();
    render(<QualificationPanel onChange={onChange} />);
    await open();
    await userEvent.click(
      screen.getByRole('button', { name: 'Buscar sin esto' }),
    );
    expect(onChange).not.toHaveBeenCalled();
    expect(screen.queryByLabelText('Ingresos mensuales')).not.toBeVisible();
  });
});
