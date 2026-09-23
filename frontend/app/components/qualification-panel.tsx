'use client';

import { useState } from 'react';

import { Button } from '@/components/ui/button';
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
}

// The micro-interview (CONTEXT.md, Qualification): three optional answers that
// let Hausy order results by whether the searcher can actually rent them.
// Signed-in searchers get their saved answers back and keep their changes.
export function QualificationPanel({ onChange }: QualificationPanelProps) {
  const [open, setOpen] = useState(false);
  const [loaded, setLoaded] = useState(false);
  const [signedIn, setSignedIn] = useState(false);
  const [guarantee, setGuarantee] = useState<string[]>([]);
  const [income, setIncome] = useState('');
  const [caucionQuoted, setCaucionQuoted] = useState('');

  async function loadProfile() {
    setLoaded(true);
    try {
      const response = await fetch('/api/me/qualification');
      if (!response.ok) return;
      const saved = (await response.json()) as Qualification;
      setSignedIn(true);
      setGuarantee(saved.guarantee ?? []);
      setIncome(saved.income_band?.[0] ?? '');
      setCaucionQuoted(saved.caucion_quoted?.[0] ?? '');
    } catch {
      // Anonymous or offline: the panel still works for this session.
    }
  }

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
        setOpen(isOpen);
        if (isOpen && !loaded) void loadProfile();
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
        <Button type="button" variant="outline" onClick={() => void submit()}>
          Usar estos datos
        </Button>
        <Button type="button" variant="ghost" onClick={() => setOpen(false)}>
          Buscar sin esto
        </Button>
      </div>
    </details>
  );
}
