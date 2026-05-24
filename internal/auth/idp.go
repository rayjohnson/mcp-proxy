package auth

// IdentityProvider abstracts the sign-in mechanism so JumpCloud OIDC can be
// plugged in later without changing session or handler logic.
type IdentityProvider interface {
	// AuthURL returns the URL to redirect the user to for authorization.
	AuthURL(state string) string
	// Exchange exchanges an authorization code for a verified user identity.
	Exchange(code, state string) (*ExternalIdentity, error)
}

// ExternalIdentity is the normalized user info returned by an IdP after auth.
type ExternalIdentity struct {
	Email string
	Name  string
}
