import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { AuthExperience } from './components/auth-experience';

const marta = {
  id: '7',
  email: 'marta@inmobiliaria.com',
  name: 'Marta',
  role: 'realtor',
};

function json(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  });
}

// Routes each call by URL; the screen asks /api/auth/me once on load.
function stubBackend(handlers: Record<string, () => Response>) {
  const fetchMock = vi.fn((input: string) => {
    const handler = handlers[input];
    return Promise.resolve(
      handler ? handler() : json({ error: 'Tenés que iniciar sesión.' }, 401),
    );
  });
  vi.stubGlobal('fetch', fetchMock);
  return fetchMock;
}

function bodyOf(fetchMock: ReturnType<typeof stubBackend>, url: string) {
  const call = fetchMock.mock.calls.find(([input]) => input === url) as
    | unknown[]
    | undefined;
  const init = call?.[1] as RequestInit | undefined;
  return JSON.parse(init?.body as string);
}

afterEach(() => {
  vi.unstubAllGlobals();
});

describe('AuthExperience', () => {
  it('signs in with email and password and greets the user by role', async () => {
    const user = userEvent.setup();
    const fetchMock = stubBackend({
      '/api/auth/sign-in': () => json({ user: marta }),
    });
    render(<AuthExperience />);

    await user.type(screen.getByLabelText('Email'), marta.email);
    await user.type(screen.getByLabelText('Contraseña'), 'alquileres-caba');
    await user.click(screen.getByRole('button', { name: /^ingresar$/i }));

    expect(
      await screen.findByRole('heading', { name: 'Hola, Marta.' }),
    ).toBeVisible();
    expect(screen.getByText('Agente inmobiliario')).toBeVisible();
    expect(bodyOf(fetchMock, '/api/auth/sign-in')).toEqual({
      email: marta.email,
      password: 'alquileres-caba',
    });
  });

  it('creates a realtor account with the chosen role', async () => {
    const user = userEvent.setup();
    const fetchMock = stubBackend({
      '/api/auth/sign-up': () => json({ user: marta }, 201),
    });
    render(<AuthExperience />);

    await user.click(screen.getByRole('tab', { name: 'Crear cuenta' }));
    await user.click(
      screen.getByRole('radio', { name: /soy agente inmobiliario/i }),
    );
    await user.type(screen.getByLabelText('Nombre'), 'Marta');
    await user.type(screen.getByLabelText('Email'), marta.email);
    await user.type(screen.getByLabelText('Contraseña'), 'alquileres-caba');
    await user.click(screen.getByRole('button', { name: /crear cuenta/i }));

    expect(
      await screen.findByRole('heading', { name: 'Hola, Marta.' }),
    ).toBeVisible();
    expect(bodyOf(fetchMock, '/api/auth/sign-up')).toEqual({
      name: 'Marta',
      email: marta.email,
      password: 'alquileres-caba',
      role: 'realtor',
    });
  });

  it('defaults new accounts to searchers', async () => {
    const user = userEvent.setup();
    render(<AuthExperience />);

    await user.click(screen.getByRole('tab', { name: 'Crear cuenta' }));

    expect(
      screen.getByRole('radio', { name: /busco dónde vivir/i }),
    ).toBeChecked();
  });

  it('shows the backend error next to the form and keeps the email', async () => {
    const user = userEvent.setup();
    stubBackend({
      '/api/auth/sign-in': () =>
        json({ error: 'El email o la contraseña no son correctos.' }, 401),
    });
    render(<AuthExperience />);

    await user.type(screen.getByLabelText('Email'), marta.email);
    await user.type(screen.getByLabelText('Contraseña'), 'incorrecta');
    await user.click(screen.getByRole('button', { name: /^ingresar$/i }));

    expect(await screen.findByRole('alert')).toHaveTextContent(
      'El email o la contraseña no son correctos.',
    );
    expect(screen.getByLabelText('Email')).toHaveValue(marta.email);
  });

  it('recognizes an existing session and signs out', async () => {
    const user = userEvent.setup();
    const fetchMock = stubBackend({
      '/api/auth/me': () => json({ user: { ...marta, role: 'searcher' } }),
      '/api/auth/sign-out': () => new Response(null, { status: 204 }),
    });
    render(<AuthExperience />);

    expect(
      await screen.findByRole('heading', { name: 'Hola, Marta.' }),
    ).toBeVisible();
    expect(screen.getByText('Busco dónde vivir')).toBeVisible();

    await user.click(screen.getByRole('button', { name: /cerrar sesión/i }));

    expect(
      await screen.findByRole('button', { name: /^ingresar$/i }),
    ).toBeVisible();
    expect(fetchMock).toHaveBeenCalledWith(
      '/api/auth/sign-out',
      expect.objectContaining({ method: 'POST' }),
    );
  });
});
