'use client';

import { ArrowRight, Bot, MoveUpRight, Square } from 'lucide-react';
import {
  KeyboardEvent,
  useCallback,
  useEffect,
  useId,
  useRef,
  useState,
} from 'react';

import { Button } from '@/components/ui/button';
import { Textarea } from '@/components/ui/textarea';
import type { AgentResponse, Listing, Requirement } from '@/lib/types';
import { cn } from '@/lib/utils';

import { PromptLedCanvas } from './prompt-led-canvas';
import { PropertyList } from './property-list';

const exampleQueries = [
  {
    label: 'Home office y Subte D',
    query:
      'Trabajo desde casa y necesito mucha luz natural, silencio y estar cerca del Subte D.',
  },
  {
    label: 'Palermo con prioridades',
    query:
      'Busco dos dormitorios en Palermo, hasta USD 1.000. Priorizo luz natural y poco ruido por encima del balcón.',
  },
];

type SearchState = 'idle' | 'loading';

interface ConversationTurn {
  id: number;
  query: string;
  reply: string;
}

export function SearchExperience() {
  const [query, setQuery] = useState('');
  const [error, setError] = useState('');
  const [state, setState] = useState<SearchState>('idle');
  const [listings, setListings] = useState<Listing[]>([]);
  const [requirements, setRequirements] = useState<Requirement[]>([]);
  const [hasSearched, setHasSearched] = useState(false);
  const [turns, setTurns] = useState<ConversationTurn[]>([]);
  const [pendingQuery, setPendingQuery] = useState('');

  const reactSessionID = useId();
  const inputRef = useRef<HTMLTextAreaElement>(null);
  const requestRef = useRef<AbortController | null>(null);
  const sessionRef = useRef(`browser-${reactSessionID}`);
  const turnIDRef = useRef(0);
  const isWorking = state === 'loading';
  const isWorkspace = hasSearched || isWorking;

  useEffect(() => {
    return () => requestRef.current?.abort();
  }, []);

  const startSearch = useCallback(
    async (nextQuery: string, returnFocus = false) => {
      const normalizedQuery = nextQuery.trim();
      if (!normalizedQuery) {
        setError('Contanos al menos una necesidad o preferencia.');
        setState('idle');
        if (returnFocus) inputRef.current?.focus();
        return Promise.reject(new Error('La consulta no puede estar vacía.'));
      }

      setError('');
      setState('loading');
      setPendingQuery(normalizedQuery);
      setQuery('');
      requestRef.current?.abort();
      const controller = new AbortController();
      requestRef.current = controller;

      try {
        const response = await fetch('/api/agent', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({
            session_id: sessionRef.current,
            message: normalizedQuery,
          }),
          signal: controller.signal,
        });
        const payload = (await response.json()) as AgentResponse;
        if (!response.ok || (!payload.reply && !payload.listings)) {
          throw new Error(
            payload.error || 'El agente local no pudo responder.',
          );
        }

        setListings(payload.listings || []);
        setRequirements(payload.requirements || []);
        setHasSearched(true);
        setTurns((currentTurns) => [
          ...currentTurns,
          {
            id: ++turnIDRef.current,
            query: normalizedQuery,
            reply: payload.reply || '',
          },
        ]);
        setPendingQuery('');
        setState('idle');
        if (returnFocus) inputRef.current?.focus();

        return {
          status: 'complete',
          query: normalizedQuery,
          reply: payload.reply || '',
          listings: payload.listings || [],
        };
      } catch (cause) {
        if (controller.signal.aborted) throw cause;
        const message =
          cause instanceof Error
            ? cause.message
            : 'El agente local no pudo responder.';
        setError(message);
        setPendingQuery('');
        setQuery(normalizedQuery);
        setState('idle');
        if (returnFocus) inputRef.current?.focus();
        throw cause;
      } finally {
        if (requestRef.current === controller) requestRef.current = null;
      }
    },
    [],
  );

  useEffect(() => {
    const context = document.modelContext;
    if (!context?.registerTool) return;

    const lifecycle = new AbortController();
    const registration = context.registerTool(
      {
        name: 'search_properties',
        title: 'Buscar propiedades',
        description:
          'Ejecuta la búsqueda con requisitos y preferencias en lenguaje natural y actualiza los resultados visibles en pantalla.',
        inputSchema: {
          type: 'object',
          properties: {
            query: { type: 'string', minLength: 1 },
          },
          required: ['query'],
          additionalProperties: false,
        },
        annotations: { readOnlyHint: false, untrustedContentHint: false },
        execute(input: unknown) {
          if (
            typeof input !== 'object' ||
            input === null ||
            !('query' in input) ||
            typeof input.query !== 'string'
          ) {
            throw new Error('Se requiere una consulta de texto.');
          }
          return startSearch(input.query);
        },
      },
      { signal: lifecycle.signal },
    );

    void Promise.resolve(registration).catch(() => lifecycle.abort());
    return () => lifecycle.abort();
  }, [startSearch]);

  function handleSubmit(event: { preventDefault(): void }) {
    event.preventDefault();
    if (isWorking) return;
    void startSearch(query, true).catch(() => undefined);
  }

  function handleKeyDown(event: KeyboardEvent<HTMLTextAreaElement>) {
    if (event.key !== 'Enter' || event.shiftKey) return;
    if (event.nativeEvent.isComposing) return;
    event.preventDefault();
    if (isWorking) return;
    void startSearch(query, true).catch(() => undefined);
  }

  function stopSearch() {
    requestRef.current?.abort();
    setQuery(pendingQuery);
    setPendingQuery('');
    setState('idle');
    inputRef.current?.focus();
  }

  function applyExample(example: string) {
    setQuery(example);
    setError('');
    inputRef.current?.focus();
  }

  return (
    <main
      className="site-shell"
      data-view={isWorkspace ? 'workspace' : 'welcome'}
    >
      <header className="site-header">
        <a className="brand" href="#top" aria-label="Hausy, inicio">
          <span className="brand-mark" aria-hidden="true">
            H/
          </span>
          <span>Hausy</span>
        </a>
        <p className="prototype-note">Prototipo de búsqueda</p>
        {hasSearched ? (
          <a className="results-link" href="#resultados">
            Ver selección <span aria-hidden="true">({listings.length})</span>
          </a>
        ) : null}
      </header>

      <div className="experience-frame">
        <section
          id="top"
          className="conversation-surface"
          aria-labelledby="experience-title"
          aria-busy={isWorking}
        >
          <div className="experience-intro">
            <p className="eyebrow">Búsqueda inmobiliaria personal</p>
            <h1 id="experience-title">
              {isWorkspace
                ? 'Tu búsqueda, en conversación.'
                : 'Encontrá el lugar que encaja con tu vida.'}
            </h1>
            <p className="hero-subtitle">
              {isWorkspace
                ? 'Afiná prioridades, preguntá por una propiedad o cambiá una condición sin empezar de nuevo.'
                : 'Contanos cómo vivís. Hausy separa requisitos, preferencias e inferencias antes de comparar opciones.'}
            </p>
          </div>

          {isWorkspace ? (
            <div className="conversation-panel">
              <div className="panel-heading">
                <Bot aria-hidden="true" />
                <div>
                  <p>Agente comprador</p>
                  <h2>Respuesta de Hausy</h2>
                </div>
              </div>

              <ol
                className="conversation-log"
                role="log"
                aria-label="Conversación con Hausy"
              >
                {turns.map((turn) => (
                  <li key={turn.id} className="conversation-turn">
                    <div className="message message-user">
                      <p className="conversation-speaker">Vos</p>
                      <p>{turn.query}</p>
                    </div>
                    <div className="message message-agent">
                      <p className="conversation-speaker">Hausy</p>
                      <p>{turn.reply}</p>
                    </div>
                  </li>
                ))}
                {isWorking ? (
                  <li>
                    <output className="conversation-pending" aria-live="polite">
                      <div className="message message-user">
                        <p className="conversation-speaker">Vos</p>
                        <p>{pendingQuery}</p>
                      </div>
                      <div className="message message-agent message-working">
                        <p className="conversation-speaker">Hausy</p>
                        <p>
                          Consultando el inventario y comparando tus
                          prioridades...
                        </p>
                        <span className="working-line" aria-hidden="true" />
                      </div>
                    </output>
                  </li>
                ) : null}
              </ol>

              {requirements.length > 0 ? (
                <div className="criteria" aria-label="Criterios entendidos">
                  <p>Criterios entendidos</p>
                  <ul>
                    {requirements.map((requirement, index) => (
                      <li
                        key={`${requirement.type}-${requirement.value}-${index}`}
                      >
                        <span>{requirement.type.replaceAll('_', ' ')}</span>
                        {requirement.value}
                      </li>
                    ))}
                  </ul>
                </div>
              ) : null}
            </div>
          ) : null}

          <form className="query-form" onSubmit={handleSubmit} noValidate>
            <label htmlFor="property-query">
              {isWorkspace
                ? 'Sumá una condición o hacé una pregunta'
                : 'Describí cómo querés vivir'}
            </label>
            <div
              className={cn('query-control', error && 'query-control-error')}
            >
              <PromptLedCanvas />
              <Textarea
                ref={inputRef}
                id="property-query"
                value={query}
                onChange={(event) => {
                  setQuery(event.target.value);
                  if (error) setError('');
                }}
                onKeyDown={handleKeyDown}
                aria-describedby={
                  error ? 'query-error query-help' : 'query-help'
                }
                aria-invalid={Boolean(error)}
                placeholder={
                  isWorkspace
                    ? 'Ejemplo: priorizá silencio aunque quede un poco más lejos del subte.'
                    : 'Ejemplo: dos dormitorios en Palermo, mucha luz y poco ruido. Puedo estirar el presupuesto si realmente vale la pena.'
                }
                rows={isWorkspace ? 2 : 4}
              />
              {isWorking ? (
                <Button
                  type="button"
                  size="lg"
                  variant="outline"
                  onClick={stopSearch}
                >
                  Detener
                  <Square aria-hidden="true" />
                </Button>
              ) : (
                <Button type="submit" size="lg" variant="outline">
                  Buscar hogares
                  <ArrowRight aria-hidden="true" />
                </Button>
              )}
            </div>
            <div className="form-meta">
              <p id="query-help">
                Enter para enviar · Shift + Enter para una nueva línea
              </p>
              {error ? (
                <p id="query-error" role="alert">
                  {error}
                </p>
              ) : null}
            </div>
          </form>

          {!isWorkspace ? (
            <div className="examples" aria-label="Consultas de ejemplo">
              <span>Podés empezar por</span>
              {exampleQueries.map((example) => (
                <button
                  key={example.label}
                  type="button"
                  onClick={() => applyExample(example.query)}
                >
                  {example.label}
                  <MoveUpRight aria-hidden="true" />
                </button>
              ))}
            </div>
          ) : null}

          <p className="trust-note">
            Los datos publicados y las inferencias del modelo aparecen
            identificados por separado.
          </p>
        </section>

        {isWorkspace ? (
          <section
            id="resultados"
            aria-label="Análisis y resultados"
            className="results-section"
          >
            <PropertyList
              listings={listings}
              isLoading={isWorking && !hasSearched}
            />
          </section>
        ) : null}
      </div>
    </main>
  );
}
