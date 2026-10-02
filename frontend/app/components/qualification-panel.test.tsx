import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { factsFixture } from '@/lib/eligibility-facts.fixture';

import {
  describeQualification,
  QualificationPanel,
} from './qualification-panel';

afterEach(() => vi.unstubAllGlobals());

// localStorage is not available in this test environment; stub it the way the
// theme tests do.
function installStorage(initial: Record<string, string> = {}) {
  const values = new Map(Object.entries(initial));
  vi.stubGlobal('localStorage', {
    getItem: (key: string) => values.get(key) ?? null,
    setItem: (key: string, value: string) => void values.set(key, value),
    removeItem: (key: string) => void values.delete(key),
  });
}

const facts = factsFixture;

// routes answers fetch by URL; anything unrouted fails the way a missing
// backend does.
function routes(answers: Record<string, () => Response>) {
  const fetchMock = vi.fn((url: string) => {
    const answer = answers[url];
    return answer
      ? Promise.resolve(answer())
      : Promise.reject(new Error('unreachable'));
  });
  vi.stubGlobal('fetch', fetchMock);
  return fetchMock;
}

const factsOK = () => Response.json({ facts });

async function open() {
  await userEvent.click(await screen.findByText(/Tu situación/));
}

describe('QualificationPanel', () => {
  it('asks the questions the backend serves and turns the answers into a qualification', async () => {
    const fetchMock = routes({ '/api/eligibility/facts': factsOK });
    const onChange = vi.fn();
    render(<QualificationPanel onChange={onChange} />);
    await open();

    await userEvent.click(screen.getByLabelText('Garantía propietaria'));
    await userEvent.click(
      within(
        screen.getByRole('group', { name: '¿Podés comprobar tus ingresos?' }),
      ).getByLabelText('Sí'),
    );
    await userEvent.click(screen.getByLabelText('Perro'));
    await userEvent.selectOptions(
      screen.getByLabelText('Ingresos mensuales'),
      '2000000-3000000',
    );
    await userEvent.click(
      within(
        screen.getByRole('group', {
          name: '¿Ya cotizaste un seguro de caución?',
        }),
      ).getByLabelText('No'),
    );
    await userEvent.click(
      screen.getByRole('button', { name: 'Usar estos datos' }),
    );

    expect(onChange).toHaveBeenLastCalledWith({
      guarantee: ['propietaria'],
      income_documented: ['yes'],
      pets: ['dog'],
      income_band: ['2000000-3000000'],
      caucion_quoted: ['no'],
    });
    // Anonymous: the questions are read, the profile is neither read nor saved.
    expect(fetchMock.mock.calls.map(([url]) => url)).toEqual([
      '/api/eligibility/facts',
    ]);
  });

  it('prefills a signed-in profile and saves changes back to the account', async () => {
    installStorage({ hausy_signed_in: '1' });
    const fetchMock = routes({
      '/api/eligibility/facts': factsOK,
      '/api/me/qualification': () =>
        fetchMock.mock.calls.length > 2
          ? new Response(null, { status: 204 })
          : Response.json({ guarantee: ['caucion'], pets: ['none'] }),
    });
    const onChange = vi.fn();
    render(<QualificationPanel onChange={onChange} />);
    await open();

    await waitFor(() =>
      expect(screen.getByLabelText('Seguro de caución')).toBeChecked(),
    );
    expect(screen.getByLabelText('Ninguna')).toBeChecked();
    await userEvent.click(screen.getByLabelText('Garantía propietaria'));
    await userEvent.click(
      screen.getByRole('button', { name: 'Usar estos datos' }),
    );

    expect(fetchMock).toHaveBeenLastCalledWith(
      '/api/me/qualification',
      expect.objectContaining({
        method: 'PUT',
        body: JSON.stringify({
          guarantee: ['propietaria', 'caucion'],
          pets: ['none'],
        }),
      }),
    );
  });

  it('keeps "Ninguna" apart from the pets it denies', async () => {
    routes({ '/api/eligibility/facts': factsOK });
    const onChange = vi.fn();
    render(<QualificationPanel onChange={onChange} />);
    await open();

    await userEvent.click(screen.getByLabelText('Perro'));
    await userEvent.click(screen.getByLabelText('Ninguna'));
    expect(screen.getByLabelText('Perro')).not.toBeChecked();
    await userEvent.click(screen.getByLabelText('Gato'));
    expect(screen.getByLabelText('Ninguna')).not.toBeChecked();
    await userEvent.click(
      screen.getByRole('button', { name: 'Usar estos datos' }),
    );

    expect(onChange).toHaveBeenLastCalledWith({ pets: ['cat'] });
  });

  it('stays out of the way when the questions cannot be loaded', async () => {
    const fetchMock = routes({});
    const onChange = vi.fn();
    render(<QualificationPanel onChange={onChange} />);

    await waitFor(() => expect(fetchMock).toHaveBeenCalled());
    await waitFor(() =>
      expect(screen.queryByText(/Tu situación/)).toBeNull(),
    );
    expect(onChange).not.toHaveBeenCalled();
  });

  it('can be skipped', async () => {
    routes({ '/api/eligibility/facts': factsOK });
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

describe('describeQualification', () => {
  it('names what a search uses with the catalog labels', () => {
    expect(
      describeQualification(
        {
          guarantee: ['caucion'],
          income_band: ['2000000-3000000'],
          caucion_quoted: ['yes'],
          income_documented: ['yes'],
          pets: ['dog', 'cat'],
        },
        facts,
      ),
    ).toBe(
      'seguro de caución · ingresos comprobables · perro, gato · $2.000.000 a $3.000.000 · caución cotizada',
    );
    expect(describeQualification({ pets: ['none'] }, facts)).toBe(
      'sin mascotas',
    );
    expect(describeQualification({ guarantee: ['caucion'] }, [])).toBe('');
  });
});
