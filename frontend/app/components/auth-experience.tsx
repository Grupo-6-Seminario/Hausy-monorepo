'use client';

import { ArrowRight, LogOut } from 'lucide-react';
import { KeyboardEvent, useEffect, useRef, useState } from 'react';

import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import type { AuthRole, AuthUser } from '@/lib/auth';
import { cn } from '@/lib/utils';

import { usePointerGlow } from './pointer-glow';
import { ThemeToggle } from './theme-toggle';

type Mode = 'sign-in' | 'sign-up';

const modes: { value: Mode; label: string }[] = [
  { value: 'sign-in', label: 'Ingresar' },
  { value: 'sign-up', label: 'Crear cuenta' },
];

const roles: { value: AuthRole; title: string; description: string }[] = [
  {
    value: 'searcher',
    title: 'Busco dónde vivir',
    description: 'Tu agente recuerda tus preferencias entre búsquedas.',
  },
  {
    value: 'realtor',
    title: 'Soy agente inmobiliario',
    description: 'Gestioná las propiedades que publicás.',
  },
];

const roleTitle = (role: AuthRole) =>
  role === 'realtor' ? 'Agente inmobiliario' : 'Busco dónde vivir';

const roleWelcome = (role: AuthRole) =>
  role === 'realtor'
    ? 'Desde acá vas a poder gestionar tus propiedades y los requisitos de cada una.'
    : 'Tu agente va a recordar tus preferencias la próxima vez que busques.';

const roleDestination = (role: AuthRole) =>
  role === 'realtor'
    ? { href: '/inmobiliaria', label: 'Ir al catálogo' }
    : { href: '/', label: 'Ir a la búsqueda' };

export function AuthExperience() {
  const [mode, setMode] = useState<Mode>('sign-in');
  const [role, setRole] = useState<AuthRole>('searcher');
  const [name, setName] = useState('');
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [user, setUser] = useState<AuthUser | null>(null);
  const shellRef = useRef<HTMLElement>(null);

  usePointerGlow(shellRef);

  useEffect(() => {
    const controller = new AbortController();
    fetch('/api/auth/me', { signal: controller.signal })
      .then(async (response) => {
        if (!response.ok) return;
        const payload = (await response.json()) as { user: AuthUser };
        setUser(payload.user);
      })
      .catch(() => undefined);
    return () => controller.abort();
  }, []);

  function chooseMode(next: Mode) {
    setMode(next);
    setError('');
  }

  function handleTabKeys(event: KeyboardEvent<HTMLButtonElement>) {
    if (event.key !== 'ArrowLeft' && event.key !== 'ArrowRight') return;
    event.preventDefault();
    const next = mode === 'sign-in' ? 'sign-up' : 'sign-in';
    chooseMode(next);
    document.getElementById(`auth-tab-${next}`)?.focus();
  }

  async function handleSubmit(event: { preventDefault(): void }) {
    event.preventDefault();
    if (isSubmitting) return;
    setError('');
    setIsSubmitting(true);

    const body =
      mode === 'sign-up'
        ? { name, email, password, role }
        : { email, password };

    try {
      const response = await fetch(`/api/auth/${mode}`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(body),
      });
      const payload = (await response.json()) as {
        user?: AuthUser;
        error?: string;
      };
      if (!response.ok || !payload.user) {
        throw new Error(payload.error || 'No pudimos procesar tu cuenta.');
      }
      setUser(payload.user);
      setPassword('');
    } catch (cause) {
      setError(
        cause instanceof Error
          ? cause.message
          : 'No pudimos procesar tu cuenta.',
      );
    } finally {
      setIsSubmitting(false);
    }
  }

  async function signOut() {
    await fetch('/api/auth/sign-out', { method: 'POST' }).catch(
      () => undefined,
    );
    setUser(null);
    setMode('sign-in');
  }

  return (
    <main ref={shellRef} className="site-shell">
      <header className="site-header">
        {/* Full navigation on purpose: vinext only shims next/link inside Vite, not vitest. */}
        {/* oxlint-disable-next-line next/no-html-link-for-pages */}
        <a className="brand" href="/" aria-label="Hausy, inicio">
          <img src="/hausy_logo.png" alt="Hausy" width="40" height="40" />
          <span>Hausy</span>
        </a>
        <p className="prototype-note">Prototipo de acceso</p>
        <div className="header-actions">
          <ThemeToggle />
        </div>
      </header>

      <div className="auth-frame">
        <section className="auth-intro" aria-labelledby="auth-title">
          <p className="eyebrow">Cuenta Hausy</p>
          <h1 id="auth-title">Tu cuenta en Hausy.</h1>
          <p className="hero-subtitle">
            Ingresá o creá una cuenta. También podés buscar sin registrarte.
          </p>
          {/* oxlint-disable-next-line next/no-html-link-for-pages */}
          <a className="conversation-return" href="/">
            Continuar sin cuenta
          </a>
        </section>

        <section className="auth-panel" aria-label="Acceso a tu cuenta">
          {user ? (
            <div className="auth-session">
              <p className="eyebrow">Sesión iniciada</p>
              <h2>Hola, {user.name}.</h2>
              <p className="auth-role-chip">{roleTitle(user.role)}</p>
              <p className="auth-session-copy">{roleWelcome(user.role)}</p>
              <p className="auth-session-email">{user.email}</p>
              <div className="auth-session-actions">
                {/* oxlint-disable-next-line next/no-html-link-for-pages */}
                <a
                  className="auth-primary-link"
                  href={roleDestination(user.role).href}
                  data-glow
                >
                  {roleDestination(user.role).label}
                  <ArrowRight aria-hidden="true" />
                </a>
                <Button type="button" variant="outline" onClick={signOut}>
                  Cerrar sesión
                  <LogOut aria-hidden="true" />
                </Button>
              </div>
            </div>
          ) : (
            <>
              <div
                className="auth-tabs"
                role="tablist"
                aria-label="Elegí cómo continuar"
              >
                {modes.map((option) => (
                  <button
                    key={option.value}
                    id={`auth-tab-${option.value}`}
                    type="button"
                    role="tab"
                    aria-selected={mode === option.value}
                    aria-controls="auth-form"
                    tabIndex={mode === option.value ? 0 : -1}
                    onClick={() => chooseMode(option.value)}
                    onKeyDown={handleTabKeys}
                  >
                    {option.label}
                  </button>
                ))}
              </div>

              <form
                id="auth-form"
                className="auth-form"
                role="tabpanel"
                aria-labelledby={`auth-tab-${mode}`}
                onSubmit={handleSubmit}
                noValidate
              >
                <h2>{mode === 'sign-up' ? 'Creá tu cuenta' : 'Ingresá'}</h2>

                {mode === 'sign-up' ? (
                  <>
                    <fieldset className="auth-roles">
                      <legend>Tipo de cuenta</legend>
                      {roles.map((option) => (
                        <label
                          key={option.value}
                          data-glow
                          className={cn(
                            'auth-role',
                            role === option.value && 'auth-role-selected',
                          )}
                        >
                          <input
                            type="radio"
                            name="role"
                            value={option.value}
                            checked={role === option.value}
                            onChange={() => setRole(option.value)}
                          />
                          <span>{option.title}</span>
                          <small>{option.description}</small>
                        </label>
                      ))}
                    </fieldset>

                    <div className="auth-field">
                      <label htmlFor="auth-name">Nombre</label>
                      <Input
                        id="auth-name"
                        autoComplete="name"
                        value={name}
                        onChange={(event) => setName(event.target.value)}
                      />
                    </div>
                  </>
                ) : null}

                <div className="auth-field">
                  <label htmlFor="auth-email">Email</label>
                  <Input
                    id="auth-email"
                    type="email"
                    autoComplete="email"
                    inputMode="email"
                    value={email}
                    onChange={(event) => setEmail(event.target.value)}
                  />
                </div>

                <div className="auth-field">
                  <label htmlFor="auth-password">Contraseña</label>
                  <Input
                    id="auth-password"
                    type="password"
                    autoComplete={
                      mode === 'sign-up' ? 'new-password' : 'current-password'
                    }
                    aria-describedby={
                      mode === 'sign-up' ? 'auth-password-help' : undefined
                    }
                    value={password}
                    onChange={(event) => setPassword(event.target.value)}
                  />
                  {mode === 'sign-up' ? (
                    <p id="auth-password-help">Al menos 8 caracteres.</p>
                  ) : null}
                </div>

                {error ? (
                  <p className="auth-error" role="alert">
                    {error}
                  </p>
                ) : null}

                <Button
                  type="submit"
                  size="lg"
                  className="auth-submit"
                  data-glow
                  disabled={isSubmitting}
                >
                  {mode === 'sign-up' ? 'Crear cuenta' : 'Ingresar'}
                  <ArrowRight aria-hidden="true" />
                </Button>
              </form>
            </>
          )}
        </section>
      </div>
    </main>
  );
}
