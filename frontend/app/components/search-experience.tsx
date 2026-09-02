'use client';

import {
  ArrowRight,
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

import { Button } from '@/components/ui/button';
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { Textarea } from '@/components/ui/textarea';
import { cn } from '@/lib/utils';

import { PromptLedCanvas } from './prompt-led-canvas';

const exampleQueries = [
  'Trabajo desde casa y necesito mucha luz natural, silencio y estar cerca del Subte D.',
  'Busco dos dormitorios en Palermo, hasta USD 1.000. Priorizo luz natural y poco ruido por encima del balcón.',
];

type SearchState = 'idle' | 'loading';

interface AgentResponse {
  reply?: string;
  error?: string;
}

export function SearchExperience() {
  const [query, setQuery] = useState('');
  const [error, setError] = useState('');
  const [state, setState] = useState<SearchState>('idle');
  const [answer, setAnswer] = useState('');
  const [dialogOpen, setDialogOpen] = useState(false);
  const reactSessionID = useId();
  const inputRef = useRef<HTMLTextAreaElement>(null);
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
      if (!response.ok || !payload.reply) {
        throw new Error(payload.error || 'El agente local no pudo responder.');
      }

      setAnswer(payload.reply);
      setDialogOpen(true);
      setState('idle');
      return { status: 'complete', query: normalizedQuery, reply: payload.reply };
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
    if (event.key !== 'Enter' || event.shiftKey) return;
    // Enter also commits an IME candidate; that keystroke belongs to the editor.
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
        </nav>
        <span className="prototype-note">Prototipo de búsqueda</span>
      </header>

      <section id="top" className="hero-section">
        <div className="hero-copy">
          <p className="eyebrow">Tu búsqueda, bien entendida</p>
          <h1>Encontrá el lugar que encaja con tu vida.</h1>
          <p className="hero-subtitle">
            Contanos cómo vivís. Hausy ordena prioridades, pregunta lo que falta y compara opciones por vos.
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

      <section id="como-funciona" className="explanation-section">
        <div>
          <h2>Primero entiende. Después filtra.</h2>
          <p>
            Precio, ambientes y zona son datos. Luz, ruido y flexibilidad necesitan contexto. Hausy mantiene esa diferencia visible.
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

      <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
        <DialogContent className="agent-response-dialog" showCloseButton={false}>
          <DialogHeader>
            <span className="agent-response-kicker">Tu búsqueda llegó al agente</span>
            <DialogTitle>Respuesta de Hausy</DialogTitle>
          </DialogHeader>
          <DialogDescription className="agent-response-copy">{answer}</DialogDescription>
          <DialogFooter>
            <DialogClose render={<Button type="button" />}>Cerrar</DialogClose>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </main>
  );
}
