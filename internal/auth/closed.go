package auth

import "context"

// ClosedToRealtors refuses realtor sign-ups and passes everything else to
// Provider. On a public deployment anyone could otherwise publish listings to
// every searcher; realtor accounts there come from the demo seed.
type ClosedToRealtors struct{ Provider }

// SignUp creates searcher accounts only.
func (p ClosedToRealtors) SignUp(ctx context.Context, registration Registration) (User, error) {
	if registration.Role == RoleRealtor {
		return User{}, RegistrationError{Reason: "El alta de inmobiliarias está cerrada en esta demo."}
	}
	return p.Provider.SignUp(ctx, registration)
}
