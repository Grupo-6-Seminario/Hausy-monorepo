# Accounts

**User sees:** `/ingresar` has the tabs "Ingresar" and "Crear cuenta", with fields Nombre (sign-up only), Email and Contraseña.
- After a searcher signs up, the page offers "Completar mi perfil y buscar".
- After the realtor signs in, it offers "Ir al catálogo".
- A signed-in user can choose "Cerrar sesión".

The search page header always shows "Ingresar", even when signed in, because the link is static in `search-experience.tsx`.

**Source:**
- `frontend/app/ingresar/page.tsx`
- `frontend/app/components/auth-experience.tsx`
- `frontend/app/api/auth/*`
- `internal/httpapi/auth.go`
- `internal/auth`

**Drive:**
- **Sign up:** the "sign up" step in `scripts/journey.mjs`.
- **Sign in:** use the realtor `inmobiliaria@journeys.test` with password `journeys-realtor`.

**Automated:** `frontend/app/auth-experience.test.tsx` and both whole-stack journeys.
