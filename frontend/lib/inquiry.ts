// Buyer-side inquiry (CONTEXT.md, Inquiry).
//
// The preview and the send share one input, and the server builds both, so
// the searcher consents to exactly the payload the agency receives. The client
// mints inquiry_id once per deliberate send; a retry reuses it and the backend
// stores the inquiry once.

import type { Inquiry } from './agency';
import type { Qualification, Requirement } from './types';

export interface InquiryContact {
  name: string;
  email: string;
  phone: string;
  message: string;
}

export function inquiryPath(listingId: string, preview = false): string {
  return `/api/listings/${encodeURIComponent(listingId)}/inquiries${preview ? '/preview' : ''}`;
}

export class InquiryRequestError extends Error {
  constructor(
    message: string,
    readonly code?: string,
  ) {
    super(message);
  }
}

async function post(path: string, body: object): Promise<Inquiry> {
  const response = await fetch(path, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  });
  const payload = (await response.json().catch(() => ({}))) as Inquiry & {
    error?: string;
    code?: string;
  };
  if (!response.ok) {
    throw new InquiryRequestError(
      payload.error || 'No pudimos enviar la consulta. Probá de nuevo.',
      payload.code,
    );
  }
  return payload;
}

export function previewInquiry(
  listingId: string,
  qualification: Qualification,
  searching: string[],
): Promise<Inquiry> {
  return post(inquiryPath(listingId, true), { qualification, searching });
}

export function sendInquiry(
  listingId: string,
  inquiryId: string,
  contact: InquiryContact,
  qualification: Qualification,
  searching: string[],
  previewRevision: string,
): Promise<Inquiry> {
  return post(inquiryPath(listingId), {
    inquiry_id: inquiryId,
    preview_revision: previewRevision,
    ...contact,
    qualification,
    searching,
  });
}

// What the searcher is looking for, as the "Tu búsqueda" chips name it, minus
// their budget: an inquiry never tells the agency how much they would pay.
const budget = new Set(['min_price', 'max_price', 'max_expenses']);

export function searchingPhrases(
  requirements: Requirement[],
  label: (requirement: Requirement) => string,
): string[] {
  return requirements.filter(({ type }) => !budget.has(type)).map(label);
}
