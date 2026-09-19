'use client';

import {
  Building2,
  ExternalLink,
  Home,
  MousePointerClick,
  Pencil,
  Plus,
  Trash2,
} from 'lucide-react';
import {
  type SyntheticEvent,
  useEffect,
  useMemo,
  useRef,
  useState,
} from 'react';

import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Textarea } from '@/components/ui/textarea';
import type { AgencyProperty, AgencyPropertyInput } from '@/lib/agency';
import type { AuthUser } from '@/lib/auth';

import { usePointerGlow } from './pointer-glow';
import { ThemeToggle } from './theme-toggle';

type LoadState = 'loading' | 'ready' | 'error';

const emptyProperty: AgencyPropertyInput = {
  url: '',
  neighborhood: '',
  address: '',
  description: '',
  operation: 'alquiler',
  price: { amount: null, currency: 'ARS' },
  expenses: { amount: null, currency: 'ARS' },
  total_area_m2: null,
  rooms: null,
  bedrooms: null,
  bathrooms: null,
};

function formatMoney(amount?: number | null, currency?: string) {
  if (amount == null) return 'A consultar';
  const formatted = new Intl.NumberFormat('es-AR', {
    maximumFractionDigits: 0,
  }).format(amount);
  return currency === 'USD' ? `USD ${formatted}` : `$ ${formatted}`;
}

function propertyTitle(property: AgencyProperty) {
  return property.address || `Propiedad en ${property.neighborhood}`;
}

function editableProperty(property: AgencyProperty): AgencyPropertyInput {
  return {
    url: property.url,
    neighborhood: property.neighborhood,
    address: property.address,
    description: property.description,
    operation: property.operation,
    price: { ...property.price },
    expenses: { ...property.expenses },
    total_area_m2: property.total_area_m2,
    covered_area_m2: property.covered_area_m2,
    rooms: property.rooms,
    bedrooms: property.bedrooms,
    bathrooms: property.bathrooms,
    parking_spaces: property.parking_spaces,
    age_years: property.age_years,
    floor: property.floor,
  };
}

export function AgencyDashboard() {
  const [state, setState] = useState<LoadState>('loading');
  const [user, setUser] = useState<AuthUser | null>(null);
  const [properties, setProperties] = useState<AgencyProperty[]>([]);
  const [error, setError] = useState('');
  const [editorOpen, setEditorOpen] = useState(false);
  const [editingID, setEditingID] = useState<string | null>(null);
  const [draft, setDraft] = useState<AgencyPropertyInput>(emptyProperty);
  const [editorError, setEditorError] = useState('');
  const [isSaving, setIsSaving] = useState(false);
  const [removingProperty, setRemovingProperty] =
    useState<AgencyProperty | null>(null);
  const [removeError, setRemoveError] = useState('');
  const [isRemoving, setIsRemoving] = useState(false);
  const shellRef = useRef<HTMLElement>(null);
  const editorFirstFieldRef = useRef<HTMLInputElement>(null);
  const removeConfirmRef = useRef<HTMLButtonElement>(null);

  usePointerGlow(shellRef);

  useEffect(() => {
    if (!editorOpen) return;
    const frame = requestAnimationFrame(() =>
      editorFirstFieldRef.current?.focus(),
    );
    return () => cancelAnimationFrame(frame);
  }, [editorOpen]);

  useEffect(() => {
    if (!removingProperty) return;
    const frame = requestAnimationFrame(() =>
      removeConfirmRef.current?.focus(),
    );
    return () => cancelAnimationFrame(frame);
  }, [removingProperty]);

  useEffect(() => {
    const controller = new AbortController();

    Promise.all([
      fetch('/api/auth/me', { signal: controller.signal }),
      fetch('/api/agency/catalog', { signal: controller.signal }),
    ])
      .then(async ([userResponse, catalogResponse]) => {
        const userPayload = (await userResponse.json()) as {
          user?: AuthUser;
          error?: string;
        };
        if (!userResponse.ok || !userPayload.user) {
          throw new Error(userPayload.error || 'Tenés que iniciar sesión.');
        }
        if (userPayload.user.role !== 'realtor') {
          throw new Error(
            'Esta cuenta no administra un catálogo inmobiliario.',
          );
        }

        const catalogPayload = (await catalogResponse.json()) as {
          properties?: AgencyProperty[];
          error?: string;
        };
        if (!catalogResponse.ok || !catalogPayload.properties) {
          throw new Error(
            catalogPayload.error || 'No pudimos cargar tu catálogo.',
          );
        }

        setUser(userPayload.user);
        setProperties(catalogPayload.properties);
        setState('ready');
      })
      .catch((cause) => {
        if (controller.signal.aborted) return;
        setError(
          cause instanceof Error
            ? cause.message
            : 'No pudimos cargar tu catálogo.',
        );
        setState('error');
      });

    return () => controller.abort();
  }, []);

  const contactTotal = useMemo(
    () =>
      properties.reduce((total, property) => total + property.contact_count, 0),
    [properties],
  );

  function openAddEditor() {
    setEditingID(null);
    setDraft(emptyProperty);
    setEditorError('');
    setEditorOpen(true);
  }

  function openEditEditor(property: AgencyProperty) {
    setEditingID(property.id);
    setDraft(editableProperty(property));
    setEditorError('');
    setEditorOpen(true);
  }

  function setNumber(
    field:
      | 'total_area_m2'
      | 'covered_area_m2'
      | 'rooms'
      | 'bedrooms'
      | 'bathrooms'
      | 'parking_spaces'
      | 'age_years',
    value: string,
  ) {
    setDraft((current) => ({
      ...current,
      [field]: value === '' ? null : Number(value),
    }));
  }

  async function saveProperty(event: SyntheticEvent<HTMLFormElement>) {
    event.preventDefault();
    if (isSaving) return;
    setIsSaving(true);
    setEditorError('');

    try {
      const response = await fetch(
        editingID
          ? `/api/agency/catalog/${encodeURIComponent(editingID)}`
          : '/api/agency/catalog',
        {
          method: editingID ? 'PATCH' : 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify(draft),
        },
      );
      const payload = (await response.json()) as AgencyProperty & {
        error?: string;
      };
      if (!response.ok || !payload.id) {
        throw new Error(payload.error || 'No pudimos guardar la propiedad.');
      }
      setProperties((current) =>
        editingID
          ? current.map((property) =>
              property.id === editingID ? payload : property,
            )
          : [payload, ...current],
      );
      setEditorOpen(false);
    } catch (cause) {
      setEditorError(
        cause instanceof Error
          ? cause.message
          : 'No pudimos guardar la propiedad.',
      );
    } finally {
      setIsSaving(false);
    }
  }

  async function removeProperty() {
    if (!removingProperty || isRemoving) return;
    setIsRemoving(true);
    setRemoveError('');

    try {
      const response = await fetch(
        `/api/agency/catalog/${encodeURIComponent(removingProperty.id)}`,
        { method: 'DELETE' },
      );
      if (!response.ok) {
        const payload = (await response.json()) as { error?: string };
        throw new Error(payload.error || 'No pudimos quitar la propiedad.');
      }
      setProperties((current) =>
        current.filter((property) => property.id !== removingProperty.id),
      );
      setRemovingProperty(null);
    } catch (cause) {
      setRemoveError(
        cause instanceof Error
          ? cause.message
          : 'No pudimos quitar la propiedad.',
      );
    } finally {
      setIsRemoving(false);
    }
  }

  return (
    <main ref={shellRef} className="site-shell agency-shell">
      <header className="site-header">
        {/* oxlint-disable-next-line next/no-html-link-for-pages */}
        <a className="brand" href="/" aria-label="Hausy, inicio">
          <img src="/hausy_logo.png" alt="Hausy" width="40" height="40" />
          <span>Hausy</span>
        </a>
        <div className="header-actions">
          {user ? (
            <span className="agency-account" data-glow>
              <Building2 aria-hidden="true" />
              {user.name}
            </span>
          ) : null}
          <ThemeToggle />
        </div>
      </header>

      {state === 'loading' ? (
        <section className="agency-loading" aria-live="polite" aria-busy="true">
          <p>Cargando tu catálogo</p>
          <div aria-hidden="true" />
          <div aria-hidden="true" />
        </section>
      ) : null}

      {state === 'error' ? (
        <section className="agency-error" role="alert">
          <Building2 aria-hidden="true" />
          <h1>No pudimos abrir el catálogo</h1>
          <p>{error}</p>
          {/* oxlint-disable-next-line next/no-html-link-for-pages */}
          <a href="/ingresar">Ir a ingreso</a>
        </section>
      ) : null}

      {state === 'ready' ? (
        <>
          <section className="agency-overview" aria-labelledby="agency-title">
            <div>
              <p className="eyebrow">Panel inmobiliario</p>
              <h1 id="agency-title">Tu catálogo</h1>
              <p>
                Mantené cada publicación actualizada y revisá cuántas personas
                iniciaron un contacto desde Hausy.
              </p>
              <Button type="button" size="lg" data-glow onClick={openAddEditor}>
                <Plus aria-hidden="true" />
                Agregar propiedad
              </Button>
            </div>
            <dl className="agency-metrics" aria-label="Resumen del catálogo">
              <div>
                <dt>Catálogo activo</dt>
                <dd>
                  <Home aria-hidden="true" />
                  {properties.length}{' '}
                  {properties.length === 1
                    ? 'propiedad activa'
                    : 'propiedades activas'}
                </dd>
              </div>
              <div>
                <dt>Interés observado</dt>
                <dd>
                  <MousePointerClick aria-hidden="true" />
                  {contactTotal}{' '}
                  {contactTotal === 1
                    ? 'contacto iniciado'
                    : 'contactos iniciados'}
                </dd>
              </div>
            </dl>
          </section>

          <section className="agency-catalog" aria-labelledby="catalog-title">
            <div className="agency-section-heading">
              <div>
                <h2 id="catalog-title">Propiedades publicadas</h2>
                <p>
                  Un contacto iniciado cuenta la activación del botón Contactar,
                  no una consulta enviada.
                </p>
              </div>
            </div>

            {properties.length === 0 ? (
              <div className="agency-empty">
                <Home aria-hidden="true" />
                <h3>Tu catálogo está vacío</h3>
                <p>Agregá tu primera propiedad para verla acá.</p>
              </div>
            ) : (
              <ul className="agency-property-list">
                {properties.map((property) => (
                  <li key={property.id} className="agency-property" data-glow>
                    <div className="agency-property-main">
                      <div>
                        <span className="agency-property-operation">
                          {property.operation.replace('_', ' ')}
                        </span>
                        <span>{property.neighborhood}</span>
                      </div>
                      <h3>{propertyTitle(property)}</h3>
                      <p className="agency-property-price">
                        {formatMoney(
                          property.price.amount,
                          property.price.currency,
                        )}
                      </p>
                      <p>
                        {[
                          property.rooms && `${property.rooms} amb`,
                          property.bedrooms && `${property.bedrooms} dorm`,
                          property.total_area_m2 &&
                            `${property.total_area_m2} m²`,
                        ]
                          .filter(Boolean)
                          .join(' / ')}
                      </p>
                    </div>
                    <div className="agency-property-interest">
                      <span>Contactos iniciados</span>
                      <strong>{property.contact_count}</strong>
                    </div>
                    <div className="agency-property-actions">
                      <a
                        href={property.url}
                        target="_blank"
                        rel="noopener noreferrer"
                        aria-label={`Abrir publicación de ${propertyTitle(property)}`}
                      >
                        <ExternalLink aria-hidden="true" />
                        Ver publicación
                      </a>
                      <Button
                        type="button"
                        variant="outline"
                        size="sm"
                        onClick={() => openEditEditor(property)}
                      >
                        <Pencil aria-hidden="true" />
                        Editar
                      </Button>
                      <Button
                        type="button"
                        variant="ghost"
                        size="sm"
                        onClick={() => {
                          setRemoveError('');
                          setRemovingProperty(property);
                        }}
                      >
                        <Trash2 aria-hidden="true" />
                        Quitar
                      </Button>
                    </div>
                  </li>
                ))}
              </ul>
            )}
          </section>

          {editorOpen ? (
            <div className="agency-editor-backdrop">
              <dialog
                open
                className="agency-editor"
                aria-modal="true"
                aria-labelledby="property-editor-title"
                onKeyDown={(event) => {
                  if (event.key === 'Escape') setEditorOpen(false);
                }}
              >
                <div className="agency-editor-heading">
                  <div>
                    <p className="eyebrow">Catálogo</p>
                    <h2 id="property-editor-title">
                      {editingID ? 'Editar propiedad' : 'Agregar propiedad'}
                    </h2>
                  </div>
                  <Button
                    type="button"
                    variant="ghost"
                    onClick={() => setEditorOpen(false)}
                  >
                    Cancelar
                  </Button>
                </div>

                <form className="agency-property-form" onSubmit={saveProperty}>
                  <div className="agency-field agency-field-wide">
                    <label htmlFor="property-url">URL de la publicación</label>
                    <Input
                      ref={editorFirstFieldRef}
                      id="property-url"
                      type="url"
                      required
                      value={draft.url}
                      onChange={(event) =>
                        setDraft((current) => ({
                          ...current,
                          url: event.target.value,
                        }))
                      }
                    />
                  </div>
                  <div className="agency-field">
                    <label htmlFor="property-address">Dirección</label>
                    <Input
                      id="property-address"
                      value={draft.address}
                      onChange={(event) =>
                        setDraft((current) => ({
                          ...current,
                          address: event.target.value,
                        }))
                      }
                    />
                  </div>
                  <div className="agency-field">
                    <label htmlFor="property-neighborhood">Barrio</label>
                    <Input
                      id="property-neighborhood"
                      required
                      value={draft.neighborhood}
                      onChange={(event) =>
                        setDraft((current) => ({
                          ...current,
                          neighborhood: event.target.value,
                        }))
                      }
                    />
                  </div>
                  <div className="agency-field">
                    <label htmlFor="property-operation">Operación</label>
                    <select
                      id="property-operation"
                      value={draft.operation}
                      onChange={(event) =>
                        setDraft((current) => ({
                          ...current,
                          operation: event.target
                            .value as AgencyPropertyInput['operation'],
                        }))
                      }
                    >
                      <option value="alquiler">Alquiler</option>
                      <option value="alquiler_temporal">
                        Alquiler temporal
                      </option>
                      <option value="venta">Venta</option>
                    </select>
                  </div>
                  <div className="agency-field">
                    <label htmlFor="property-price">Precio</label>
                    <Input
                      id="property-price"
                      type="number"
                      min="0"
                      inputMode="decimal"
                      value={draft.price.amount ?? ''}
                      onChange={(event) =>
                        setDraft((current) => ({
                          ...current,
                          price: {
                            ...current.price,
                            amount:
                              event.target.value === ''
                                ? null
                                : Number(event.target.value),
                          },
                        }))
                      }
                    />
                  </div>
                  <div className="agency-field">
                    <label htmlFor="property-currency">Moneda</label>
                    <select
                      id="property-currency"
                      value={draft.price.currency}
                      onChange={(event) =>
                        setDraft((current) => ({
                          ...current,
                          price: {
                            ...current.price,
                            currency: event.target.value,
                          },
                        }))
                      }
                    >
                      <option value="ARS">ARS</option>
                      <option value="USD">USD</option>
                    </select>
                  </div>
                  <div className="agency-field">
                    <label htmlFor="property-expenses">Expensas</label>
                    <Input
                      id="property-expenses"
                      type="number"
                      min="0"
                      inputMode="decimal"
                      value={draft.expenses.amount ?? ''}
                      onChange={(event) =>
                        setDraft((current) => ({
                          ...current,
                          expenses: {
                            ...current.expenses,
                            amount:
                              event.target.value === ''
                                ? null
                                : Number(event.target.value),
                          },
                        }))
                      }
                    />
                  </div>
                  <div className="agency-field">
                    <label htmlFor="property-expenses-currency">
                      Moneda de expensas
                    </label>
                    <select
                      id="property-expenses-currency"
                      value={draft.expenses.currency}
                      onChange={(event) =>
                        setDraft((current) => ({
                          ...current,
                          expenses: {
                            ...current.expenses,
                            currency: event.target.value,
                          },
                        }))
                      }
                    >
                      <option value="ARS">ARS</option>
                      <option value="USD">USD</option>
                    </select>
                  </div>
                  <div className="agency-field">
                    <label htmlFor="property-rooms">Ambientes</label>
                    <Input
                      id="property-rooms"
                      type="number"
                      min="0"
                      value={draft.rooms ?? ''}
                      onChange={(event) =>
                        setNumber('rooms', event.target.value)
                      }
                    />
                  </div>
                  <div className="agency-field">
                    <label htmlFor="property-bedrooms">Dormitorios</label>
                    <Input
                      id="property-bedrooms"
                      type="number"
                      min="0"
                      value={draft.bedrooms ?? ''}
                      onChange={(event) =>
                        setNumber('bedrooms', event.target.value)
                      }
                    />
                  </div>
                  <div className="agency-field">
                    <label htmlFor="property-bathrooms">Baños</label>
                    <Input
                      id="property-bathrooms"
                      type="number"
                      min="0"
                      value={draft.bathrooms ?? ''}
                      onChange={(event) =>
                        setNumber('bathrooms', event.target.value)
                      }
                    />
                  </div>
                  <div className="agency-field">
                    <label htmlFor="property-area">Superficie total</label>
                    <Input
                      id="property-area"
                      type="number"
                      min="0"
                      inputMode="decimal"
                      value={draft.total_area_m2 ?? ''}
                      onChange={(event) =>
                        setNumber('total_area_m2', event.target.value)
                      }
                    />
                  </div>
                  <div className="agency-field">
                    <label htmlFor="property-covered-area">
                      Superficie cubierta
                    </label>
                    <Input
                      id="property-covered-area"
                      type="number"
                      min="0"
                      inputMode="decimal"
                      value={draft.covered_area_m2 ?? ''}
                      onChange={(event) =>
                        setNumber('covered_area_m2', event.target.value)
                      }
                    />
                  </div>
                  <div className="agency-field">
                    <label htmlFor="property-parking">Cocheras</label>
                    <Input
                      id="property-parking"
                      type="number"
                      min="0"
                      value={draft.parking_spaces ?? ''}
                      onChange={(event) =>
                        setNumber('parking_spaces', event.target.value)
                      }
                    />
                  </div>
                  <div className="agency-field">
                    <label htmlFor="property-age">Antigüedad en años</label>
                    <Input
                      id="property-age"
                      type="number"
                      min="0"
                      value={draft.age_years ?? ''}
                      onChange={(event) =>
                        setNumber('age_years', event.target.value)
                      }
                    />
                  </div>
                  <div className="agency-field">
                    <label htmlFor="property-floor">Piso</label>
                    <Input
                      id="property-floor"
                      value={draft.floor ?? ''}
                      onChange={(event) =>
                        setDraft((current) => ({
                          ...current,
                          floor: event.target.value,
                        }))
                      }
                    />
                  </div>
                  <div className="agency-field agency-field-wide">
                    <label htmlFor="property-description">Descripción</label>
                    <Textarea
                      id="property-description"
                      required
                      value={draft.description}
                      onChange={(event) =>
                        setDraft((current) => ({
                          ...current,
                          description: event.target.value,
                        }))
                      }
                    />
                  </div>

                  {editorError ? (
                    <p className="agency-form-error" role="alert">
                      {editorError}
                    </p>
                  ) : null}

                  <div className="agency-form-actions">
                    <Button
                      type="button"
                      variant="outline"
                      onClick={() => setEditorOpen(false)}
                    >
                      Cancelar
                    </Button>
                    <Button type="submit" disabled={isSaving}>
                      {isSaving
                        ? 'Guardando'
                        : editingID
                          ? 'Guardar cambios'
                          : 'Guardar propiedad'}
                    </Button>
                  </div>
                </form>
              </dialog>
            </div>
          ) : null}

          {removingProperty ? (
            <div className="agency-editor-backdrop">
              <dialog
                open
                className="agency-confirmation"
                role="alertdialog"
                aria-modal="true"
                aria-labelledby="remove-property-title"
                aria-describedby="remove-property-description"
                onKeyDown={(event) => {
                  if (event.key === 'Escape') setRemovingProperty(null);
                }}
              >
                <Trash2 aria-hidden="true" />
                <h2 id="remove-property-title">
                  Quitar {propertyTitle(removingProperty)}
                </h2>
                <p id="remove-property-description">
                  La propiedad dejará de aparecer en tu catálogo y en las
                  búsquedas. El historial de contactos se conserva.
                </p>
                {removeError ? (
                  <p className="agency-form-error" role="alert">
                    {removeError}
                  </p>
                ) : null}
                <div className="agency-form-actions">
                  <Button
                    ref={removeConfirmRef}
                    type="button"
                    variant="outline"
                    onClick={() => setRemovingProperty(null)}
                  >
                    Cancelar
                  </Button>
                  <Button
                    type="button"
                    disabled={isRemoving}
                    onClick={removeProperty}
                  >
                    {isRemoving ? 'Quitando' : 'Confirmar quitar'}
                  </Button>
                </div>
              </dialog>
            </div>
          ) : null}
        </>
      ) : null}
    </main>
  );
}
