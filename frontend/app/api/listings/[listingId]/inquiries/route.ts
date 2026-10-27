import { inquiryPath } from '@/lib/inquiry';
import { passThrough, postToBackend, unreachable } from '@/lib/auth';

// Same-origin seam for a searcher's inquiry. It forwards the body verbatim:
// the backend assesses eligibility and builds what the agency receives, so the
// browser never names an agency, an owner, or a verdict.
export async function POST(
  request: Request,
  { params }: { params: Promise<{ listingId: string }> },
) {
  const { listingId } = await params;
  try {
    return await passThrough(
      await postToBackend(inquiryPath(listingId), await request.text()),
    );
  } catch {
    return unreachable();
  }
}
