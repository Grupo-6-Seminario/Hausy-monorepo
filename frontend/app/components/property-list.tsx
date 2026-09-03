import { Home, Sparkles } from 'lucide-react';

import type { Listing } from '@/lib/types';
import { cn } from '@/lib/utils';
import { PropertyCard } from './property-card';

interface PropertyListProps {
  listings?: Listing[];
  isLoading?: boolean;
  className?: string;
}

export function PropertyList({ listings = [], isLoading = false, className }: PropertyListProps) {
  if (isLoading) {
    return (
      <section
        aria-label="Buscando propiedades"
        aria-busy="true"
        className={cn('property-list-loading space-y-4', className)}
      >
        <div className="flex items-center gap-2 text-sm font-medium text-muted-foreground">
          <Sparkles className="h-4 w-4 animate-spin text-primary" />
          <span>Consultando el inventario y evaluando coincidencias...</span>
        </div>
        <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
          {[1, 2].map((idx) => (
            <div
              key={idx}
              className="h-64 animate-pulse rounded-2xl border border-border/60 bg-muted/40 p-6"
            />
          ))}
        </div>
      </section>
    );
  }

  if (listings.length === 0) {
    return (
      <section
        aria-label="Propiedades encontradas"
        className={cn(
          'flex flex-col items-center justify-center rounded-2xl border border-dashed border-border bg-card/50 p-8 text-center sm:p-12',
          className,
        )}
      >
        <div className="flex h-12 w-12 items-center justify-center rounded-xl bg-muted text-muted-foreground">
          <Home className="h-6 w-6" aria-hidden="true" />
        </div>
        <h3 className="mt-4 font-heading text-lg font-semibold text-foreground">
          No encontramos propiedades que coincidan exactamente
        </h3>
        <p className="mt-1.5 max-w-md text-sm text-muted-foreground">
          Probá ampliando el rango de presupuesto, explorando barrios cercanos o indicando si podés
          flexibilizar algún requisito.
        </p>
      </section>
    );
  }

  const count = listings.length;
  const countLabel = count === 1 ? '1 propiedad seleccionada' : `${count} propiedades seleccionadas`;

  return (
    <section
      aria-label="Propiedades encontradas"
      className={cn('property-list space-y-5', className)}
    >
      <div className="flex flex-wrap items-baseline justify-between gap-2 border-b border-border/60 pb-3">
        <h2 className="font-heading text-xl font-bold tracking-tight text-foreground sm:text-2xl">
          {countLabel}
        </h2>
        <span className="text-xs font-medium text-muted-foreground">
          Ordenadas por afinidad con tus prioridades
        </span>
      </div>

      <ul className="grid grid-cols-1 gap-6 md:grid-cols-2">
        {listings.map((listing, index) => (
          <li key={listing.url || listing.id || index} className="list-none">
            <PropertyCard
              listing={listing.rank != null ? listing : { ...listing, rank: index + 1 }}
            />
          </li>
        ))}
      </ul>
    </section>
  );
}
