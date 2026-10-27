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

export type EligibilityState =
  | 'eligible'
  | 'conditionally_eligible'
  | 'unknown'
  | 'ineligible';

export interface EligibilityRule {
  fact: string;
  operator?: string;
  values?: string[];
  hardness?: string;
  // "declared" when the agency stated it in Hausy rather than the ad.
  source?: string;
  evidence?: string;
}

export interface EligibilityCondition {
  reason: string;
  rule: EligibilityRule;
  // The monthly income (ARS) a near-the-line band was compared with.
  threshold?: number;
}

export interface Eligibility {
  state: EligibilityState;
  conditions?: EligibilityCondition[];
  // The requirements the searcher clears.
  met?: EligibilityRule[];
}

// Zero-results line: declaring `value` for `fact` brings `count` listings back.
export interface Relaxation {
  fact: string;
  value: string;
  count: number;
}

// What the searcher declared: fact name -> values (see CONTEXT.md, Qualification).
export type Qualification = Record<string, string[]>;

export interface Listing {
  id?: number | string;
  rank?: number;
  source: string;
  url: string;
  neighborhood: string;
  agency?: string;
  // An agency manages it in Hausy, so the searcher can send it an inquiry.
  managed?: boolean;
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
  // The attributes that answer what the searcher asked for; the card shows only these.
  matched?: ListingAttribute[];
  parsed_at?: string;
  parser_model?: string;
  eligibility?: Eligibility;
  qualitative_fit?: 'exact' | 'unconfirmed';
}

export interface Requirement {
  type: string;
  value: string;
}

export interface ClarificationQuestion {
  id: string;
  request?: string;
  source: string;
  prompt: string;
  kind: 'search' | 'qualification' | 'unsupported';
  can_remove?: boolean;
  multi?: boolean;
  choices?: { id: string; label: string }[];
}

export interface AgentResponse {
  // The qualification used to assess these results, including chat answers.
  qualification?: Qualification;
  reply?: string;
  listings?: Listing[];
  requirements?: Requirement[];
  relaxations?: Relaxation[];
  hidden_ineligible?: number;
  // Sent only when nothing else is shown (specs/009).
  ineligible?: Listing[];
  error?: string;
  clarification?: ClarificationQuestion;
  clarification_hint?: string;
}
