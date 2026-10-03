import { backendURL, passThrough, unreachable } from '@/lib/auth';

// The qualification form's questions. Public, like search: no session needed.
// The backend's Cache-Control is kept so the browser reuses the catalog.
export async function GET() {
  try {
    const backend = await fetch(`${backendURL}/api/eligibility/facts`);
    const response = await passThrough(backend);
    const cacheControl = backend.headers.get('Cache-Control');
    if (cacheControl) response.headers.set('Cache-Control', cacheControl);
    return response;
  } catch {
    return unreachable();
  }
}
