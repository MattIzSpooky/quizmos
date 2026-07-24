// Package auth validates Keycloak-issued JWT bearer tokens as a pure
// resource server: no local admin table, no token issuance, just signature
// + issuer + role verification against Keycloak's JWKS.
package auth

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/lestrrat-go/jwx/v3/jwk"
	"github.com/lestrrat-go/jwx/v3/jwt"
)

type contextKey string

const claimsContextKey contextKey = "quizmos.admin.claims"

// AdminClaims is the subset of the access token we care about.
type AdminClaims struct {
	Subject string
	// Username is the token's preferred_username claim — a human-readable
	// identifier (e.g. "alice@quizmos.dev"), unlike Subject, which is an
	// opaque Keycloak user ID. It's what distinguishes one admin from
	// another in logs (see DisplayName) now that there's more than one.
	Username string
	Roles    []string
}

func (c AdminClaims) HasRole(role string) bool {
	for _, r := range c.Roles {
		if r == role {
			return true
		}
	}
	return false
}

// DisplayName returns a human-readable identifier for the admin, for
// logging — their Keycloak username, falling back to the raw subject on
// the off chance a token lacks preferred_username (e.g. a client not
// granted the standard "profile" scope).
func (c AdminClaims) DisplayName() string {
	if c.Username != "" {
		return c.Username
	}
	return c.Subject
}

// ClaimsFromContext returns the authenticated admin's claims, if any.
func ClaimsFromContext(ctx context.Context) (AdminClaims, bool) {
	claims, ok := ctx.Value(claimsContextKey).(AdminClaims)
	return claims, ok
}

// Keycloak validates bearer tokens issued by a Keycloak realm.
type Keycloak struct {
	issuer    string
	jwksURL   string
	adminRole string

	mu   sync.RWMutex
	keys jwk.Set
}

func NewKeycloak(issuer, adminRole string) *Keycloak {
	return &Keycloak{
		issuer:    strings.TrimSuffix(issuer, "/"),
		jwksURL:   strings.TrimSuffix(issuer, "/") + "/protocol/openid-connect/certs",
		adminRole: adminRole,
	}
}

// StartRefresh fetches the JWKS immediately and then keeps it refreshed in
// the background until ctx is cancelled.
func (k *Keycloak) StartRefresh(ctx context.Context, interval time.Duration) error {
	if err := k.refresh(ctx); err != nil {
		return fmt.Errorf("initial JWKS fetch from %s: %w", k.jwksURL, err)
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				_ = k.refresh(ctx)
			}
		}
	}()
	return nil
}

func (k *Keycloak) refresh(ctx context.Context) error {
	set, err := jwk.Fetch(ctx, k.jwksURL)
	if err != nil {
		return err
	}
	k.mu.Lock()
	k.keys = set
	k.mu.Unlock()
	return nil
}

func (k *Keycloak) keySet() jwk.Set {
	k.mu.RLock()
	defer k.mu.RUnlock()
	return k.keys
}

type realmAccess struct {
	Roles []string `json:"roles"`
}

func init() {
	// jwx only unmarshals unregistered custom claims into
	// map[string]interface{}; registering the shape lets Token.Get
	// decode "realm_access" straight into our struct.
	jwt.RegisterCustomField("realm_access", realmAccess{})
}

// Verify parses and validates a raw bearer token, returning the caller's
// admin claims. It does not check role membership; callers decide that.
func (k *Keycloak) Verify(token string) (AdminClaims, error) {
	set := k.keySet()
	if set == nil {
		return AdminClaims{}, fmt.Errorf("JWKS not yet loaded")
	}

	parsed, err := jwt.Parse([]byte(token), jwt.WithKeySet(set), jwt.WithIssuer(k.issuer))
	if err != nil {
		return AdminClaims{}, fmt.Errorf("verify token: %w", err)
	}

	var ra realmAccess
	_ = parsed.Get("realm_access", &ra) // absent for tokens with no realm roles

	var username string
	_ = parsed.Get("preferred_username", &username) // absent if the client doesn't request the "profile" scope

	sub, _ := parsed.Subject()
	return AdminClaims{Subject: sub, Username: username, Roles: ra.Roles}, nil
}

// RequireAdminToken validates a raw "Authorization" header value ("Bearer
// <token>") and checks the admin role, returning the claims on success.
// Shared by AuthenticationFunc (the normal path, via openapi3filter) and
// by routes that bypass the standard request validator entirely — the
// question-media upload/delete handlers, whose multipart body the
// validator would otherwise consume before the handler can stream it to
// storage (see httpserver's Skipper).
func (k *Keycloak) RequireAdminToken(header string) (AdminClaims, error) {
	const prefix = "Bearer "
	if !strings.HasPrefix(header, prefix) {
		return AdminClaims{}, &httpStatusError{status: http.StatusUnauthorized, message: "missing bearer token"}
	}

	claims, err := k.Verify(strings.TrimPrefix(header, prefix))
	if err != nil {
		return AdminClaims{}, &httpStatusError{status: http.StatusUnauthorized, message: "invalid bearer token"}
	}
	if !claims.HasRole(k.adminRole) {
		return AdminClaims{}, &httpStatusError{status: http.StatusForbidden, message: "missing required role"}
	}
	return claims, nil
}

// AuthenticationFunc plugs into oapi-codegen's request validator
// (openapi3filter.Options.AuthenticationFunc). It only runs for operations
// whose OpenAPI spec declares `security: [{bearerAuth: []}]`, so it doubles
// as the sole gate on admin routes.
func (k *Keycloak) AuthenticationFunc(ctx context.Context, ai *openapi3filter.AuthenticationInput) error {
	if ai.SecuritySchemeName != "bearerAuth" {
		return fmt.Errorf("unsupported security scheme %q", ai.SecuritySchemeName)
	}

	claims, err := k.RequireAdminToken(ai.RequestValidationInput.Request.Header.Get("Authorization"))
	if err != nil {
		return err
	}

	*ai.RequestValidationInput.Request = *ai.RequestValidationInput.Request.WithContext(
		context.WithValue(ai.RequestValidationInput.Request.Context(), claimsContextKey, claims),
	)
	return nil
}

// httpStatusError lets the router's error handler map validator failures to
// the right HTTP status instead of a blanket 400.
type httpStatusError struct {
	status  int
	message string
}

func (e *httpStatusError) Error() string { return e.message }

func (e *httpStatusError) HTTPStatus() int { return e.status }
