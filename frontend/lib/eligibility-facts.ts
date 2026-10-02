import { useEffect, useState } from 'react';

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
// a half-understood catalog would ask the wrong questions.
export function parseFacts(body: unknown): EligibilityFact[] | null {
  if (!isRecord(body) || !Array.isArray(body.facts)) return null;
  const facts = body.facts.map(parseFact);
  return facts.some((fact) => fact === null)
    ? null
    : facts.filter((fact) => fact !== null);
}

// useEligibilityFacts loads the form's questions. The response is cacheable
// (max-age=300), so each caller asking is cheap.
export function useEligibilityFacts(): FactsState {
  const [state, setState] = useState<FactsState>({ kind: 'loading' });
  useEffect(() => {
    const controller = new AbortController();
    fetch('/api/eligibility/facts', { signal: controller.signal })
      .then(async (response) => {
        const facts = response.ok ? parseFacts(await response.json()) : null;
        setState(facts ? { kind: 'ready', facts } : { kind: 'failed' });
      })
      .catch(() => {
        if (!controller.signal.aborted) setState({ kind: 'failed' });
      });
    return () => controller.abort();
  }, []);
  return state;
}
