// The session cookie is HttpOnly, so scripts cannot see it. This hint records
// that this browser signed in, so the search page asks for the saved
// qualification only then, instead of sending every anonymous visit a 401.
// It is a hint, not auth: the backend still decides with the cookie.
const KEY = 'hausy_signed_in';

export function rememberSignedIn(signedIn: boolean) {
  try {
    if (signedIn) localStorage.setItem(KEY, '1');
    else localStorage.removeItem(KEY);
  } catch {
    // Private mode or blocked storage: the profile just is not prefilled.
  }
}

export function wasSignedIn(): boolean {
  try {
    return localStorage.getItem(KEY) === '1';
  } catch {
    return false;
  }
}
