'use client';

import { ArrowRight, Square } from 'lucide-react';
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
  ClarificationQuestion,
  Listing,
  Qualification,
  Relaxation,
  Requirement,
} from '@/lib/types';
import { readTurn } from '@/lib/read-turn';
import { cn } from '@/lib/utils';
import { morph } from '@/lib/view-transition';

import { AgentReply } from './agent-reply';
import { LandingDiscovery } from './landing-discovery';
import { usePointerGlow } from './pointer-glow';
import { PropertyList } from './property-list';
import {
  describeQualification,
  QualificationPanel,
} from './qualification-panel';
import { PromptLuminary } from './prompt-luminary';
import { ThemeToggle } from './theme-toggle';

// Neighborhoods the committed catalog covers, so an example never starts empty.
const exampleQueries = [
  {
    label: '2 amb en Palermo, con luz',
    query:
      'Busco 2 ambientes en Palermo con mucha luz natural. Puedo estirar un poco el presupuesto si vale la pena.',
  },
  {
    label: 'Home office y Subte D',
    query:
      'Trabajo desde casa y necesito mucha luz natural, silencio y estar cerca del Subte D.',
  },
  {
    label: 'Con mascota en Congreso',
    query:
      'Busco un departamento en Congreso que acepte mascotas, con balcón si es posible.',
  },
];

type SearchState = 'idle' | 'loading';
const chatStorageKey = 'hausy-browser-chat';
const queryStorageKey = 'hausy-browser-query';
const pendingStorageKey = 'hausy-browser-pending';

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
  const [clarification, setClarification] = useState<ClarificationQuestion | null>(null);
  const [selectedChoices, setSelectedChoices] = useState<string[]>([]);
  const [otherOpen, setOtherOpen] = useState(false);
  const [otherAnswer, setOtherAnswer] = useState('');

  const shellRef = useRef<HTMLElement>(null);
  const inputRef = useRef<HTMLTextAreaElement>(null);
  const logRef = useRef<HTMLOListElement>(null);
  const requestRef = useRef<AbortController | null>(null);
  // A browser chat survives a reload; a new search explicitly clears it.
  const sessionRef = useRef('');
  const turnIDRef = useRef(0);
  const isWorking = state === 'loading';
  const isWorkspace = hasSearched || isWorking || clarification !== null;

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

  useEffect(() => {
    const saved = window.sessionStorage.getItem(chatStorageKey);
    if (!saved) return;
    sessionRef.current = saved;
    const controller = new AbortController();
    void fetch(`/api/agent?session_id=${encodeURIComponent(saved)}`, { signal: controller.signal })
      .then(async (response) => (await response.json()) as { clarification?: ClarificationQuestion })
      .then((payload: { clarification?: ClarificationQuestion }) => {
        if (!controller.signal.aborted && payload.clarification) {
          setClarification(payload.clarification);
          setQualificationOpen(false);
        } else if (!controller.signal.aborted && window.sessionStorage.getItem(pendingStorageKey)) {
          setQuery(window.sessionStorage.getItem(queryStorageKey) || '');
          setError('La pregunta anterior ya no está disponible. Podés editar la búsqueda y volver a enviarla.');
          window.sessionStorage.removeItem(pendingStorageKey);
        }
      })
      .catch(() => undefined);
    return () => controller.abort();
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

      requestRef.current?.abort();
      const controller = new AbortController();
      requestRef.current = controller;

      sessionRef.current ||= `browser-${crypto.randomUUID()}`;
      window.sessionStorage.setItem(chatStorageKey, sessionRef.current);
      window.sessionStorage.setItem(queryStorageKey, normalizedQuery);
      try {
        // The request waits for the new view: its results render into it.
        // Read from the page: this callback outlives the render it came from.
        const fromWelcome = shellRef.current?.dataset.view === 'welcome';
        await morph(fromWelcome ? 'workspace' : 'turn', () => {
          setError('');
          setState('loading');
          setQualificationOpen(false);
          setPendingQuery(normalizedQuery);
          setPendingReply('');
          setQuery('');
        });
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
        if (!response.ok || (!payload.reply && !payload.listings && !payload.clarification)) {
          throw new Error(
            payload.error || 'El agente local no pudo responder.',
          );
        }

        if (payload.clarification) {
          setClarification(payload.clarification);
          window.sessionStorage.setItem(pendingStorageKey, '1');
          setSelectedChoices([]);
          setOtherOpen(false);
          setPendingQuery('');
          setPendingReply('');
          setState('idle');
          return { status: 'clarification', question: payload.clarification };
        }

        setListings(payload.listings || []);
        window.sessionStorage.removeItem(pendingStorageKey);
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

  async function answerClarification(answer: { question_id: string; selected?: string[]; other?: string; action?: string }) {
    if (!clarification || isWorking) return;
    const original = clarification;
    setState('loading');
    setError('');
    try {
      const response = await fetch('/api/agent', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', Accept: 'application/x-ndjson' },
        body: JSON.stringify({ session_id: sessionRef.current, answer, qualification: qualificationRef.current }),
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
      if (!response.ok || payload.error) throw new Error(payload.error || 'No pudimos continuar la búsqueda.');
      if (answer.action === 'edit') {
        setClarification(null);
        window.sessionStorage.removeItem(pendingStorageKey);
        setQuery(original.request || '');
        setPendingReply('');
        setState('idle');
        inputRef.current?.focus();
        return;
      }
      if (payload.clarification) {
        setClarification(payload.clarification);
        window.sessionStorage.setItem(pendingStorageKey, '1');
        setSelectedChoices([]);
        setOtherOpen(Boolean(payload.clarification_hint));
        if (!payload.clarification_hint) setOtherAnswer('');
        setError(payload.clarification_hint || '');
      } else {
        setClarification(null);
        window.sessionStorage.removeItem(pendingStorageKey);
        setListings(payload.listings || []);
        setRequirements(payload.requirements || []);
        setRelaxations(payload.relaxations || []);
        setRecommendedRanks(referencedRanks(payload.reply || ''));
        setHasSearched(true);
        setTurns((current) => [...current, { id: ++turnIDRef.current, query: original.request || original.source, reply: payload.reply || '' }]);
      }
      setPendingReply('');
      setState('idle');
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : 'No pudimos continuar la búsqueda.');
      setState('idle');
    }
  }

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
    window.sessionStorage.removeItem(chatStorageKey);
    window.sessionStorage.removeItem(queryStorageKey);
    window.sessionStorage.removeItem(pendingStorageKey);
    setClarification(null);
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

  // The landing's closing call to action brings the reader back to the prompt.
  function returnToComposer() {
    inputRef.current?.scrollIntoView({ block: 'center' });
    inputRef.current?.focus({ preventScroll: true });
  }

  const qualificationPanel = (
    <QualificationPanel
      initial={qualification}
      open={qualificationOpen}
      onOpenChange={setQualificationOpen}
      onChange={(next) => {
        qualificationRef.current = next;
        setQualification(next);
      }}
    />
  );
  const declared = describeQualification(qualification);

  return (
    <main
      ref={shellRef}
      className="site-shell"
      data-view={isWorkspace ? 'workspace' : 'welcome'}
    >
      <header className="site-header">
        <a className="brand" href="#top" aria-label="Hausy, inicio">
          hausy<span className="brand-mark" aria-hidden="true" />
        </a>
        {isWorkspace ? null : (
          <nav className="header-sections" aria-label="Secciones">
            <a href="#como-funciona">Cómo funciona</a>
            <a href="#principios">Principios</a>
          </nav>
        )}
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
            <a className="results-link header-agency" href="/inmobiliaria" data-glow>
              Soy inmobiliaria
            </a>
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
              <p className="eyebrow">
                <span className="brand-dot" aria-hidden="true" />
                Alquileres en CABA
              </p>
            ) : null}
            <h1 id="experience-title">
              {clarification && !hasSearched ? (
                'Ajustemos un detalle de tu búsqueda.'
              ) : isWorkspace ? (
                'Tu búsqueda'
              ) : (
                <>
                  Primero, a qué podés acceder. <em>Después, cuál te gusta.</em>
                </>
              )}
            </h1>
            {isWorkspace && (hasSearched || !clarification) ? (
              <button type="button" className="new-search" onClick={startOver}>
                Empezar de nuevo
              </button>
            ) : (
              <p className="hero-subtitle">
                {clarification
                  ? 'Tu respuesta nos ayuda a buscar con el criterio que tenías en mente.'
                  : 'Contá qué buscás como se lo contarías a alguien. Antes de mostrarte propiedades, cruzamos tu garantía y tu ingreso con lo que pide cada aviso.'}
              </p>
            )}
          </div>

          {isWorkspace && (hasSearched || !clarification) ? (
            <div className="conversation-panel">
              {requirements.length > 0 ? (
                <ul className="criteria" aria-label="Criterios que estoy usando">
                  {requirements.map((requirement, index) => (
                    <li
                      key={`${requirement.type}-${requirement.value}-${index}`}
                    >
                      <span>{requirement.type.replaceAll('_', ' ')}</span>{' '}
                      {requirement.value}
                    </li>
                  ))}
                </ul>
              ) : null}

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
            </div>
          ) : null}

          {clarification ? (
            <section className="clarification-card" aria-labelledby="clarification-prompt">
              <p className="eyebrow">Una pregunta para afinar la búsqueda</p>
              {clarification.request ? <p className="clarification-request">Vos: {clarification.request}</p> : null}
              <h2 id="clarification-prompt">{clarification.prompt}</h2>
              {clarification.kind !== 'unsupported' ? (
                <fieldset disabled={isWorking}>
                  <legend>Elegí {clarification.multi ? 'una o más opciones' : 'una opción'}</legend>
                  {(clarification.choices || []).map((choice) => (
                    <label key={choice.id} className="clarification-choice">
                      <input
                        type={clarification.multi ? 'checkbox' : 'radio'}
                        name="clarification-choice"
                        value={choice.id}
                        checked={selectedChoices.includes(choice.id)}
                        onChange={() => { setOtherOpen(false); setSelectedChoices((current) => clarification.multi ? current.includes(choice.id) ? current.filter((id) => id !== choice.id) : [...current, choice.id] : [choice.id]); }}
                      />
                      <span>{choice.label}</span>
                    </label>
                  ))}
                </fieldset>
              ) : null}
              {clarification.kind !== 'unsupported' ? (
                <>
                  <button type="button" className="clarification-other" onClick={() => { setOtherOpen(true); setSelectedChoices([]); }}>Ninguna de estas</button>
                  {otherOpen ? <input aria-label="Tu respuesta" value={otherAnswer} onChange={(event) => setOtherAnswer(event.target.value)} placeholder="Contanos qué querías decir" /> : null}
                </>
              ) : null}
              <div className="clarification-actions">
                {clarification.kind === 'unsupported' && clarification.can_remove ? (
                  <Button type="button" disabled={isWorking} onClick={() => void answerClarification({ question_id: clarification.id, action: 'remove' })}>Quitar condición y buscar</Button>
                ) : clarification.kind !== 'unsupported' ? (
                  <Button type="button" disabled={isWorking || (otherOpen ? !otherAnswer.trim() : selectedChoices.length === 0)} onClick={() => void answerClarification({ question_id: clarification.id, ...(otherOpen ? { other: otherAnswer.trim() } : { selected: selectedChoices }) })}>Continuar búsqueda</Button>
                ) : null}
                {clarification.kind === 'qualification' ? <Button type="button" variant="outline" disabled={isWorking} onClick={() => void answerClarification({ question_id: clarification.id, action: 'decline' })}>Prefiero no decir</Button> : null}
                <Button type="button" variant="outline" disabled={isWorking} onClick={() => void answerClarification({ question_id: clarification.id, action: 'edit' })}>Editar búsqueda</Button>
              </div>
              {error ? <p role="alert">{error}</p> : null}
            </section>
          ) : <search><form className="query-form" onSubmit={handleSubmit} noValidate>
            <label htmlFor="property-query">
              {isWorkspace ? 'Sumá a tu búsqueda' : '¿Qué estás buscando?'}
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
                      ? 'Sumá algo: “que tenga balcón”, “priorizá el silencio”…'
                      : 'Ej: 2 ambientes en Palermo, mucha luz y poco ruido. Hasta $850.000, puedo estirar un poco si vale la pena.'
                  }
                  rows={2}
                />
                <p id="query-help" className="query-hint">
                  {isWorkspace
                    ? 'Enter para sumar a tu búsqueda'
                    : 'Enter para buscar · Shift + Enter para nueva línea'}
                </p>
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
                    {isWorkspace ? 'Sumar' : 'Empezar búsqueda'}
                    {isWorkspace ? null : <ArrowRight aria-hidden="true" />}
                  </Button>
                )}
              </div>
            </div>
            {error ? (
              <p id="query-error" className="query-error" role="alert">
                {error}
              </p>
            ) : null}
          </form></search>}

          {/* Before the first results there is no results column to hold it. */}
          {isWorkspace && clarification && !hasSearched ? qualificationPanel : null}

          {!isWorkspace ? (
            <>
              <div className="examples" aria-label="Consultas de ejemplo">
                <span>Probá con</span>
                {exampleQueries.map((example) => (
                  <button
                    key={example.label}
                    type="button"
                    onClick={() => applyExample(example.query)}
                  >
                    {example.label}
                  </button>
                ))}
              </div>
              {qualificationPanel}
            </>
          ) : null}
        </section>

        {isWorkspace && (hasSearched || !clarification && isWorking) ? (
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
            >
              <div className="situation-bar">
                <p className="qualification-chips">
                  <span className="situation-label">Tu situación</span>
                  <span data-declared={Boolean(declared)}>
                    {declared || 'Sin garantía declarada'}
                  </span>
                  <button
                    type="button"
                    onClick={() => setQualificationOpen(true)}
                  >
                    {declared ? 'Editar' : 'Completar'}
                  </button>
                </p>
                {qualificationOpen ? qualificationPanel : null}
              </div>
            </PropertyList>
            <a className="conversation-return" href="#property-query">
              Seguir la conversación con Hausy
              <ArrowRight aria-hidden="true" />
            </a>
          </section>
        ) : null}
      </div>
      {!isWorkspace ? <LandingDiscovery onStart={returnToComposer} /> : null}
    </main>
  );
}
