'use client';

import {
  ArrowRight,
  Bot,
  Check,
  MoveUpRight,
  Sparkles,
} from 'lucide-react';
import {
  KeyboardEvent,
  useCallback,
  useEffect,
  useId,
  useRef,
  useState,
} from 'react';

import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Textarea } from '@/components/ui/textarea';
import type { AgentResponse, Listing, Requirement } from '@/lib/types';
import { cn } from '@/lib/utils';

import { PromptLedCanvas } from './prompt-led-canvas';
import { PropertyList } from './property-list';

const exampleQueries = [
  'Trabajo desde casa y necesito mucha luz natural, silencio y estar cerca del Subte D.',
  'Busco dos dormitorios en Palermo, hasta USD 1.000. Priorizo luz natural y poco ruido por encima del balcón.',
];

type SearchState = 'idle' | 'loading';

export function SearchExperience() {
  const [query, setQuery] = useState('');
  const [error, setError] = useState('');
  const [state, setState] = useState<SearchState>('idle');
  const [answer, setAnswer] = useState('');
  const [listings, setListings] = useState<Listing[]>([]);
  const [requirements, setRequirements] = useState<Requirement[]>([]);
  const [hasSearched, setHasSearched] = useState(false);

  const reactSessionID = useId();
  const inputRef = useRef<HTMLTextAreaElement>(null);
  const resultsRef = useRef<HTMLElement>(null);
  const requestRef = useRef<AbortController | null>(null);
  const sessionRef = useRef(`browser-${reactSessionID}`);

  useEffect(() => {
    return () => requestRef.current?.abort();
  }, []);

  const startSearch = useCallback(async (nextQuery: string, returnFocus = false) => {
    const normalizedQuery = nextQuery.trim();
    if (!normalizedQuery) {
      setError('Contanos al menos una necesidad o preferencia.');
      setState('idle');
      if (returnFocus) inputRef.current?.focus();
      return Promise.reject(new Error('La consulta no puede estar vacía.'));
    }

    setQuery(nextQuery);
    setError('');
    setState('loading');
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
        throw new Error(payload.error || 'El agente local no pudo responder.');
      }

      setAnswer(payload.reply || '');
      setListings(payload.listings || []);
      setRequirements(payload.requirements || []);
      setHasSearched(true);
      setState('idle');

      // Allow DOM update, then scroll smoothly to the results section
      setTimeout(() => {
        resultsRef.current?.scrollIntoView?.({ behavior: 'smooth', block: 'start' });
      }, 50);

      return {
        status: 'complete',
        query: normalizedQuery,
        reply: payload.reply || '',
        listings: payload.listings || [],
      };
    } catch (cause) {
      if (controller.signal.aborted) throw cause;
      const message = cause instanceof Error ? cause.message : 'El agente local no pudo responder.';
      setError(message);
      setState('idle');
      if (returnFocus) inputRef.current?.focus();
      throw cause;
    } finally {
      if (requestRef.current === controller) requestRef.current = null;
    }
  }, []);

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
    void startSearch(query, true).catch(() => undefined);
  }

  function handleKeyDown(event: KeyboardEvent<HTMLTextAreaElement>) {
    if (event.key !== 'Enter' || event.shiftKey) return;
    if (event.nativeEvent.isComposing) return;
    event.preventDefault();
    void startSearch(query, true).catch(() => undefined);
  }

  function applyExample(example: string) {
    setQuery(example);
    setError('');
    setState('idle');
    inputRef.current?.focus();
  }

  return (
    <main className="site-shell">
      <header className="site-header">
        <a className="brand" href="#top" aria-label="Hausy, inicio">
          <span className="brand-mark" aria-hidden="true">
            H
          </span>
          <span>Hausy</span>
        </a>
        <nav aria-label="Navegación principal">
          <a href="#como-funciona">Cómo funciona</a>
          <a href="#principios">Principios</a>
          {hasSearched ? <a href="#resultados">Resultados</a> : null}
        </nav>
        <span className="prototype-note">Prototipo de búsqueda</span>
      </header>

      <section id="top" className="hero-section">
        <div className="hero-copy">
          <p className="eyebrow">Tu búsqueda, bien entendida</p>
          <h1>Encontrá el lugar que encaja con tu vida.</h1>
          <p className="hero-subtitle">
            Contanos cómo vivís. Hausy interpreta tus prioridades, consulta la base de propiedades y
            te presenta las opciones más afines con sus cualidades verificables.
          </p>

          <form className="query-form" onSubmit={handleSubmit} noValidate>
            <label htmlFor="property-query">Describí cómo querés vivir</label>
            <div className={cn('query-control', error && 'query-control-error')}>
              <PromptLedCanvas />
              <Textarea
                ref={inputRef}
                id="property-query"
                value={query}
                onChange={(event) => setQuery(event.target.value)}
                onKeyDown={handleKeyDown}
                aria-describedby={error ? 'query-error query-help' : 'query-help'}
                aria-invalid={Boolean(error)}
                placeholder="Ejemplo: dos dormitorios en Palermo, mucha luz y poco ruido. Puedo estirar el presupuesto si realmente vale la pena."
                rows={4}
              />
              <Button type="submit" size="lg" disabled={state === 'loading'}>
                {state === 'loading' ? 'Buscando...' : 'Buscar hogares'}
                <ArrowRight aria-hidden="true" />
              </Button>
            </div>
            <div className="form-meta">
              <p id="query-help">Enter para buscar, Shift + Enter para una nueva línea</p>
              {error ? (
                <p id="query-error" role="alert">
                  {error}
                </p>
              ) : null}
            </div>
          </form>

          <div className="examples" aria-label="Consultas de ejemplo">
            <span>Probá con</span>
            {exampleQueries.map((example, index) => (
              <button key={example} type="button" onClick={() => applyExample(example)}>
                {index === 0 ? 'Home office y Subte D' : 'Palermo con prioridades'}
                <MoveUpRight aria-hidden="true" />
              </button>
            ))}
          </div>
        </div>

        <figure className="hero-visual">
          <img
            src="/assets/buenos-aires-apartment.webp"
            alt="Living luminoso de un departamento de Buenos Aires con grandes ventanales y vegetación"
            width="1536"
            height="1024"
          />
          <figcaption>
            <span>Ejemplo visual</span>
            <strong>No es una publicación activa</strong>
          </figcaption>
        </figure>
      </section>

      {/* Results Section rendered inline once a search is made or loading */}
      {(hasSearched || state === 'loading') && (
        <section
          id="resultados"
          ref={resultsRef}
          aria-label="Análisis y resultados"
          className="results-section my-16 scroll-mt-8 space-y-8"
        >
          {/* Agent Analysis & Reasoning Box */}
          <div className="rounded-3xl border border-border bg-card/90 p-6 shadow-sm backdrop-blur-md md:p-8">
            <div className="flex items-center gap-2 text-xs font-semibold uppercase tracking-widest text-primary">
              <Bot className="h-4 w-4" />
              <span>Análisis de Hausy</span>
            </div>

            <h2 className="mt-2 font-heading text-2xl font-bold tracking-tight text-foreground sm:text-3xl">
              Respuesta de Hausy
            </h2>

            {state === 'loading' ? (
              <div className="mt-4 space-y-2">
                <div className="h-4 w-3/4 animate-pulse rounded bg-muted" />
                <div className="h-4 w-1/2 animate-pulse rounded bg-muted" />
              </div>
            ) : (
              <p className="mt-3 text-base leading-relaxed text-foreground whitespace-pre-wrap sm:text-lg">
                {answer}
              </p>
            )}

            {/* Extracted requirements pills */}
            {requirements.length > 0 ? (
              <div className="mt-5 flex flex-wrap items-center gap-2 border-t border-border/50 pt-4">
                <span className="text-xs font-medium text-muted-foreground">
                  Criterios entendidos:
                </span>
                {requirements.map((req, idx) => (
                  <Badge
                    key={`${req.type}-${req.value}-${idx}`}
                    variant="secondary"
                    className="text-xs font-medium"
                  >
                    {req.type}: {req.value}
                  </Badge>
                ))}
              </div>
            ) : null}
          </div>

          {/* List of Properties in Box Card Fashion */}
          <PropertyList listings={listings} isLoading={state === 'loading'} />
        </section>
      )}

      <section id="como-funciona" className="explanation-section">
        <div>
          <h2>Primero entiende. Después filtra.</h2>
          <p>
            Precio, ambientes y zona son datos. Luz, ruido y flexibilidad necesitan contexto. Hausy
            mantiene esa diferencia visible.
          </p>
        </div>
        <div className="principle-grid" id="principios">
          <article>
            <Check aria-hidden="true" />
            <h3>Requisitos claros</h3>
            <p>Lo que se puede comprobar se trata como dato, no como opinión.</p>
          </article>
          <article>
            <Sparkles aria-hidden="true" />
            <h3>Preferencias con matices</h3>
            <p>Las prioridades y concesiones quedan explícitas antes de ordenar resultados.</p>
          </article>
        </div>
      </section>
    </main>
  );
}
