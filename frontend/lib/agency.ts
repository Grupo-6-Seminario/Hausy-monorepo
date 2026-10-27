import type { EligibilityState } from './types';

export interface AgencyMoney {
  amount?: number | null;
  currency?: string;
}

export interface AgencyPropertyInput {
  url: string;
  neighborhood: string;
  address: string;
  description: string;
  operation: 'alquiler' | 'alquiler_temporal' | 'venta';
  price: AgencyMoney;
  expenses: AgencyMoney;
  total_area_m2?: number | null;
  covered_area_m2?: number | null;
  rooms?: number | null;
  bedrooms?: number | null;
  bathrooms?: number | null;
  parking_spaces?: number | null;
  age_years?: number | null;
  floor?: string;
  // What the agency declares about who can rent it; it replaces what was read
  // from the ad. The server words each one (evidence).
  requirements: AgencyRequirement[];
}

export interface AgencyRequirement {
  fact: string;
  operator: string;
  values: string[];
  hardness: 'hard' | 'discretionary';
  evidence?: string;
}

export interface AgencyProperty extends AgencyPropertyInput {
  id: string;
  agency: string;
  source: string;
  contact_count: number;
  inquiry_count: number;
}

// One requirement of the property and what the searcher declared about it,
// by the catalog's labels; declared is empty when they declared nothing.
export interface InquiryRequirement {
  fact: string;
  label: string;
  requirement: string;
  hardness: 'hard' | 'discretionary';
  declared: string[];
  outcome: string;
}

// Exactly what the agency receives (CONTEXT.md, Inquiry).
export interface Inquiry {
  preview_revision?: string;
  id?: string;
  listing_id: string;
  property: string;
  agency: string;
  name: string;
  email?: string;
  phone?: string;
  message?: string;
  searching?: string[];
  eligibility: EligibilityState;
  requirements: InquiryRequirement[];
  created_at?: string;
}
