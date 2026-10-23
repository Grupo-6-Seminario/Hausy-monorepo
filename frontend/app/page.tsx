import { backendURL } from '@/lib/auth';

import { SearchExperience } from './components/search-experience';

// Rendered per request: the catalog is the backend's, never the build's.
export const dynamic = 'force-dynamic';

// serverFacts brings the qualification form's questions with the page, so the
// form renders without waiting for a request after hydration. A slow or
// unreachable backend never holds the page: the client asks instead.
async function serverFacts(): Promise<unknown> {
  try {
    const response = await fetch(`${backendURL}/api/eligibility/facts`, {
      signal: AbortSignal.timeout(1500),
    });
    return response.ok ? await response.json() : null;
  } catch {
    return null;
  }
}

export default async function Home() {
  return <SearchExperience initialFacts={await serverFacts()} />;
}
