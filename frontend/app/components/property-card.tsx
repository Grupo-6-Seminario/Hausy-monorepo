'use client';

import { ExternalLink } from 'lucide-react';
import { useState } from 'react';

import { recordContactIntent, stableListingID } from '@/lib/contact-intent';
import type { EligibilityFact } from '@/lib/eligibility-facts';
import type {
  Eligibility,
  EligibilityCondition,
  EligibilityRule,
  EligibilityState,
  Listing,
  ListingAttribute,
  Qualification,
} from '@/lib/types';
import { cn } from '@/lib/utils';

import { yesPhrases } from './qualification-panel';

// What the searcher declared and the catalog that names it: the card's
// eligibility reason needs both.
export interface Situation {
  facts: EligibilityFact[];
  declared: Qualification;
}

interface PropertyCardProps {
  listing: Listing;
  isRecommended?: boolean;
  className?: string;
  situation?: Situation;
}

function formatMoney(amount?: number | null, currency?: string | null): string {
  if (amount == null) return 'A consultar';
  const formattedNumber = new Intl.NumberFormat('es-AR', {
    maximumFractionDigits: 0,
  }).format(amount);

  return currency === 'USD' ? `USD ${formattedNumber}` : `$ ${formattedNumber}`;
}

function formatAttributeLabel({ type, value }: ListingAttribute): string {
  switch (type) {
    case 'natural_light':
      if (value === 'high') return 'Luz natural: alta';
      if (value === 'medium') return 'Luz natural: media';
      if (value === 'low') return 'Luz natural: baja';
      return `Luz natural: ${value}`;
    case 'noise_level':
      if (value === 'quiet') return 'Silencioso';
      if (value === 'moderate') return 'Ruido moderado';
      if (value === 'noisy') return 'Ruidoso';
      return `Ruido: ${value}`;
    case 'exposure':
      if (value === 'frente') return 'Al frente';
      if (value === 'contrafrente') return 'Contrafrente';
      if (value === 'interno') return 'Interno';
      if (value === 'lateral') return 'Lateral';
      return `Disposición: ${value}`;
    case 'outdoor_space':
      if (value === 'balcon') return 'Balcón';
      if (value === 'balcon_terraza') return 'Balcón terraza';
      if (value === 'terraza') return 'Terraza';
      if (value === 'patio') return 'Patio';
      if (value === 'jardin') return 'Jardín';
      return value;
    case 'transit_access':
      if (value.startsWith('subte_')) {
        return `Subte ${value.replace('subte_', '').toUpperCase()}`;
      }
      return value.charAt(0).toUpperCase() + value.slice(1);
    case 'amenity':
      return value.charAt(0).toUpperCase() + value.slice(1);
    default:
      return `${type.replaceAll('_', ' ')}: ${value}`;
  }
}

// The searcher-facing names of the eligibility states (CONTEXT.md, Eligibility).
export const eligibilityLabel: Record<EligibilityState, string> = {
  eligible: 'Calificás',
  conditionally_eligible: 'Depende de la inmobiliaria',
  unknown: 'A confirmar',
  ineligible: 'No calificás',
};

// Text beside the label, never instead of it: the badge always names the state.
export const eligibilityMark: Record<EligibilityState, string> = {
  eligible: '✓',
  conditionally_eligible: '!',
  unknown: '?',
  ineligible: '–',
};

// An unknown verdict says why: the ad publishes nothing, it asks for something
// the searcher has not declared, or what it asks cannot be checked. Only the
// first two are data that is missing.
function unknownReason(
  eligibility: Eligibility,
  facts: EligibilityFact[] = [],
): { text: string; missing: boolean } | null {
  if (eligibility.state !== 'unknown') return null;
  const conditions = eligibility.conditions ?? [];
  if (conditions.length === 0) {
    return { text: 'No publica requisitos', missing: true };
  }
  if (conditions.some((condition) => condition.reason === 'missing')) {
    return { text: 'Publica requisitos · completá tus datos', missing: true };
  }
  // A refusal that spares the searcher's guarantee names what it refuses
  // (specs/008); the catalog labels it, or the raw value stands in.
  const refusal = conditions.find((condition) => condition.reason === 'unconfirmed');
  if (refusal) {
    const choices = facts.find((f) => f.name === refusal.rule.fact)?.choices ?? [];
    const refused = (refusal.rule.values ?? []).map(
      (value) =>
        choices.find((c) => c.value === value)?.label.toLowerCase() ?? value,
    );
    return { text: `No acepta ${refused.join(' ni ')}`, missing: false };
  }
  return { text: 'Requisito no verificable', missing: false };
}

// A conditional verdict names who decides (CONTEXT.md, Conditionally
// eligible): the agency for a discretionary rule, the searcher's exact income
// for a band that straddles a hard threshold.
function badgeLabel(eligibility: Eligibility) {
  const conditions = eligibility.conditions ?? [];
  if (
    eligibility.state !== 'conditionally_eligible' ||
    !conditions.some((condition) => condition.reason === 'near_line')
  ) {
    return eligibilityLabel[eligibility.state];
  }
  return conditions.some((condition) => condition.reason === 'discretionary')
    ? 'Depende de tus ingresos y de la inmobiliaria'
    : 'Depende de tus ingresos';
}

// Pesos written as the catalog's bands are ("$3.000.000").
function pesos(amount: number): string {
  const digits = new Intl.NumberFormat('es-AR', {
    maximumFractionDigits: 0,
  }).format(amount);
  return `$${digits}`;
}

function lowerFirst(text: string): string {
  return text.charAt(0).toLowerCase() + text.slice(1);
}

// The article each fact takes in "si acepta tu garantía"; the noun is the
// catalog's label. A fact without one is named generically.
const factArticle: Record<string, string> = { guarantee: 'tu', pets: 'tus' };

// metPhrase names what the searcher declared that clears a rule, in the
// catalog's words.
function metPhrase(
  rule: EligibilityRule,
  { facts, declared }: Situation,
): string | null {
  const fact = facts.find((f) => f.name === rule.fact);
  const values = declared[rule.fact] ?? [];
  if (!fact || values.length === 0) return null;
  if (rule.fact === 'income_band') {
    const band = fact.choices.find((c) => c.value === values[0]);
    return band ? `tus ingresos de ${lowerFirst(band.label)}` : null;
  }
  if (rule.fact === 'pets' && values.includes('none')) return 'sin mascotas';
  // A yes/no fact reads as the phrase the situation bar uses, not "sí".
  if (yesPhrases[rule.fact] && values.includes('yes')) return yesPhrases[rule.fact];
  const labels = fact.choices
    .filter((c) => values.includes(c.value) && (rule.values ?? []).includes(c.value))
    .map((c) => c.label.toLowerCase());
  return labels.length > 0 ? labels.join(', ') : null;
}

// unmetLine names one requirement the searcher does not meet, for a listing
// shown only because nothing else was left (specs/009): what it asks for,
// or what it does not take, in the catalog's words.
function unmetLine(
  condition: EligibilityCondition,
  { facts, declared }: Situation,
): string {
  const { rule } = condition;
  const choices = facts.find((f) => f.name === rule.fact)?.choices ?? [];
  const label = (value: string) =>
    choices.find((c) => c.value === value)?.label.toLowerCase() ?? value;
  switch (rule.fact) {
    case 'guarantee':
      return rule.operator === 'none_of'
        ? `No acepta ${(rule.values ?? []).map(label).join(' ni ')}`
        : `Pide ${(rule.values ?? []).map(label).join(' o ')}`;
    case 'pets': {
      const refused = (declared.pets ?? []).filter(
        (pet) => !(rule.values ?? []).includes(pet),
      );
      return refused.length === 0 || (rule.values ?? []).join() === 'none'
        ? 'No acepta mascotas'
        : `No acepta ${refused.map(label).join(' ni ')}`;
    }
    case 'income_band':
      if (condition.threshold != null) {
        return `Pide ingresos de ${pesos(condition.threshold)}`;
      }
      break;
    default:
      if (yesPhrases[rule.fact]) return `Pide ${yesPhrases[rule.fact]}`;
  }
  return `Pide: «${rule.evidence ?? rule.fact}»`;
}

// The line under the badge: why this verdict, with this listing's numbers.
// The income threshold is the engine's; "A confirmar" keeps its own reason.
function reasonLines(eligibility: Eligibility, situation?: Situation): string[] {
  if (!situation) return [];
  if (eligibility.state === 'ineligible') {
    return [
      ...new Set(
        (eligibility.conditions ?? [])
          .filter((condition) => condition.reason === 'not_met')
          .map((condition) => unmetLine(condition, situation)),
      ),
    ];
  }
  if (eligibility.state === 'eligible') {
    const met = (eligibility.met ?? [])
      .map((rule) => metPhrase(rule, situation))
      .filter((phrase) => phrase !== null);
    return met.length > 0 ? [`Cumplís lo que pide: ${met.join(', ')}.`] : [];
  }
  if (eligibility.state !== 'conditionally_eligible') return [];
  const conditions = eligibility.conditions ?? [];
  const lines: string[] = [];
  const nearLine = conditions.find((c) => c.reason === 'near_line');
  const band = situation.declared.income_band?.[0];
  if (nearLine?.threshold != null && band) {
    const [min, max] = band.split('-');
    const reach = max
      ? `Tu banda llega a ${pesos(Number(max))}`
      : `Tu banda empieza en ${pesos(Number(min))}`;
    lines.push(
      `Pide ingresos de ${pesos(nearLine.threshold)}. ${reach}: depende de tu monto exacto.`,
    );
  }
  for (const condition of conditions) {
    if (condition.reason !== 'discretionary') continue;
    const fact = situation.facts.find((f) => f.name === condition.rule.fact);
    const article = factArticle[condition.rule.fact];
    lines.push(
      fact && article
        ? `La inmobiliaria decide caso por caso si acepta ${article} ${fact.label.toLowerCase()}.`
        : 'La inmobiliaria decide caso por caso este requisito.',
    );
  }
  // One line per fact, even if two rules on it are the agency's call.
  return [...new Set(lines)];
}

export function PropertyCard({
  listing,
  isRecommended = false,
  className,
  situation,
}: PropertyCardProps) {
  const {
    rank,
    source,
    url,
    neighborhood,
    agency,
    address,
    operation = 'alquiler',
    price,
    expenses,
    total_area_m2,
    rooms,
    bedrooms,
    bathrooms,
    floor,
    matched = [],
    eligibility,
  } = listing;
  const conditions = (eligibility?.conditions ?? []).filter(
    (condition) => condition.rule.evidence,
  );
  // Ads name their amenities in one sentence, so a shared quote shows once.
  const qualities: { labels: string[]; attribute: ListingAttribute }[] = [];
  for (const attribute of matched) {
    const same = qualities.find(
      ({ attribute: first }) =>
        attribute.evidence &&
        first.evidence === attribute.evidence &&
        first.provenance === attribute.provenance,
    );
    if (same) same.labels.push(formatAttributeLabel(attribute));
    else qualities.push({ labels: [formatAttributeLabel(attribute)], attribute });
  }

  // Recording interest must never stand between the searcher and the agency, so
  // "Contactar" stays a plain link to the publication: the browser navigates
  // whatever the intent request does. A listing with no stable id gets the same
  // link and no event at all — an untraceable count is worse than no count.
  const listingID = stableListingID(listing);
  const [trackingFailed, setTrackingFailed] = useState(false);

  const recordInterest = async () => {
    if (!listingID) {
      setTrackingFailed(true);
      return;
    }
    try {
      await recordContactIntent(listingID, 'search_result_card');
      setTrackingFailed(false);
    } catch {
      setTrackingFailed(true);
    }
  };

  const hasExpenses = expenses?.amount != null;
  const isRental = operation.toLowerCase().includes('alquiler');
  const sourceName = source
    ? source.charAt(0).toUpperCase() + source.slice(1)
    : 'ZonaProp';
  const metrics = [
    rooms != null ? `${rooms} amb` : null,
    bedrooms != null ? `${bedrooms} dorm` : null,
    bathrooms != null ? `${bathrooms} ${bathrooms === 1 ? 'baño' : 'baños'}` : null,
    total_area_m2 != null ? `${total_area_m2} m²` : null,
  ].filter((metric) => metric !== null);
  const toConfirm = eligibility
    ? unknownReason(eligibility, situation?.facts)
    : null;

  return (
    <article
      className={cn(
        'property-card',
        isRecommended && 'property-card-featured',
        className,
      )}
      data-glow
    >
      <header className="property-card-header">
        {eligibility ? (
          <span className="eligibility-badge" data-state={eligibility.state}>
            <span className="eligibility-mark" aria-hidden="true">
              {eligibilityMark[eligibility.state]}
            </span>
            {badgeLabel(eligibility)}
          </span>
        ) : null}
        {isRecommended ? (
          <span className="listing-fit">Destacada por Hausy</span>
        ) : null}
        {rank != null ? <span className="listing-rank">#{rank}</span> : null}
      </header>
      {eligibility
        ? reasonLines(eligibility, situation).map((line) => (
            <p key={line} className="eligibility-reason">
              {line}
            </p>
          ))
        : null}

      <div className="property-summary">
        <div>
          <h4>{address || `Departamento en ${neighborhood}`}</h4>
          <p className="listing-place">
            {neighborhood}
            {floor ? <span> · Piso {floor}</span> : null}
            <span className="listing-operation"> · {operation}</span>
          </p>
        </div>
        <div className="listing-price">
          <p>
            {formatMoney(price.amount, price.currency)}
            {isRental ? <span> / mes</span> : null}
          </p>
          {hasExpenses ? (
            <span>
              + {formatMoney(expenses.amount, expenses.currency)} expensas
            </span>
          ) : (
            <span title="El anuncio original no publicó el valor de expensas">
              Expensas no publicadas
            </span>
          )}
        </div>
      </div>

      {metrics.length > 0 ? (
        <ul
          className="property-metrics"
          aria-label="Características principales"
        >
          {metrics.map((label) => (
            <li key={label}>{label}</li>
          ))}
        </ul>
      ) : null}

      {conditions.length > 0 ? (
        <div className="card-block">
          <h5>Lo que pide el aviso</h5>
          <ul
            className="eligibility-conditions"
            aria-label="Condiciones del aviso"
          >
            {conditions.map((condition, index) => (
              <li key={index}>
                <q>{condition.rule.evidence}</q>
              </li>
            ))}
          </ul>
        </div>
      ) : null}

      {qualities.length > 0 || toConfirm ? (
        <div className="card-checks">
          {qualities.length > 0 ? (
            <section
              className="property-evidence card-block"
              aria-label="Coincide con lo que pediste"
            >
              <h5>Coincide con lo que pediste</h5>
              <ul>
                {qualities.map(({ labels, attribute }) => {
                  const isStated = attribute.provenance === 'stated';
                  return (
                    <li
                      key={labels.join()}
                      className={isStated ? 'is-published' : 'is-inferred'}
                    >
                      <span>{labels.join(' · ')}</span>
                      <small
                        className="provenance-tag"
                        data-provenance={isStated ? 'stated' : 'inferred'}
                      >
                        {isStated ? 'Publicado' : 'Inferido'}
                      </small>
                      {attribute.evidence ? <q>{attribute.evidence}</q> : null}
                    </li>
                  );
                })}
              </ul>
            </section>
          ) : null}
          {toConfirm ? (
            <section className="card-block card-confirm" aria-label="Qué confirmar">
              <h5>Qué confirmar</h5>
              <ul>
                <li>
                  <span>{toConfirm.text}</span>
                  {toConfirm.missing ? (
                    <small className="provenance-tag" data-provenance="missing">
                      Falta
                    </small>
                  ) : null}
                </li>
              </ul>
            </section>
          ) : null}
        </div>
      ) : null}

      <footer className="property-card-footer">
        <span>
          Publicado en {sourceName}
          {agency ? <span title={agency}> · {agency}</span> : null}
        </span>
        <div className="property-card-actions">
          <a
            className="listing-contact"
            href={url}
            target="_blank"
            rel="noopener noreferrer"
            data-contact-tracking={listingID ? 'ready' : 'unavailable'}
            aria-label={`Contactar por esta publicación en ${sourceName}`}
            onClick={() => {
              void recordInterest();
            }}
          >
            Contactar
          </a>
          <a
            className="listing-source"
            href={url}
            target="_blank"
            rel="noopener noreferrer"
            aria-label={`Ver en ${sourceName}`}
          >
            Ver publicación
            <ExternalLink aria-hidden="true" />
          </a>
        </div>
        <p
          className="listing-contact-note"
          aria-live="polite"
          aria-atomic="true"
        >
          {trackingFailed
            ? 'No pudimos registrar tu interés. Podés seguir con el contacto.'
            : ''}
        </p>
      </footer>
    </article>
  );
}
