export type Provenance = 'stated' | 'inferred';

export interface ListingAttribute {
  type: string;
  value: string;
  provenance: Provenance;
  evidence?: string;
}

export interface Money {
  amount?: number | null;
  currency?: string | null;
}

export interface Listing {
  id?: number | string;
  rank?: number;
  source: string;
  url: string;
  neighborhood: string;
  agency?: string;
  address?: string;
  description: string;
  operation?: string;
  price: Money;
  expenses: Money;
  total_area_m2?: number | null;
  covered_area_m2?: number | null;
  rooms?: number | null;
  bedrooms?: number | null;
  bathrooms?: number | null;
  parking_spaces?: number | null;
  age_years?: number | null;
  floor?: string | null;
  scraped_at?: string;
  attributes?: ListingAttribute[];
  parsed_at?: string;
  parser_model?: string;
}

export interface Requirement {
  type: string;
  value: string;
}

export interface AgentResponse {
  reply?: string;
  listings?: Listing[];
  requirements?: Requirement[];
  error?: string;
}
