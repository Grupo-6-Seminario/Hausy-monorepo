'use client';

import { ExternalLink } from 'lucide-react';
import { useState } from 'react';

import { recordContactIntent, stableListingID } from '@/lib/contact-intent';
import type { Eligibility, EligibilityState, Listing, ListingAttribute } from '@/lib/types';
import { cn } from '@/lib/utils';

interface PropertyCardProps {
  listing: Listing;
  isRecommended?: boolean;
  className?: string;
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
  ineligible: 'No califica',
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
): { text: string; missing: boolean } | null {
  if (eligibility.state !== 'unknown') return null;
  const conditions = eligibility.conditions ?? [];
  if (conditions.length === 0) {
    return { text: 'No publica requisitos', missing: true };
  }
  if (conditions.some((condition) => condition.reason === 'missing')) {
    return { text: 'Publica requisitos · completá tus datos', missing: true };
  }
  return { text: 'Requisito no verificable', missing: false };
}

export function PropertyCard({
  listing,
  isRecommended = false,
  className,
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
  const toConfirm = eligibility ? unknownReason(eligibility) : null;

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
            {eligibilityLabel[eligibility.state]}
          </span>
        ) : null}
        {isRecommended ? (
          <span className="listing-fit">Destacada por Hausy</span>
        ) : null}
        {rank != null ? <span className="listing-rank">#{rank}</span> : null}
      </header>

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
