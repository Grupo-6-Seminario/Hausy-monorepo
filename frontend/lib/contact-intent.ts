// Buyer-side contact intent.
//
// A deliberate "Contactar" on a result card records one event. The client mints
// the `intent_id`, so a replay of the same activation is the same intent and the
// backend can refuse to count it twice. The client sends nothing else: no agency
// id, no realtor user id, no counter value. Ownership and the aggregate are
// derived on the backend.
//
// The listing id is the stable id carried on the listing record. It is never
// parsed out of the publication URL, which belongs to the portal and can change.

import type { Listing } from './types';

export type ContactIntentSource = 'search_result_card';

export interface ContactIntentRecord {
  intent_id: string;
  listing_id: string;
  recorded: boolean;
}

export function contactIntentPath(listingId: string): string {
  return `/api/listings/${encodeURIComponent(listingId)}/contact-intents`;
}

/** The listing's stable id, or `undefined` when it has none to trace an event to. */
export function stableListingID(
  listing: Pick<Listing, 'id'>,
): string | undefined {
  if (listing.id == null) return undefined;
  const id = String(listing.id).trim();
  return id === '' ? undefined : id;
}

/**
 * Records one contact intent. Resolves with the backend's record; rejects when
 * the event did not land, so the caller can tell the searcher that tracking
 * failed without ever standing between them and the agency.
 */
export async function recordContactIntent(
  listingId: string,
  source: ContactIntentSource = 'search_result_card',
): Promise<ContactIntentRecord> {
  const intentID = crypto.randomUUID();

  const response = await fetch(contactIntentPath(listingId), {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ intent_id: intentID, source }),
  });

  if (!response.ok) {
    throw new Error(`contact intent rejected with ${response.status}`);
  }

  const record = (await response.json()) as ContactIntentRecord;
  if (record.intent_id !== intentID || record.recorded !== true) {
    throw new Error('contact intent was not recorded');
  }
  return record;
}
