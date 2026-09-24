'use client';

import { ArrowRight, Bot, MoveUpRight, RotateCcw, Square } from 'lucide-react';
import {
  KeyboardEvent,
  useCallback,
  useEffect,
  useLayoutEffect,
  useRef,
  useState,
} from 'react';

import { Button } from '@/components/ui/button';
import { Textarea } from '@/components/ui/textarea';
import type {
  Listing,
  Qualification,
  Relaxation,
  Requirement,
} from '@/lib/types';
import { readTurn } from '@/lib/read-turn';
import { cn } from '@/lib/utils';

import { AgentReply } from './agent-reply';
import { LandingDiscovery, LandingPortrait } from './landing-discovery';
import { usePointerGlow } from './pointer-glow';
import { PropertyList } from './property-list';
import {
  describeQualification,
  QualificationPanel,
} from './qualification-panel';
import { PromptLuminary } from './prompt-luminary';
import { ThemeToggle } from './theme-toggle';

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

function referencedRanks(reply: string): number[] {
  const ranks = new Set<number>();

  for (const match of reply.matchAll(/(?:#\s*|rank\s+)(\d+)\b/gi)) {
    ranks.add(Number(match[1]));
  }

  return [...ranks];
}

export function SearchExperience() {
  const [query, setQuery] = useState('');
  const [error, setError] = useState('');
  const [state, setState] = useState<SearchState>('idle');
  const [listings, setListings] = useState<Listing[]>([]);
  const [requirements, setRequirements] = useState<Requirement[]>([]);
  const [relaxations, setRelaxations] = useState<Relaxation[]>([]);
  // The micro-interview opens with the page and folds away once a search runs.
  const [qualificationOpen, setQualificationOpen] = useState(true);
  const [qualification, setQualification] = useState<Qualification>({});
  // Read inside startSearch without re-registering the WebMCP tool.
  const qualificationRef = useRef<Qualification>({});
  const [recommendedRanks, setRecommendedRanks] = useState<number[]>([]);
  const [hasSearched, setHasSearched] = useState(false);
  const [turns, setTurns] = useState<ConversationTurn[]>([]);
  const [pendingQuery, setPendingQuery] = useState('');
  // The reply as it streams in, until the turn is done.
  const [pendingReply, setPendingReply] = useState('');

  const shellRef = useRef<HTMLElement>(null);
  const inputRef = useRef<HTMLTextAreaElement>(null);
  const logRef = useRef<HTMLOListElement>(null);
  const requestRef = useRef<AbortController | null>(null);
  // One conversation per page load. useId would not do: it is derived from the
  // component's place in the tree, so every load and every visitor would share
  // one backend session and inherit each other's requirements.
  const sessionRef = useRef('');
  const turnIDRef = useRef(0);
  const isWorking = state === 'loading';
  const isWorkspace = hasSearched || isWorking;

  usePointerGlow(shellRef);

  useLayoutEffect(() => {
    const log = logRef.current;
    const latestTurn = log?.lastElementChild as HTMLElement | null;
    if (!log || !latestTurn) return;
    // Move only the history viewport. Start at the new turn so long replies
    // can be read from the beginning, without jumping the entire page.
    log.scrollTop = latestTurn.offsetTop;
  }, [turns.length, pendingQuery]);

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
      setQualificationOpen(false);
      setPendingQuery(normalizedQuery);
      setPendingReply('');
      setQuery('');
      requestRef.current?.abort();
      const controller = new AbortController();
      requestRef.current = controller;

      sessionRef.current ||= `browser-${crypto.randomUUID()}`;
      try {
        const response = await fetch('/api/agent', {
          method: 'POST',
          headers: {
            'Content-Type': 'application/json',
            Accept: 'application/x-ndjson',
          },
          body: JSON.stringify({
            session_id: sessionRef.current,
            message: normalizedQuery,
            qualification: qualificationRef.current,
          }),
          signal: controller.signal,
        });
        const payload = await readTurn(response, {
          onResults(partial) {
            setListings(partial.listings || []);
            setRequirements(partial.requirements || []);
            setRelaxations(partial.relaxations || []);
            setHasSearched(true);
          },
          onReply: (delta) => setPendingReply((text) => text + delta),
        });
        if (!response.ok || (!payload.reply && !payload.listings)) {
          throw new Error(
            payload.error || 'El agente local no pudo responder.',
          );
        }

        setListings(payload.listings || []);
        setRequirements(payload.requirements || []);
        setRelaxations(payload.relaxations || []);
        setRecommendedRanks(referencedRanks(payload.reply || ''));
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
        setPendingReply('');
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
        setPendingReply('');
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
    setPendingReply('');
    setState('idle');
    inputRef.current?.focus();
  }

  // A fresh backend session, so the planner stops reading the earlier search.
  // The qualification stays: it describes the searcher, not the search.
  function startOver() {
    requestRef.current?.abort();
    sessionRef.current = '';
    setTurns([]);
    setListings([]);
    setRequirements([]);
    setRelaxations([]);
    setRecommendedRanks([]);
    setHasSearched(false);
    setPendingQuery('');
    setPendingReply('');
    setQuery('');
    setError('');
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
      ref={shellRef}
      className="site-shell"
      data-view={isWorkspace ? 'workspace' : 'welcome'}
    >
      <header className="site-header">
        <a className="brand" href="#top" aria-label="Hausy, inicio">
          <img src="/hausy_logo.png" alt="Hausy" width="40" height="40" />
          <span>Hausy</span>
        </a>
        <p className="prototype-note">Prototipo de búsqueda</p>
        <div className="header-actions">
          <nav className="header-nav" aria-label="Accesos">
            {hasSearched ? (
              <a className="results-link" href="#resultados" data-glow>
                Ver selección{' '}
                <span aria-hidden="true">({listings.length})</span>
              </a>
            ) : null}
            {/* Full navigation on purpose: vinext only shims next/link inside Vite, not vitest. */}
            {/* oxlint-disable-next-line next/no-html-link-for-pages */}
            <a className="results-link" href="/ingresar" data-glow>
              Ingresar
            </a>
          </nav>
          <ThemeToggle />
        </div>
      </header>

      <div className="experience-frame">
        <section
          id="top"
          className="conversation-surface"
          aria-labelledby="experience-title"
          aria-busy={isWorking}
        >
          <div className="experience-intro">
            {!isWorkspace ? (
              <p className="eyebrow">Tu próximo hogar, en CABA</p>
            ) : null}
            <h1 id="experience-title">
              {isWorkspace
                ? 'Sigamos con tu búsqueda.'
                : 'Un lugar para tu forma de vivir.'}
            </h1>
            <p className="hero-subtitle">
              {isWorkspace
                ? 'Ajustá tus prioridades. Conservamos el contexto.'
                : 'Contanos qué necesitás. Comparemos opciones, con lo que sabemos y lo que falta confirmar.'}
            </p>
          </div>

          {isWorkspace ? (
            <div className="conversation-panel">
              <div className="panel-heading">
                <Bot aria-hidden="true" />
                <div>
                  <h2>Respuesta de Hausy</h2>
                </div>
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  className="new-search rounded-full"
                  onClick={startOver}
                >
                  <RotateCcw aria-hidden="true" />
                  Nueva búsqueda
                </Button>
              </div>

              <ol
                ref={logRef}
                className="conversation-log"
                role="log"
                // oxlint-disable-next-line jsx-a11y/no-noninteractive-tabindex -- The bounded history needs keyboard scrolling.
                tabIndex={0}
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
                      <AgentReply reply={turn.reply} />
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
                        {pendingReply ? (
                          <AgentReply reply={pendingReply} />
                        ) : (
                          <p>
                            Consultando el inventario y comparando tus
                            prioridades...
                          </p>
                        )}
                        <span className="working-line" aria-hidden="true" />
                      </div>
                    </output>
                  </li>
                ) : null}
              </ol>

              <p className="qualification-chips">
                {describeQualification(qualification) ? (
                  <span>Usando: {describeQualification(qualification)}</span>
                ) : (
                  <span>Sin garantía declarada</span>
                )}
                <button
                  type="button"
                  onClick={() => setQualificationOpen(true)}
                >
                  {describeQualification(qualification)
                    ? 'Editar'
                    : 'Completar'}
                </button>
              </p>

              {requirements.length > 0 ? (
                <details className="criteria">
                  <summary>
                    <span>Criterios que estoy usando</span>
                    <span>{requirements.length}</span>
                  </summary>
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
                </details>
              ) : null}
            </div>
          ) : null}

          <QualificationPanel
            open={qualificationOpen}
            onOpenChange={setQualificationOpen}
            onChange={(next) => {
              qualificationRef.current = next;
              setQualification(next);
            }}
          />

          <form className="query-form" onSubmit={handleSubmit} noValidate>
            <label htmlFor="property-query">
              {isWorkspace
                ? 'Sumá una condición o hacé una pregunta'
                : 'Describí cómo querés vivir'}
            </label>
            <div
              className="prompt-stage"
              data-prompt-stage
              data-active={isWorking ? 'true' : 'false'}
            >
              <PromptLuminary />
              <div
                className={cn('query-control', error && 'query-control-error')}
              >
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
                  rows={2}
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
                  data-glow
                  onClick={() => applyExample(example.query)}
                >
                  {example.label}
                  <MoveUpRight aria-hidden="true" />
                </button>
              ))}
            </div>
          ) : null}
        </section>

        {!isWorkspace ? <LandingPortrait /> : null}

        {isWorkspace ? (
          <section
            id="resultados"
            aria-label="Análisis y resultados"
            className="results-section"
          >
            <PropertyList
              listings={listings}
              relaxations={relaxations}
              recommendedRanks={recommendedRanks}
              isLoading={isWorking && !hasSearched}
            />
            <a className="conversation-return" href="#property-query">
              Seguir la conversación con Hausy
              <ArrowRight aria-hidden="true" />
            </a>
          </section>
        ) : null}
      </div>
      {!isWorkspace ? <LandingDiscovery onChoose={applyExample} /> : null}
    </main>
  );
}
