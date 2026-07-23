package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClientID_FromHeader(t *testing.T) {
	var got string
	var ok bool
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got, ok = ClientIDFromContext(r.Context())
	})

	req := httptest.NewRequest(http.MethodGet, "/ws", nil)
	req.Header.Set("X-Client-Id", "header-client")
	ClientID(next).ServeHTTP(httptest.NewRecorder(), req)

	if !ok || got != "header-client" {
		t.Errorf("ClientIDFromContext = (%q, %v), want (%q, true)", got, ok, "header-client")
	}
}

func TestClientID_FromQueryParam(t *testing.T) {
	var got string
	var ok bool
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got, ok = ClientIDFromContext(r.Context())
	})

	req := httptest.NewRequest(http.MethodGet, "/ws?client_id=query-client", nil)
	ClientID(next).ServeHTTP(httptest.NewRecorder(), req)

	if !ok || got != "query-client" {
		t.Errorf("ClientIDFromContext = (%q, %v), want (%q, true)", got, ok, "query-client")
	}
}

func TestClientID_HeaderTakesPrecedenceOverQueryParam(t *testing.T) {
	var got string
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got, _ = ClientIDFromContext(r.Context())
	})

	req := httptest.NewRequest(http.MethodGet, "/ws?client_id=query-client", nil)
	req.Header.Set("X-Client-Id", "header-client")
	ClientID(next).ServeHTTP(httptest.NewRecorder(), req)

	if got != "header-client" {
		t.Errorf("got %q, want header to take precedence (%q)", got, "header-client")
	}
}

func TestClientID_AbsentLeavesContextEmpty(t *testing.T) {
	var ok bool
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, ok = ClientIDFromContext(r.Context())
	})

	req := httptest.NewRequest(http.MethodGet, "/ws", nil)
	ClientID(next).ServeHTTP(httptest.NewRecorder(), req)

	if ok {
		t.Error("expected ClientIDFromContext to report not-ok when no client id was supplied")
	}
}

func TestClientIDFromContext_NotSet(t *testing.T) {
	_, ok := ClientIDFromContext(context.Background())
	if ok {
		t.Error("expected not-ok for a context with no client id set")
	}
}
