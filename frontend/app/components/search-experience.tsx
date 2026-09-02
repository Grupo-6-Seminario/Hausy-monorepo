'use client';

import {
  ArrowRight,
  BedDouble,
  Check,
  MapPin,
  MoonStar,
  MoveUpRight,
  Sparkles,
  SunMedium,
  Volume2,
} from 'lucide-react';
import {
  KeyboardEvent,
  forwardRef,
  useCallback,
  useEffect,
  useRef,
  useState,
} from 'react';

import { Button } from '@/components/ui/button';
import { Textarea } from '@/components/ui/textarea';
import { cn } from '@/lib/utils';

import { AmbientCanvas } from './ambient-canvas';

const exampleQueries = [
  'Trabajo desde casa y necesito mucha luz natural, silencio y estar cerca del Subte D.',
  'Busco dos dormitorios en Palermo, hasta USD 1.000. Priorizo luz natural y poco ruido por encima del balcón.',
];

type SearchState = 'idle' | 'loading' | 'result';

export function SearchExperience() {
  const [query, setQuery] = useState('');
  const [error, setError] = useState('');
  const [state, setState] = useState<SearchState>('idle');
  const inputRef = useRef<HTMLTextAreaElement>(null);
  const resultRef = useRef<HTMLElement>(null);
  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  useEffect(() => {
    return () => {
      if (timerRef.current) clearTimeout(timerRef.current);
    };
  }, []);

  const startSearch = useCallback((nextQuery: string, returnFocus = false) => {
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
    if (timerRef.current) clearTimeout(timerRef.current);
    return new Promise<{ status: string; query: string }>((resolve) => {
      timerRef.current = setTimeout(() => {
        setState('result');
        requestAnimationFrame(() => resultRef.current?.focus());
        resolve({ status: 'complete', query: normalizedQuery });
      }, 360);
    });
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
          'Ejecuta la búsqueda de demostración con requisitos y preferencias en lenguaje natural y actualiza el resultado visible.',
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
    if (event.key === 'Enter' && (event.metaKey || event.ctrlKey)) {
      event.preventDefault();
      void startSearch(query, true).catch(() => undefined);
    }
  }

  function applyExample(example: string) {
    setQuery(example);
    setError('');
    setState('idle');
    inputRef.current?.focus();
  }

  return (
    <main className="site-shell">
      <AmbientCanvas />

      <header className="site-header">
        <a className="brand" href="#top" aria-label="Angus, inicio">
          <span className="brand-mark" aria-hidden="true">
            A
          </span>
          <span>Angus</span>
        </a>
        <nav aria-label="Navegación principal">
          <a href="#como-funciona">Cómo funciona</a>
          <a href="#principios">Principios</a>
        </nav>
        <span className="prototype-note">Prototipo de búsqueda</span>
      </header>

      <section id="top" className="hero-section">
        <div className="hero-copy">
          <p className="eyebrow">Tu búsqueda, bien entendida</p>
          <h1>Encontrá el lugar que encaja con tu vida.</h1>
          <p className="hero-subtitle">
            Contanos cómo vivís. Angus ordena prioridades, pregunta lo que falta y compara opciones por vos.
          </p>

          <form className="query-form" onSubmit={handleSubmit} noValidate>
            <label htmlFor="property-query">Describí cómo querés vivir</label>
            <div className={cn('query-control', error && 'query-control-error')}>
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
              <p id="query-help">Usá Ctrl + Enter para buscar</p>
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

      {state === 'loading' ? <SearchSkeleton /> : null}
      {state === 'result' ? <SearchResult ref={resultRef} /> : null}

      <section id="como-funciona" className="explanation-section">
        <div>
          <h2>Primero entiende. Después filtra.</h2>
          <p>
            Precio, ambientes y zona son datos. Luz, ruido y flexibilidad necesitan contexto. Angus mantiene esa diferencia visible.
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

function SearchSkeleton() {
  return (
    <section className="result-shell loading-result" aria-live="polite" aria-busy="true">
      <p>Estamos ordenando requisitos y preferencias...</p>
      <div className="skeleton-line skeleton-wide" />
      <div className="skeleton-line skeleton-medium" />
    </section>
  );
}

const SearchResult = forwardRef<HTMLElement>(function SearchResult(_, ref) {
  return (
    <section ref={ref} tabIndex={-1} className="result-shell" aria-live="polite">
      <div className="intent-summary">
        <div className="result-heading">
          <span>Lectura de tu búsqueda</span>
          <h2>Entendimos lo importante</h2>
        </div>
        <div className="intent-columns">
          <div>
            <h3>Requisitos</h3>
            <ul>
              <li>
                <MapPin aria-hidden="true" /> Palermo
              </li>
              <li>
                <BedDouble aria-hidden="true" /> 2 dormitorios
              </li>
              <li>Hasta USD 1.000</li>
            </ul>
          </div>
          <div>
            <h3>Prioridades</h3>
            <ul>
              <li>
                <SunMedium aria-hidden="true" /> Luz natural primero
              </li>
              <li>
                <Volume2 aria-hidden="true" /> Poco ruido
              </li>
              <li>
                <MoonStar aria-hidden="true" /> Balcón negociable
              </li>
            </ul>
          </div>
        </div>
      </div>

      <article className="prototype-match">
        <img
          src="/assets/buenos-aires-apartment.webp"
          alt="Vista de ejemplo del living luminoso sugerido por el prototipo"
          width="1536"
          height="1024"
        />
        <div className="match-copy">
          <span>Coincidencia de demostración</span>
          <h3>Palermo, CABA</h3>
          <p className="match-price">USD 980 por mes</p>
          <p>
            La orientación y los ventanales apoyan tu prioridad de luz. El ruido todavía requiere verificación antes de recomendarlo.
          </p>
          <Button variant="outline" type="button">
            Ver razonamiento <ArrowRight aria-hidden="true" />
          </Button>
        </div>
      </article>
    </section>
  );
});
