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
}

export interface AgencyProperty extends AgencyPropertyInput {
  id: string;
  agency: string;
  source: string;
  contact_count: number;
}
