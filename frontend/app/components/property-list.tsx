import { Home } from 'lucide-react';
import type { ReactNode } from 'react';

import {
  Empty,
  EmptyDescription,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from '@/components/ui/empty';
import { Skeleton } from '@/components/ui/skeleton';
import { type FactsState, useEligibilityFacts } from '@/lib/eligibility-facts';
import type {
  EligibilityState,
  Listing,
  Qualification,
  Relaxation,
} from '@/lib/types';
import { cn } from '@/lib/utils';

import { eligibilityLabel, PropertyCard, type Situation } from './property-card';

// Eligibility order is the product's order (CONTEXT.md, Eligibility);
// ineligible listings never reach the list.
const sections = ['eligible', 'conditionally_eligible', 'unknown'] as const;

// A section groups conditional cards whose badges name different deciders
// (the agency, or the searcher's exact income), so its title names neither.
const sectionTitle: Record<EligibilityState, string> = {
  ...eligibilityLabel,
  conditionally_eligible: 'Calificás con condiciones',
};

// What each section means, one sentence a searcher can open beside its title.
const sectionMeaning: Record<(typeof sections)[number], string> = {
  eligible: 'Cumplís todo lo que el aviso publica como requisito.',
  conditionally_eligible:
    'Cumplís lo publicado, pero una parte depende de la inmobiliaria o de tu ingreso exacto.',
  unknown: 'El aviso no publica requisitos o pide algo que no nos dijiste.',
};

// instrumentLabel names a relaxation's value as the catalog does; before the
// catalog loads, or for a value it lacks, the raw value stands in.
function instrumentLabel(facts: FactsState, fact: string, value: string) {
  if (facts.kind !== 'ready') return value;
  const choice = facts.facts
    .find((f) => f.name === fact)
    ?.choices.find((c) => c.value === value);
  return choice ? choice.label.toLowerCase() : value;
}

function relaxationLines(facts: FactsState, relaxations: Relaxation[]) {
  return relaxations.map((relaxation) => (
    <p
      key={`${relaxation.fact}-${relaxation.value}`}
      className="relaxation-line"
    >
      Si conseguís {instrumentLabel(facts, relaxation.fact, relaxation.value)},{' '}
      {relaxation.count === 1
        ? 'vuelve 1 propiedad'
        : `vuelven ${relaxation.count} propiedades`}
      .
    </p>
  ));
}

interface PropertyListProps {
  listings?: Listing[];
  relaxations?: Relaxation[];
  // Listings the search matched but hid because the searcher does not qualify.
  hiddenIneligible?: number;
  // What the searcher declared, so each card can say why its verdict holds.
  declared?: Qualification;
  recommendedRanks?: number[];
  isLoading?: boolean;
  className?: string;
  // Shown under the heading in every state: what the search is using.
  children?: ReactNode;
}

export function PropertyList({
  listings = [],
  relaxations = [],
  hiddenIneligible = 0,
  declared,
  recommendedRanks = [],
  isLoading = false,
  className,
  children,
}: PropertyListProps) {
  const facts = useEligibilityFacts();
  const situation: Situation | undefined =
    facts.kind === 'ready' && declared
      ? { facts: facts.facts, declared }
      : undefined;
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
        {children}
        {/* Ghost cards keep the real card's outline, so the shortlist lands in place. */}
        <div className="property-skeletons" aria-hidden="true">
          {[1, 2, 3].map((index) => (
            <div key={index} className="property-skeleton">
              <div className="property-skeleton-meta">
                <Skeleton />
                <Skeleton />
                <Skeleton />
              </div>
              <div className="property-skeleton-summary">
                <Skeleton />
                <Skeleton />
              </div>
              <Skeleton className="property-skeleton-metrics" />
              <div className="property-skeleton-lines">
                <Skeleton />
                <Skeleton />
              </div>
              <div className="property-skeleton-footer">
                <Skeleton />
                <Skeleton />
                <Skeleton />
              </div>
            </div>
          ))}
        </div>
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
        {children}
        <Empty className="property-empty">
          <EmptyHeader>
            <EmptyMedia variant="icon">
              <Home aria-hidden="true" />
            </EmptyMedia>
            {hiddenIneligible > 0 ? (
              <>
                <EmptyTitle>Ninguna coincide con tu situación</EmptyTitle>
                <EmptyDescription>
                  Hay {hiddenIneligible}{' '}
                  {hiddenIneligible === 1
                    ? 'propiedad que no te mostramos'
                    : 'propiedades que no te mostramos'}{' '}
                  porque no cumplís sus requisitos.
                </EmptyDescription>
              </>
            ) : (
              <>
                <EmptyTitle>
                  No encontramos propiedades que coincidan exactamente
                </EmptyTitle>
                <EmptyDescription>
                  Contale a Hausy qué podés flexibilizar: el presupuesto, los
                  ambientes o algún requisito.
                </EmptyDescription>
              </>
            )}
          </EmptyHeader>
          {relaxations.length > 0 ? (
            relaxationLines(facts, relaxations)
          ) : hiddenIneligible > 0 ? (
            <EmptyDescription>
              Revisá tu situación o contale a Hausy qué requisito podés
              flexibilizar.
            </EmptyDescription>
          ) : null}
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
      <p className="provenance-legend">
        Origen del dato:
        <span data-provenance="stated">Publicado</span>
        <span data-provenance="inferred">Inferido</span>
        <span data-provenance="missing">Falta</span>
      </p>
      {children}
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
              {renderByEligibility(group, recommendedRanks, true, situation)}
            </div>;
          })}
        </>
      ) : renderByEligibility(listings, recommendedRanks, false, situation)}
      {relaxationLines(facts, relaxations)}
    </section>
  );
}

function renderByEligibility(listings: Listing[], recommendedRanks: number[], nested: boolean, situation?: Situation) {
  if (!listings.some((listing) => listing.eligibility)) return renderCards(listings, recommendedRanks, situation);
  return sections.map((state) => {
    const inSection = listings.filter((listing) => listing.eligibility?.state === state);
    if (inSection.length === 0) return null;
    return <div key={state} className="eligibility-section" data-state={state}>
      <div className="eligibility-section-title">
        {nested ? <h4>{sectionTitle[state]}</h4> : <h3>{sectionTitle[state]}</h3>}
        <details className="section-meaning">
          <summary>¿Qué significa?</summary>
          <p>{sectionMeaning[state]}</p>
        </details>
      </div>
      {renderCards(inSection, recommendedRanks, situation)}
    </div>;
  });
}

function renderCards(listings: Listing[], recommendedRanks: number[], situation?: Situation) {
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
              situation={situation}
            />
          </li>
        );
      })}
    </ol>
  );
}
