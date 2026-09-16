import { contactIntentPath } from '@/lib/contact-intent';
import { passThrough, postToBackend, unreachable } from '@/lib/auth';

// Same-origin seam for the buyer's contact intent. It forwards the browser's
// body verbatim — {intent_id, source} — and nothing else: listing ownership and
// the aggregate are the backend's to derive, so the client never gets to name an
// agency, a realtor, or a count. The listing id comes from the route segment,
// never from parsing the listing's publication URL.
export async function POST(
  request: Request,
  { params }: { params: Promise<{ listingId: string }> },
) {
  const { listingId } = await params;
  if (!listingId) {
    return Response.json(
      { error: 'Falta el identificador de la propiedad.' },
      { status: 400 },
    );
  }

  const body = await request.text();

  try {
    return await passThrough(
      await postToBackend(contactIntentPath(listingId), body),
    );
  } catch {
    return unreachable();
  }
}
