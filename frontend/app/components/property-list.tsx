import { Home } from 'lucide-react';

import {
  Empty,
  EmptyDescription,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from '@/components/ui/empty';
import { Skeleton } from '@/components/ui/skeleton';
import type { EligibilityState, Listing, Relaxation } from '@/lib/types';
import { cn } from '@/lib/utils';

import { eligibilityLabel, PropertyCard } from './property-card';

// Eligibility order is the product's order (CONTEXT.md, Eligibility);
// ineligible listings never reach the list.
const sections: EligibilityState[] = [
  'eligible',
  'conditionally_eligible',
  'unknown',
];

const instrumentLabel: Record<string, string> = {
  propietaria: 'garantía propietaria',
  caucion: 'seguro de caución',
};

interface PropertyListProps {
  listings?: Listing[];
  relaxations?: Relaxation[];
  recommendedRanks?: number[];
  isLoading?: boolean;
  className?: string;
}

export function PropertyList({
  listings = [],
  relaxations = [],
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

  const hasQualitativeGroups = listings.some((listing) => listing.qualitative_fit);
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
        <span>
          {listings.some((listing) => listing.eligibility)
            ? 'Ordenadas por si podés alquilarlas'
            : 'Ordenadas por afinidad'}
        </span>
      </div>
      <p className="selection-guide">
        {recommendedRanks.length > 0
          ? 'Las fichas citadas en la lectura de Hausy están señaladas. En todas distinguimos lo publicado de lo interpretado.'
          : 'Usá el número de cada ficha para relacionarla con la lectura de Hausy. Lo publicado y lo interpretado aparecen separados.'}
      </p>
      {hasQualitativeGroups ? (
        <>
          {(['exact', 'unconfirmed'] as const).map((fit) => {
            const group = listings.filter((listing) => fit === 'exact'
              ? listing.qualitative_fit !== 'unconfirmed'
              : listing.qualitative_fit === 'unconfirmed');
            if (group.length === 0) return null;
            return <div key={fit} className="qualitative-section">
              <h3>{fit === 'exact' ? 'Coincidencias con evidencia' : 'Otras opciones por confirmar'}</h3>
              {renderByEligibility(group, recommendedRanks, true)}
            </div>;
          })}
        </>
      ) : renderByEligibility(listings, recommendedRanks, false)}
      {relaxations.map((relaxation) => (
        <p
          key={`${relaxation.fact}-${relaxation.value}`}
          className="relaxation-line"
        >
          Si conseguís {instrumentLabel[relaxation.value] ?? relaxation.value},
          vuelven {relaxation.count}{' '}
          {relaxation.count === 1 ? 'propiedad' : 'propiedades'}.
        </p>
      ))}
    </section>
  );
}

function renderByEligibility(listings: Listing[], recommendedRanks: number[], nested: boolean) {
  if (!listings.some((listing) => listing.eligibility)) return renderCards(listings, recommendedRanks);
  return sections.map((state) => {
    const inSection = listings.filter((listing) => listing.eligibility?.state === state);
    if (inSection.length === 0) return null;
    return <div key={state} className="eligibility-section" data-state={state}>
      {nested ? <h4>{eligibilityLabel[state]}</h4> : <h3>{eligibilityLabel[state]}</h3>}
      {renderCards(inSection, recommendedRanks)}
    </div>;
  });
}

function renderCards(listings: Listing[], recommendedRanks: number[]) {
  return (
    <ol start={listings[0]?.rank ?? 1}>
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
  );
}
