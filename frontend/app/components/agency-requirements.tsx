'use client';

import { Input } from '@/components/ui/input';
import type { AgencyRequirement } from '@/lib/agency';
import type {
  EligibilityChoice,
  EligibilityFact,
} from '@/lib/eligibility-facts';

type Hardness = AgencyRequirement['hardness'];

// The operator each declarable fact is stated with (internal/agency/
// requirements.go); income switches to its "with expenses" variant.
const operators = {
  guarantee: 'one_of',
  income_band: 'income_multiple',
  income_documented: 'one_of',
  pets: 'subset_of',
} satisfies Record<string, string>;

type Fact = keyof typeof operators;

// The server's values for the multiple-choice facts, offered by name until
// the fact catalog labels them.
const fallbackValues: Record<'guarantee' | 'pets', string[]> = {
  guarantee: ['propietaria', 'caucion', 'recibos_garante'],
  pets: ['dog', 'cat', 'other'],
};

// A requirement without values is not declared: the editor keeps its other
// choices while the agency fills it in, and saving leaves it out. Sent empty,
// the server would read a pets requirement as "no pets".
export function declaredRequirements(requirements: AgencyRequirement[]) {
  return requirements.filter((requirement) => requirement.values.length > 0);
}

// The values a checkbox leaves checked, in the order its choices are listed.
function toggled(
  choices: EligibilityChoice[],
  values: string[],
  value: string,
  checked: boolean,
) {
  return choices
    .map((choice) => choice.value)
    .filter((v) => (v === value ? checked : values.includes(v)));
}

function HardnessField({
  fact,
  value = 'hard',
  onChange,
}: {
  fact: Fact;
  value?: Hardness;
  onChange: (hardness: Hardness) => void;
}) {
  return (
    <div className="agency-field">
      <label htmlFor={`requirement-${fact}-hardness`}>Exigencia</label>
      <select
        id={`requirement-${fact}-hardness`}
        value={value}
        onChange={(event) => onChange(event.target.value as Hardness)}
      >
        <option value="hard">Obligatorio</option>
        <option value="discretionary">A criterio de la inmobiliaria</option>
      </select>
    </div>
  );
}

interface AgencyRequirementsProps {
  facts: EligibilityFact[];
  requirements: AgencyRequirement[];
  onChange: (requirements: AgencyRequirement[]) => void;
}

// What the agency asks of whoever rents its property (CONTEXT.md,
// Eligibility requirement), at most one requirement per fact.
export function AgencyRequirements({
  facts,
  requirements,
  onChange,
}: AgencyRequirementsProps) {
  // A fact's choices other than "none", by the catalog's labels.
  const choices = (fact: keyof typeof fallbackValues): EligibilityChoice[] =>
    (
      facts.find((f) => f.name === fact)?.choices ??
      fallbackValues[fact].map((value) => ({ value, label: value }))
    ).filter((choice) => choice.value !== 'none');
  const find = (fact: Fact) => requirements.find((r) => r.fact === fact);

  function edit(fact: Fact, change: Partial<AgencyRequirement>) {
    const current = find(fact);
    const next: AgencyRequirement = {
      fact,
      operator: operators[fact],
      values: [],
      hardness: 'hard',
      ...current,
      ...change,
    };
    onChange(
      current
        ? requirements.map((r) => (r === current ? next : r))
        : [...requirements, next],
    );
  }

  const guarantees = choices('guarantee');
  const guarantee = find('guarantee');
  const income = find('income_band');
  const documented = find('income_documented');
  const animals = choices('pets');
  const pets = find('pets');
  // Every pets requirement admits "none"; what else it admits says which
  // option it is.
  const petsOption = !pets?.values.length
    ? 'unspecified'
    : pets.values.some((value) => value !== 'none')
      ? 'some'
      : 'none';

  return (
    <section
      className="agency-field-wide agency-requirements"
      aria-labelledby="requirements-title"
    >
      <div>
        <h3 id="requirements-title">Requisitos para alquilar</h3>
        <p>
          Quien busca ve lo que declares acá, en lugar de lo que diga el aviso.
        </p>
      </div>

      <fieldset>
        <legend>Garantías aceptadas</legend>
        <div className="agency-requirement-options">
          {guarantees.map((choice) => (
            <label key={choice.value} className="agency-requirement-choice">
              <input
                type="checkbox"
                checked={guarantee?.values.includes(choice.value) ?? false}
                onChange={(event) =>
                  edit('guarantee', {
                    values: toggled(
                      guarantees,
                      guarantee?.values ?? [],
                      choice.value,
                      event.target.checked,
                    ),
                  })
                }
              />
              {choice.label}
            </label>
          ))}
        </div>
        <HardnessField
          fact="guarantee"
          value={guarantee?.hardness}
          onChange={(hardness) => edit('guarantee', { hardness })}
        />
      </fieldset>

      <fieldset>
        <legend>Ingresos</legend>
        <div className="agency-requirement-options">
          <div className="agency-field agency-requirement-multiple">
            <label htmlFor="requirement-income">Veces el alquiler</label>
            <Input
              id="requirement-income"
              type="number"
              min="1"
              max="10"
              step="0.5"
              inputMode="decimal"
              aria-describedby="requirement-income-hint"
              value={income?.values[0] ?? ''}
              onChange={(event) =>
                edit('income_band', {
                  values: event.target.value === '' ? [] : [event.target.value],
                })
              }
            />
          </div>
          <label className="agency-requirement-choice">
            <input
              type="checkbox"
              checked={income?.operator === 'income_multiple_with_expenses'}
              onChange={(event) =>
                edit('income_band', {
                  operator: event.target.checked
                    ? 'income_multiple_with_expenses'
                    : 'income_multiple',
                })
              }
            />
            Sumando expensas
          </label>
        </div>
        <HardnessField
          fact="income_band"
          value={income?.hardness}
          onChange={(hardness) => edit('income_band', { hardness })}
        />
        <p id="requirement-income-hint">
          Solo se aplica a alquileres en pesos.
        </p>
      </fieldset>

      <fieldset>
        <legend>Ingresos comprobables</legend>
        <div className="agency-requirement-options">
          <label className="agency-requirement-choice">
            <input
              type="checkbox"
              checked={documented?.values.includes('yes') ?? false}
              onChange={(event) =>
                edit('income_documented', {
                  values: event.target.checked ? ['yes'] : [],
                })
              }
            />
            Los pido
          </label>
        </div>
        <HardnessField
          fact="income_documented"
          value={documented?.hardness}
          onChange={(hardness) => edit('income_documented', { hardness })}
        />
      </fieldset>

      <fieldset>
        <legend>Mascotas</legend>
        <div className="agency-requirement-options">
          <label className="agency-requirement-choice">
            <input
              type="radio"
              name="requirement-pets"
              checked={petsOption === 'unspecified'}
              onChange={() => edit('pets', { values: [] })}
            />
            No lo especifico
          </label>
          <label className="agency-requirement-choice">
            <input
              type="radio"
              name="requirement-pets"
              checked={petsOption === 'none'}
              onChange={() => edit('pets', { values: ['none'] })}
            />
            No admite mascotas
          </label>
          {/* Admitting pets starts from every animal; the agency unchecks the
              ones it refuses, and unchecking the last one admits none. */}
          <label className="agency-requirement-choice">
            <input
              type="radio"
              name="requirement-pets"
              checked={petsOption === 'some'}
              onChange={() =>
                edit('pets', {
                  values: ['none', ...animals.map((choice) => choice.value)],
                })
              }
            />
            Admite:
          </label>
          {animals.map((choice) => (
            <label key={choice.value} className="agency-requirement-choice">
              <input
                type="checkbox"
                checked={pets?.values.includes(choice.value) ?? false}
                onChange={(event) =>
                  edit('pets', {
                    values: [
                      'none',
                      ...toggled(
                        animals,
                        pets?.values ?? [],
                        choice.value,
                        event.target.checked,
                      ),
                    ],
                  })
                }
              />
              {choice.label}
            </label>
          ))}
        </div>
        <HardnessField
          fact="pets"
          value={pets?.hardness}
          onChange={(hardness) => edit('pets', { hardness })}
        />
      </fieldset>
    </section>
  );
}
