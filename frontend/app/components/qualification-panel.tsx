'use client';

import { useEffect, useState } from 'react';

import { Button } from '@/components/ui/button';
import {
  type EligibilityFact,
  useEligibilityFacts,
} from '@/lib/eligibility-facts';
import { wasSignedIn } from '@/lib/session-hint';
import type { Qualification } from '@/lib/types';

interface QualificationPanelProps {
  onChange: (qualification: Qualification) => void;
  // What was already declared, for a panel that mounts again in another place.
  initial?: Qualification;
  // Controlled by the page when given; the panel manages itself otherwise.
  open?: boolean;
  onOpenChange?: (open: boolean) => void;
}

// A yes/no fact reads as a phrase in the summary only when the answer is yes.
const yesPhrases: Record<string, string> = {
  income_documented: 'ingresos comprobables',
  caucion_quoted: 'caución cotizada',
};

function isYesNo(fact: EligibilityFact): boolean {
  return (
    fact.choices.length === 2 &&
    fact.choices.every(
      (choice) => choice.value === 'yes' || choice.value === 'no',
    )
  );
}

// describeQualification renders what a search is using, in the catalog's
// order, e.g. "garantía propietaria · $2.000.000 a $3.000.000".
export function describeQualification(
  q: Qualification,
  facts: EligibilityFact[],
): string {
  const parts: string[] = [];
  for (const fact of facts) {
    const declared = q[fact.name] ?? [];
    const chosen = fact.choices.filter((c) => declared.includes(c.value));
    if (chosen.length === 0) continue;
    if (isYesNo(fact)) {
      const phrase = yesPhrases[fact.name];
      if (phrase && declared.includes('yes')) parts.push(phrase);
    } else if (fact.name === 'pets' && declared.includes('none')) {
      parts.push('sin mascotas');
    } else if (fact.multiple) {
      parts.push(chosen.map((c) => c.label.toLowerCase()).join(', '));
    } else {
      parts.push(chosen[0].label);
    }
  }
  return parts.join(' · ');
}

function isQualification(value: unknown): value is Qualification {
  return (
    typeof value === 'object' &&
    value !== null &&
    Object.values(value).every(
      (values) =>
        Array.isArray(values) && values.every((v) => typeof v === 'string'),
    )
  );
}

// The micro-interview (CONTEXT.md, Qualification): optional answers that let
// Hausy order results by whether the searcher can actually rent them. The
// questions come from the backend; without them the panel stays out of the way
// and search continues without a qualification. Signed-in searchers get their
// saved answers back and keep their changes.
export function QualificationPanel({
  onChange,
  initial = {},
  open: controlledOpen,
  onOpenChange,
}: QualificationPanelProps) {
  const factsState = useEligibilityFacts();
  const [ownOpen, setOwnOpen] = useState(false);
  const open = controlledOpen ?? ownOpen;
  const setOpen = (next: boolean) =>
    onOpenChange ? onOpenChange(next) : setOwnOpen(next);
  const [signedIn, setSignedIn] = useState(false);
  const [answers, setAnswers] = useState<Qualification>(initial);

  // A signed-in searcher gets their saved answers back; anonymous visitors
  // keep the panel for this session only and send no request.
  useEffect(() => {
    if (!wasSignedIn()) return;
    const controller = new AbortController();
    fetch('/api/me/qualification', { signal: controller.signal })
      .then(async (response) => {
        if (!response.ok) return;
        const saved: unknown = await response.json();
        if (!isQualification(saved)) return;
        setSignedIn(true);
        setAnswers(saved);
      })
      .catch(() => undefined);
    return () => controller.abort();
  }, []);

  if (factsState.kind !== 'ready') return null;
  const { facts } = factsState;

  // "none" stands alone, as the backend requires: checking it clears the
  // rest, and checking anything else clears it.
  const toggle = (fact: string, value: string, checked: boolean) =>
    setAnswers((current) => {
      const values = current[fact] ?? [];
      const kept = value === 'none' ? [] : values.filter((v) => v !== 'none');
      return {
        ...current,
        [fact]: checked ? [...kept, value] : values.filter((v) => v !== value),
      };
    });
  const choose = (fact: string, value: string) =>
    setAnswers((current) => ({ ...current, [fact]: value ? [value] : [] }));

  async function submit() {
    const qualification: Qualification = {};
    for (const fact of facts) {
      const declared = answers[fact.name] ?? [];
      const chosen = fact.choices
        .map((c) => c.value)
        .filter((v) => declared.includes(v));
      if (chosen.length > 0) qualification[fact.name] = chosen;
    }
    if (signedIn) {
      await fetch('/api/me/qualification', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(qualification),
      }).catch(() => undefined);
    }
    onChange(qualification);
    setOpen(false);
  }

  return (
    <details
      className="qualification-panel"
      open={open}
      onToggle={(event) => {
        const isOpen = (event.target as HTMLDetailsElement).open;
        if (isOpen !== open) setOpen(isOpen);
      }}
    >
      <summary>
        Tu situación{' '}
        <span>Opcional · te mostramos primero donde podés aplicar</span>
      </summary>
      {facts.map((fact) => {
        const declared = answers[fact.name] ?? [];
        // More than two single choices read better as a list than as chips.
        if (!fact.multiple && fact.choices.length > 2) {
          return (
            <label key={fact.name} className="qualification-income">
              {fact.label}
              <select
                value={declared[0] ?? ''}
                onChange={(event) => choose(fact.name, event.target.value)}
              >
                <option value="">Prefiero no decir</option>
                {fact.choices.map((choice) => (
                  <option key={choice.value} value={choice.value}>
                    {choice.label}
                  </option>
                ))}
              </select>
            </label>
          );
        }
        return (
          <fieldset key={fact.name}>
            <legend>{fact.label}</legend>
            {fact.choices.map((choice) => (
              <label key={choice.value}>
                <input
                  type={fact.multiple ? 'checkbox' : 'radio'}
                  name={fact.multiple ? undefined : fact.name}
                  checked={declared.includes(choice.value)}
                  onChange={(event) =>
                    fact.multiple
                      ? toggle(fact.name, choice.value, event.target.checked)
                      : choose(fact.name, choice.value)
                  }
                />
                {choice.label}
              </label>
            ))}
          </fieldset>
        );
      })}
      <div className="qualification-actions">
        <Button type="button" onClick={() => void submit()}>
          Usar estos datos
        </Button>
        <Button type="button" variant="ghost" onClick={() => setOpen(false)}>
          Buscar sin esto
        </Button>
      </div>
    </details>
  );
}
