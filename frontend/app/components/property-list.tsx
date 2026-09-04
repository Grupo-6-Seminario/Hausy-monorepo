import { Home } from 'lucide-react';

import {
  Empty,
  EmptyDescription,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from '@/components/ui/empty';
import { Skeleton } from '@/components/ui/skeleton';
import type { Listing } from '@/lib/types';
import { cn } from '@/lib/utils';

import { PropertyCard } from './property-card';

interface PropertyListProps {
  listings?: Listing[];
  recommendedRanks?: number[];
  isLoading?: boolean;
  className?: string;
}

export function PropertyList({
  listings = [],
  recommendedRanks = [],
  isLoading = false,
  className,
}: PropertyListProps) {
  if (isLoading) {
    return (
      <section
        aria-label="Buscando propiedades"
        aria-busy="true"
        className={cn('property-list property-list-loading', className)}
      >
        <div className="property-list-heading">
          <div>
            <p>Selección actual</p>
            <h2>Buscando propiedades</h2>
          </div>
          <span>Evaluando afinidad</span>
        </div>
        {[1, 2, 3].map((index) => (
          <div key={index} className="property-skeleton" aria-hidden="true">
            <Skeleton />
            <Skeleton />
            <Skeleton />
          </div>
        ))}
      </section>
    );
  }

  if (listings.length === 0) {
    return (
      <section
        aria-label="Propiedades encontradas"
        className={cn('property-list', className)}
      >
        <div className="property-list-heading">
          <div>
            <p>Selección actual</p>
            <h2>Sin coincidencias exactas</h2>
          </div>
        </div>
        <Empty className="property-empty">
          <EmptyHeader>
            <EmptyMedia variant="icon">
              <Home aria-hidden="true" />
            </EmptyMedia>
            <EmptyTitle>
              No encontramos propiedades que coincidan exactamente
            </EmptyTitle>
            <EmptyDescription>
              Contale a Hausy qué requisito podrías flexibilizar o pedile
              explorar barrios cercanos.
            </EmptyDescription>
          </EmptyHeader>
        </Empty>
      </section>
    );
  }

  const count = listings.length;
  const countLabel =
    count === 1
      ? '1 propiedad seleccionada'
      : `${count} propiedades seleccionadas`;

  return (
    <section
      aria-label="Propiedades encontradas"
      className={cn('property-list', className)}
    >
      <div className="property-list-heading">
        <div>
          <p>Selección actual</p>
          <h2>{countLabel}</h2>
        </div>
        <span>Ordenadas por afinidad</span>
      </div>
      <p className="selection-guide">
        {recommendedRanks.length > 0
          ? 'Las fichas citadas en la lectura de Hausy están señaladas. En todas distinguimos lo publicado de lo interpretado.'
          : 'Usá el número de cada ficha para relacionarla con la lectura de Hausy. Lo publicado y lo interpretado aparecen separados.'}
      </p>
      <ol>
        {listings.map((listing, index) => {
          const rank = listing.rank ?? index + 1;
          const rankedListing = { ...listing, rank };

          return (
            <li key={listing.url || listing.id || index}>
              <PropertyCard
                isRecommended={recommendedRanks.includes(rank)}
                listing={rankedListing}
              />
            </li>
          );
        })}
      </ol>
    </section>
  );
}
