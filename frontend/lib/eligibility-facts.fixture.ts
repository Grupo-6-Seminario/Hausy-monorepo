import type { EligibilityFact } from './eligibility-facts';

const yesNo = [
  { value: 'yes', label: 'Sí' },
  { value: 'no', label: 'No' },
];

// GET /api/eligibility/facts as migration 0007 seeds it, for tests.
export const factsFixture: EligibilityFact[] = [
  {
    name: 'guarantee',
    label: 'Garantía',
    priority: 'high',
    multiple: true,
    choices: [
      { value: 'propietaria', label: 'Garantía propietaria' },
      { value: 'caucion', label: 'Seguro de caución' },
      { value: 'recibos_garante', label: 'Recibos de sueldo de un garante' },
    ],
  },
  {
    name: 'income_documented',
    label: '¿Podés comprobar tus ingresos?',
    priority: 'high',
    multiple: false,
    choices: yesNo,
  },
  {
    name: 'pets',
    label: 'Mascotas',
    priority: 'high',
    multiple: true,
    choices: [
      { value: 'none', label: 'Ninguna' },
      { value: 'dog', label: 'Perro' },
      { value: 'cat', label: 'Gato' },
    ],
  },
  {
    name: 'income_band',
    label: 'Ingresos mensuales',
    priority: 'medium',
    multiple: false,
    choices: [
      { value: '0-1000000', label: 'Hasta $1.000.000' },
      { value: '1000000-2000000', label: '$1.000.000 a $2.000.000' },
      { value: '2000000-3000000', label: '$2.000.000 a $3.000.000' },
      { value: '3000000-', label: 'Más de $3.000.000' },
    ],
  },
  {
    name: 'caucion_quoted',
    label: '¿Ya cotizaste un seguro de caución?',
    priority: 'medium',
    multiple: false,
    choices: yesNo,
  },
];

// withFacts answers the catalog request itself and hands every other request
// to fetchImpl, so a test's own fetch mock sees only the calls it is about.
export function withFacts(
  fetchImpl: (
    input: RequestInfo | URL,
    init?: RequestInit,
  ) => Promise<Response>,
) {
  return (input: RequestInfo | URL, init?: RequestInit) =>
    (typeof input === 'string'
      ? input
      : input instanceof URL
        ? input.href
        : input.url) === '/api/eligibility/facts'
      ? Promise.resolve(Response.json({ facts: factsFixture }))
      : fetchImpl(input, init);
}
