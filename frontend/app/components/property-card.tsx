'use client';

import {
  Bath,
  BedDouble,
  Building2,
  ExternalLink,
  Handshake,
  Layers,
  MapPin,
  Maximize2,
} from 'lucide-react';
import { useState } from 'react';

import { recordContactIntent, stableListingID } from '@/lib/contact-intent';
import type { Listing, ListingAttribute } from '@/lib/types';
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
    attributes = [],
  } = listing;

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
    rooms != null ? { icon: Layers, label: `${rooms} amb` } : null,
    bedrooms != null ? { icon: BedDouble, label: `${bedrooms} dorm` } : null,
    bathrooms != null
      ? {
          icon: Bath,
          label: `${bathrooms} ${bathrooms === 1 ? 'baño' : 'baños'}`,
        }
      : null,
    total_area_m2 != null
      ? { icon: Maximize2, label: `${total_area_m2} m²` }
      : null,
  ].filter(
    (metric): metric is { icon: typeof Layers; label: string } =>
      metric !== null,
  );

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
        <div className="listing-position">
          {rank != null ? <span className="listing-rank">#{rank}</span> : null}
          {isRecommended ? (
            <span className="listing-fit">Destacada por Hausy</span>
          ) : null}
          <span className="listing-operation">{operation}</span>
          <span className="listing-neighborhood">
            <MapPin aria-hidden="true" />
            {neighborhood}
          </span>
        </div>
        {agency ? <p title={agency}>{agency}</p> : null}
      </header>

      <div className="property-summary">
        <div>
          <h4>{address || `Departamento en ${neighborhood}`}</h4>
          {floor ? <p className="listing-floor">Piso {floor}</p> : null}
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
          {metrics.map(({ icon: Icon, label }) => (
            <li key={label}>
              <Icon aria-hidden="true" />
              {label}
            </li>
          ))}
        </ul>
      ) : null}

      {attributes.length > 0 ? (
        <section
          className="property-evidence"
          aria-label="Cualidades identificadas"
        >
          <h5>Cualidades identificadas</h5>
          <ul>
            {attributes.map((attribute, index) => {
              const isStated = attribute.provenance === 'stated';
              return (
                <li
                  key={`${attribute.type}-${attribute.value}-${index}`}
                  className={isStated ? 'is-published' : 'is-inferred'}
                >
                  <div>
                    <span>{formatAttributeLabel(attribute)}</span>
                    <small
                      className={isStated ? undefined : 'evidence-provenance'}
                    >
                      {isStated ? 'Publicado' : 'Inferido por Hausy'}
                    </small>
                  </div>
                  {attribute.evidence ? <q>{attribute.evidence}</q> : null}
                </li>
              );
            })}
          </ul>
        </section>
      ) : null}

      <footer className="property-card-footer">
        <span>
          <Building2 aria-hidden="true" />
          Publicado en {sourceName}
        </span>
        <div className="property-card-actions">
          <a
            className="listing-contact"
            href={url}
            target="_blank"
            rel="noopener noreferrer"
            data-glow
            data-contact-tracking={listingID ? 'ready' : 'unavailable'}
            aria-label={`Contactar por esta publicación en ${sourceName}`}
            onClick={() => {
              void recordInterest();
            }}
          >
            <Handshake aria-hidden="true" />
            Contactar
          </a>
          <a
            href={url}
            target="_blank"
            rel="noopener noreferrer"
            data-glow
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
