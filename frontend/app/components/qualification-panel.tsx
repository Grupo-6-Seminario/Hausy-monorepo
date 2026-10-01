'use client';

import { useEffect, useState } from 'react';

import { Button } from '@/components/ui/button';
import { wasSignedIn } from '@/lib/session-hint';
import type { Qualification } from '@/lib/types';

// ponytail: mirrors the eligibility_facts seed (migration 0004); serve the
// choices from the backend when a second market needs different instruments.
const guarantees = [
  { value: 'propietaria', label: 'Garantía propietaria' },
  { value: 'caucion', label: 'Seguro de caución' },
];
const incomeBands = [
  { value: '0-1000000', label: 'Hasta $1.000.000' },
  { value: '1000000-2000000', label: '$1.000.000 a $2.000.000' },
  { value: '2000000-3000000', label: '$2.000.000 a $3.000.000' },
  { value: '3000000-', label: 'Más de $3.000.000' },
];
const quoted = [
  { value: 'yes', label: 'Sí' },
  { value: 'no', label: 'No' },
];

interface QualificationPanelProps {
  onChange: (qualification: Qualification) => void;
  // What was already declared, for a panel that mounts again in another place.
  initial?: Qualification;
  // Controlled by the page when given; the panel manages itself otherwise.
  open?: boolean;
  onOpenChange?: (open: boolean) => void;
}

// describeQualification renders what a search is using, e.g.
// "garantía propietaria · $2.000.000 a $3.000.000".
export function describeQualification(q: Qualification): string {
  const parts = [
    ...guarantees
      .filter((g) => q.guarantee?.includes(g.value))
      .map((g) => g.label.toLowerCase()),
    ...incomeBands
      .filter((b) => q.income_band?.includes(b.value))
      .map((b) => b.label),
  ];
  if (q.caucion_quoted?.includes('yes')) parts.push('caución cotizada');
  return parts.join(' · ');
}

// The micro-interview (CONTEXT.md, Qualification): three optional answers that
// let Hausy order results by whether the searcher can actually rent them.
// Signed-in searchers get their saved answers back and keep their changes.
export function QualificationPanel({
  onChange,
  initial = {},
  open: controlledOpen,
  onOpenChange,
}: QualificationPanelProps) {
  const [ownOpen, setOwnOpen] = useState(false);
  const open = controlledOpen ?? ownOpen;
  const setOpen = (next: boolean) =>
    onOpenChange ? onOpenChange(next) : setOwnOpen(next);
  const [signedIn, setSignedIn] = useState(false);
  const [guarantee, setGuarantee] = useState<string[]>(initial.guarantee ?? []);
  const [income, setIncome] = useState(initial.income_band?.[0] ?? '');
  const [caucionQuoted, setCaucionQuoted] = useState(
    initial.caucion_quoted?.[0] ?? '',
  );

  // A signed-in searcher gets their saved answers back; anonymous visitors
  // keep the panel for this session only and send no request.
  useEffect(() => {
    if (!wasSignedIn()) return;
    const controller = new AbortController();
    fetch('/api/me/qualification', { signal: controller.signal })
      .then(async (response) => {
        if (!response.ok) return;
        const saved = (await response.json()) as Qualification;
        setSignedIn(true);
        setGuarantee(saved.guarantee ?? []);
        setIncome(saved.income_band?.[0] ?? '');
        setCaucionQuoted(saved.caucion_quoted?.[0] ?? '');
      })
      .catch(() => undefined);
    return () => controller.abort();
  }, []);

  async function submit() {
    const qualification: Qualification = {};
    const chosen = guarantees
      .map((g) => g.value)
      .filter((v) => guarantee.includes(v));
    if (chosen.length > 0) qualification.guarantee = chosen;
    if (income) qualification.income_band = [income];
    if (caucionQuoted) qualification.caucion_quoted = [caucionQuoted];
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
        ¿Qué garantía tenés?{' '}
        <span>Opcional · te mostramos primero donde podés aplicar</span>
      </summary>
      <fieldset>
        <legend>Garantías que podés presentar</legend>
        {guarantees.map((g) => (
          <label key={g.value}>
            <input
              type="checkbox"
              checked={guarantee.includes(g.value)}
              onChange={(event) =>
                setGuarantee((current) =>
                  event.target.checked
                    ? [...current, g.value]
                    : current.filter((v) => v !== g.value),
                )
              }
            />
            {g.label}
          </label>
        ))}
      </fieldset>
      <label className="qualification-income">
        Ingresos mensuales
        <select
          value={income}
          onChange={(event) => setIncome(event.target.value)}
        >
          <option value="">Prefiero no decir</option>
          {incomeBands.map((band) => (
            <option key={band.value} value={band.value}>
              {band.label}
            </option>
          ))}
        </select>
      </label>
      <fieldset>
        <legend>¿Ya cotizaste un seguro de caución?</legend>
        {quoted.map((q) => (
          <label key={q.value}>
            <input
              type="radio"
              name="caucion-quoted"
              checked={caucionQuoted === q.value}
              onChange={() => setCaucionQuoted(q.value)}
            />
            {q.label}
          </label>
        ))}
      </fieldset>
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
