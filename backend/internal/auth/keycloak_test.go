package auth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/lestrrat-go/jwx/v3/jwa"
	"github.com/lestrrat-go/jwx/v3/jwk"
	"github.com/lestrrat-go/jwx/v3/jwt"
)

// fakeJWKS is a minimal stand-in for Keycloak's JWKS endpoint: it signs
// tokens with a locally generated key and serves the matching public key
// at /protocol/openid-connect/certs, exactly where NewKeycloak expects it.
type fakeJWKS struct {
	server *httptest.Server
	priv   *rsa.PrivateKey
	key    jwk.Key
	issuer string
}

func newFakeJWKS(t *testing.T) *fakeJWKS {
	t.Helper()
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate RSA key: %v", err)
	}
	key, err := jwk.Import(priv)
	if err != nil {
		t.Fatalf("import key: %v", err)
	}
	if err := key.Set(jwk.KeyIDKey, "test-key-1"); err != nil {
		t.Fatalf("set kid: %v", err)
	}
	if err := key.Set(jwk.AlgorithmKey, jwa.RS256()); err != nil {
		t.Fatalf("set alg: %v", err)
	}

	pubSet := jwk.NewSet()
	pubKey, err := jwk.PublicKeyOf(key)
	if err != nil {
		t.Fatalf("derive public key: %v", err)
	}
	if err := pubSet.AddKey(pubKey); err != nil {
		t.Fatalf("add public key to set: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/protocol/openid-connect/certs", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(pubSet); err != nil {
			t.Errorf("encode JWKS: %v", err)
		}
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	return &fakeJWKS{server: server, priv: priv, key: key, issuer: server.URL}
}

func (f *fakeJWKS) mint(t *testing.T, roles []string, expiry time.Duration) string {
	t.Helper()
	builder := jwt.NewBuilder().
		Issuer(f.issuer).
		Subject("user-123").
		IssuedAt(time.Now()).
		Expiration(time.Now().Add(expiry))
	if roles != nil {
		builder = builder.Claim("realm_access", map[string]any{"roles": roles})
	}
	token, err := builder.Build()
	if err != nil {
		t.Fatalf("build token: %v", err)
	}
	signed, err := jwt.Sign(token, jwt.WithKey(jwa.RS256(), f.key))
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return string(signed)
}

// mintWithUsername is mint plus a preferred_username claim, kept separate
// rather than added as a parameter to mint so the many existing call sites
// that don't care about it stay untouched.
func (f *fakeJWKS) mintWithUsername(t *testing.T, username string, roles []string, expiry time.Duration) string {
	t.Helper()
	builder := jwt.NewBuilder().
		Issuer(f.issuer).
		Subject("user-123").
		Claim("preferred_username", username).
		IssuedAt(time.Now()).
		Expiration(time.Now().Add(expiry))
	if roles != nil {
		builder = builder.Claim("realm_access", map[string]any{"roles": roles})
	}
	token, err := builder.Build()
	if err != nil {
		t.Fatalf("build token: %v", err)
	}
	signed, err := jwt.Sign(token, jwt.WithKey(jwa.RS256(), f.key))
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return string(signed)
}

func newTestKeycloak(t *testing.T, issuer string) *Keycloak {
	t.Helper()
	kc := NewKeycloak(issuer, "quiz-admin")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := kc.StartRefresh(ctx, time.Hour); err != nil {
		t.Fatalf("StartRefresh: %v", err)
	}
	return kc
}

func TestVerify_ValidTokenWithRole(t *testing.T) {
	jwks := newFakeJWKS(t)
	kc := newTestKeycloak(t, jwks.issuer)

	token := jwks.mint(t, []string{"quiz-admin", "some-other-role"}, time.Hour)

	claims, err := kc.Verify(token)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if claims.Subject != "user-123" {
		t.Errorf("Subject = %q, want %q", claims.Subject, "user-123")
	}
	if !claims.HasRole("quiz-admin") {
		t.Errorf("expected HasRole(quiz-admin) = true, roles = %v", claims.Roles)
	}
	if claims.HasRole("nonexistent-role") {
		t.Error("expected HasRole(nonexistent-role) = false")
	}
}

func TestVerify_ExtractsPreferredUsername(t *testing.T) {
	jwks := newFakeJWKS(t)
	kc := newTestKeycloak(t, jwks.issuer)

	token := jwks.mintWithUsername(t, "alice@quizmos.dev", []string{"quiz-admin"}, time.Hour)

	claims, err := kc.Verify(token)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if claims.Username != "alice@quizmos.dev" {
		t.Errorf("Username = %q, want %q", claims.Username, "alice@quizmos.dev")
	}
	if got := claims.DisplayName(); got != "alice@quizmos.dev" {
		t.Errorf("DisplayName() = %q, want %q", got, "alice@quizmos.dev")
	}
}

func TestVerify_MissingPreferredUsernameFallsBackToSubjectInDisplayName(t *testing.T) {
	jwks := newFakeJWKS(t)
	kc := newTestKeycloak(t, jwks.issuer)

	token := jwks.mint(t, []string{"quiz-admin"}, time.Hour) // no preferred_username claim

	claims, err := kc.Verify(token)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if claims.Username != "" {
		t.Errorf("Username = %q, want empty", claims.Username)
	}
	if got := claims.DisplayName(); got != "user-123" {
		t.Errorf("DisplayName() = %q, want %q (fallback to Subject)", got, "user-123")
	}
}

func TestVerify_MissingRealmAccessClaim(t *testing.T) {
	jwks := newFakeJWKS(t)
	kc := newTestKeycloak(t, jwks.issuer)

	token := jwks.mint(t, nil, time.Hour)

	claims, err := kc.Verify(token)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if claims.HasRole("quiz-admin") {
		t.Error("expected no roles when realm_access is absent")
	}
}

func TestVerify_ExpiredToken(t *testing.T) {
	jwks := newFakeJWKS(t)
	kc := newTestKeycloak(t, jwks.issuer)

	token := jwks.mint(t, []string{"quiz-admin"}, -time.Hour)

	if _, err := kc.Verify(token); err == nil {
		t.Fatal("expected an error verifying an expired token, got nil")
	}
}

func TestVerify_WrongIssuerRejected(t *testing.T) {
	jwks := newFakeJWKS(t)
	// Configure the Keycloak client with a *different* issuer than the
	// one embedded in minted tokens, simulating a token from another
	// realm/environment.
	kc := NewKeycloak(jwks.issuer, "quiz-admin")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := kc.StartRefresh(ctx, time.Hour); err != nil {
		t.Fatalf("StartRefresh: %v", err)
	}

	builder := jwt.NewBuilder().
		Issuer("https://not-the-configured-issuer.example").
		Subject("user-123").
		Expiration(time.Now().Add(time.Hour)).
		Claim("realm_access", map[string]any{"roles": []string{"quiz-admin"}})
	token, err := builder.Build()
	if err != nil {
		t.Fatalf("build token: %v", err)
	}
	signed, err := jwt.Sign(token, jwt.WithKey(jwa.RS256(), jwks.key))
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}

	if _, err := kc.Verify(string(signed)); err == nil {
		t.Fatal("expected an error verifying a token with the wrong issuer, got nil")
	}
}

func TestVerify_TamperedSignatureRejected(t *testing.T) {
	jwks := newFakeJWKS(t)
	kc := newTestKeycloak(t, jwks.issuer)

	token := jwks.mint(t, []string{"quiz-admin"}, time.Hour)
	// Flip a character in the middle of the signature segment rather than
	// the very last one: base64url's trailing character can carry unused
	// padding bits, so mutating it sometimes decodes to the exact same
	// signature bytes and the test becomes flaky.
	mid := len(token) - 10
	replacement := byte('A')
	if token[mid] == 'A' {
		replacement = 'B'
	}
	tampered := token[:mid] + string(replacement) + token[mid+1:]

	if _, err := kc.Verify(tampered); err == nil {
		t.Fatal("expected an error verifying a tampered token, got nil")
	}
}

func TestRequireAdminToken_ValidTokenWithAdminRole(t *testing.T) {
	jwks := newFakeJWKS(t)
	kc := newTestKeycloak(t, jwks.issuer)
	token := jwks.mint(t, []string{"quiz-admin"}, time.Hour)

	claims, err := kc.RequireAdminToken("Bearer " + token)
	if err != nil {
		t.Fatalf("RequireAdminToken: %v", err)
	}
	if claims.Subject != "user-123" {
		t.Errorf("Subject = %q, want %q", claims.Subject, "user-123")
	}
}

func TestRequireAdminToken_MissingBearerPrefix(t *testing.T) {
	jwks := newFakeJWKS(t)
	kc := newTestKeycloak(t, jwks.issuer)
	token := jwks.mint(t, []string{"quiz-admin"}, time.Hour)

	_, err := kc.RequireAdminToken(token) // no "Bearer " prefix
	if err == nil {
		t.Fatal("expected an error for a header missing the Bearer prefix")
	}
	var statusErr interface{ HTTPStatus() int }
	if !errors.As(err, &statusErr) {
		t.Fatalf("expected an *httpStatusError, got %T", err)
	}
	if statusErr.HTTPStatus() != http.StatusUnauthorized {
		t.Errorf("HTTPStatus() = %d, want %d", statusErr.HTTPStatus(), http.StatusUnauthorized)
	}
}

func TestRequireAdminToken_InvalidToken(t *testing.T) {
	jwks := newFakeJWKS(t)
	kc := newTestKeycloak(t, jwks.issuer)

	_, err := kc.RequireAdminToken("Bearer not-a-real-token")
	if err == nil {
		t.Fatal("expected an error for an unparseable token")
	}
	var statusErr interface{ HTTPStatus() int }
	if !errors.As(err, &statusErr) || statusErr.HTTPStatus() != http.StatusUnauthorized {
		t.Errorf("expected a 401 status error, got %v", err)
	}
}

func TestRequireAdminToken_MissingAdminRole(t *testing.T) {
	jwks := newFakeJWKS(t)
	kc := newTestKeycloak(t, jwks.issuer)
	token := jwks.mint(t, []string{"some-other-role"}, time.Hour)

	_, err := kc.RequireAdminToken("Bearer " + token)
	if err == nil {
		t.Fatal("expected an error for a token without the admin role")
	}
	var statusErr interface{ HTTPStatus() int }
	if !errors.As(err, &statusErr) {
		t.Fatalf("expected an *httpStatusError, got %T", err)
	}
	if statusErr.HTTPStatus() != http.StatusForbidden {
		t.Errorf("HTTPStatus() = %d, want %d", statusErr.HTTPStatus(), http.StatusForbidden)
	}
}
