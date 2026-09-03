import {
  Bath,
  BedDouble,
  Building2,
  ExternalLink,
  Layers,
  MapPin,
  Maximize2,
  Sparkles,
} from 'lucide-react';

import { Badge } from '@/components/ui/badge';
import type { Listing, ListingAttribute } from '@/lib/types';
import { cn } from '@/lib/utils';

interface PropertyCardProps {
  listing: Listing;
  className?: string;
}

function formatMoney(amount?: number | null, currency?: string | null): string {
  if (amount == null) return 'A consultar';
  const formattedNumber = new Intl.NumberFormat('es-AR', {
    maximumFractionDigits: 0,
  }).format(amount);

  if (currency === 'USD') {
    return `USD ${formattedNumber}`;
  }
  return `$ ${formattedNumber}`;
}

function formatAttributeLabel(attr: ListingAttribute): string {
  const { type, value } = attr;
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
      return `${type.replace(/_/g, ' ')}: ${value}`;
  }
}

export function PropertyCard({ listing, className }: PropertyCardProps) {
  const {
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

  const hasExpenses = expenses?.amount != null;
  const isRental = operation.toLowerCase().includes('alquiler');
  const sourceName = source ? source.charAt(0).toUpperCase() + source.slice(1) : 'ZonaProp';

  return (
    <article
      className={cn(
        'property-card group relative flex flex-col justify-between overflow-hidden rounded-2xl border border-border bg-card/90 p-5 shadow-sm transition-all duration-200 hover:-translate-y-0.5 hover:shadow-md md:p-6',
        className,
      )}
    >
      <div>
        {/* Top Header: Operation + Neighborhood / Agency */}
        <div className="flex flex-wrap items-center justify-between gap-2 border-b border-border/40 pb-3">
          <div className="flex items-center gap-2">
            <Badge variant="outline" className="text-xs font-semibold capitalize tracking-wide">
              {operation}
            </Badge>
            <span className="flex items-center gap-1 text-xs font-medium text-muted-foreground">
              <MapPin className="h-3 w-3 text-primary" />
              <span className="capitalize">{neighborhood}</span>
            </span>
          </div>

          {agency ? (
            <span className="truncate text-xs text-muted-foreground" title={agency}>
              {agency}
            </span>
          ) : null}
        </div>

        {/* Address & Price Row */}
        <div className="mt-4 flex flex-col justify-between gap-2 sm:flex-row sm:items-baseline">
          <div>
            <h3 className="font-heading text-lg font-semibold tracking-tight text-foreground sm:text-xl">
              {address || `Departamento en ${neighborhood}`}
            </h3>
            {floor ? (
              <p className="mt-0.5 text-xs text-muted-foreground">Piso {floor}</p>
            ) : null}
          </div>

          <div className="text-left sm:text-right">
            <div className="font-mono text-xl font-bold tracking-tight text-primary sm:text-2xl">
              {formatMoney(price.amount, price.currency)}
              {isRental ? <span className="text-xs font-normal text-muted-foreground"> / mes</span> : null}
            </div>

            <div className="mt-0.5 text-xs">
              {hasExpenses ? (
                <span className="font-mono text-muted-foreground">
                  + {formatMoney(expenses.amount, expenses.currency)} expensas
                </span>
              ) : (
                <span className="inline-flex items-center rounded-sm bg-muted/60 px-1.5 py-0.5 font-sans text-xs font-medium text-muted-foreground" title="El anuncio original no publicó el valor de expensas">
                  Expensas no publicadas
                </span>
              )}
            </div>
          </div>
        </div>

        {/* Key Metrics Grid */}
        <div className="mt-4 grid grid-cols-2 gap-2 rounded-xl bg-muted/40 p-2.5 sm:grid-cols-4 sm:gap-3">
          {rooms != null ? (
            <div className="flex items-center gap-1.5 text-xs text-foreground">
              <Layers className="h-3.5 w-3.5 text-muted-foreground" />
              <span className="font-medium">{rooms} amb</span>
            </div>
          ) : null}

          {bedrooms != null ? (
            <div className="flex items-center gap-1.5 text-xs text-foreground">
              <BedDouble className="h-3.5 w-3.5 text-muted-foreground" />
              <span className="font-medium">{bedrooms} dorm</span>
            </div>
          ) : null}

          {bathrooms != null ? (
            <div className="flex items-center gap-1.5 text-xs text-foreground">
              <Bath className="h-3.5 w-3.5 text-muted-foreground" />
              <span className="font-medium">
                {bathrooms} {bathrooms === 1 ? 'baño' : 'baños'}
              </span>
            </div>
          ) : null}

          {total_area_m2 != null ? (
            <div className="flex items-center gap-1.5 text-xs text-foreground">
              <Maximize2 className="h-3.5 w-3.5 text-muted-foreground" />
              <span className="font-medium">{total_area_m2} m²</span>
            </div>
          ) : null}
        </div>

        {/* Parsed Attributes with Provenance & Quotes */}
        {attributes.length > 0 ? (
          <div className="mt-4 border-t border-border/40 pt-3">
            <p className="mb-2 text-xs font-semibold text-muted-foreground">
              Cualidades identificadas
            </p>
            <div className="flex flex-wrap gap-2">
              {attributes.map((attr, idx) => {
                const isStated = attr.provenance === 'stated';
                const label = formatAttributeLabel(attr);

                return (
                  <div
                    key={`${attr.type}-${attr.value}-${idx}`}
                    className={cn(
                      'inline-flex flex-col gap-0.5 rounded-lg border px-2.5 py-1.5 text-xs transition-colors',
                      isStated
                        ? 'border-border/80 bg-background text-foreground'
                        : 'border-dashed border-primary/40 bg-secondary/20 text-foreground',
                    )}
                  >
                    <div className="flex items-center gap-1.5 font-medium">
                      {!isStated ? (
                        <span
                          title="Deducido por el modelo a partir de la descripción"
                          className="flex items-center text-primary"
                        >
                          <Sparkles className="h-3 w-3" />
                        </span>
                      ) : null}
                      <span>{label}</span>
                      <span className="text-[10px] text-muted-foreground/80">
                        ({isStated ? 'publicado' : 'inferido'})
                      </span>
                    </div>

                    {attr.evidence ? (
                      <p className="text-[11px] italic text-muted-foreground">
                        “{attr.evidence}”
                      </p>
                    ) : null}
                  </div>
                );
              })}
            </div>
          </div>
        ) : null}
      </div>

      {/* Footer: Link to Original Listing */}
      <div className="mt-5 flex items-center justify-between border-t border-border/40 pt-3">
        <span className="flex items-center gap-1 text-xs text-muted-foreground">
          <Building2 className="h-3 w-3" />
          Publicado en {sourceName}
        </span>

        <a
          href={url}
          target="_blank"
          rel="noopener noreferrer"
          className="inline-flex items-center gap-1.5 rounded-lg bg-primary px-3 py-1.5 text-xs font-semibold text-primary-foreground shadow-xs transition-colors hover:bg-primary/90 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring"
          aria-label={`Ver en ${sourceName}`}
        >
          <span>Ver en {sourceName}</span>
          <ExternalLink className="h-3 w-3" aria-hidden="true" />
        </a>
      </div>
    </article>
  );
}
