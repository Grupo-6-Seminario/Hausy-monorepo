import { createContext, useContext, useEffect, useState } from 'react';

// One question of the qualification form, as GET /api/eligibility/facts serves
// it. The backend owns the catalog; the form only renders it.
export interface EligibilityChoice {
  value: string;
  label: string;
}

export interface EligibilityFact {
  name: string;
  label: string;
  priority: 'high' | 'medium' | 'low';
  multiple: boolean;
  choices: EligibilityChoice[];
}

export type FactsState =
  | { kind: 'loading' }
  | { kind: 'ready'; facts: EligibilityFact[] }
  | { kind: 'failed' };

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null;
}

function parseChoice(value: unknown): EligibilityChoice | null {
  if (!isRecord(value)) return null;
  const { value: choice, label } = value;
  return typeof choice === 'string' && typeof label === 'string'
    ? { value: choice, label }
    : null;
}

function parseFact(value: unknown): EligibilityFact | null {
  if (!isRecord(value)) return null;
  const { name, label, priority, multiple, choices } = value;
  if (
    typeof name !== 'string' ||
    typeof label !== 'string' ||
    (priority !== 'high' && priority !== 'medium' && priority !== 'low') ||
    typeof multiple !== 'boolean' ||
    !Array.isArray(choices)
  ) {
    return null;
  }
  const parsed = choices.map(parseChoice);
  if (parsed.some((choice) => choice === null)) return null;
  return {
    name,
    label,
    priority,
    multiple,
    choices: parsed.filter((choice) => choice !== null),
  };
}

// parseFacts reads the endpoint's body. Anything malformed fails as a whole:
// a half-understood catalog would ask the wrong questions, and an empty one
// would show a form with none.
export function parseFacts(body: unknown): EligibilityFact[] | null {
  if (!isRecord(body) || !Array.isArray(body.facts) || body.facts.length === 0)
    return null;
  const facts = body.facts.map(parseFact);
  return facts.some((fact) => fact === null)
    ? null
    : facts.filter((fact) => fact !== null);
}

// FactsContext carries the catalog the page loaded on the server, so the form
// renders with the page instead of after a request.
export const FactsContext = createContext<EligibilityFact[] | null>(null);

// inFlight is the one request every component that mounts at the same time
// shares. It is dropped once settled: later mounts ask again, and the
// response is cacheable (max-age=300).
let inFlight: Promise<EligibilityFact[] | null> | null = null;

function requestFacts(): Promise<EligibilityFact[] | null> {
  inFlight ??= fetch('/api/eligibility/facts')
    .then(async (response) =>
      response.ok ? parseFacts(await response.json()) : null,
    )
    .catch(() => null)
    .finally(() => {
      inFlight = null;
    });
  return inFlight;
}

// useEligibilityFacts gives the form's questions: the page's catalog when it
// brought one, or one shared request otherwise.
export function useEligibilityFacts(): FactsState {
  const provided = useContext(FactsContext);
  const [state, setState] = useState<FactsState>(
    provided ? { kind: 'ready', facts: provided } : { kind: 'loading' },
  );
  useEffect(() => {
    if (provided) return;
    let mounted = true;
    void requestFacts().then((facts) => {
      if (mounted) {
        setState(facts ? { kind: 'ready', facts } : { kind: 'failed' });
      }
    });
    return () => {
      mounted = false;
    };
  }, [provided]);
  return state;
}
