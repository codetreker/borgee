package api_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"borgee-server/internal/api"
	"borgee-server/internal/auth"
	"borgee-server/internal/store"
)

// stubRemoteProxy lets the stat handler test drive success / timeout / offline
// branches without a real WS conn or the 10s SendRequest timeout.
type stubRemoteProxy struct {
	online bool
	resp   json.RawMessage
	err    error
}

func (s *stubRemoteProxy) IsNodeOnline(string) bool { return s.online }
func (s *stubRemoteProxy) ProxyRequest(_ string, _ string, _ string) (json.RawMessage, error) {
	return s.resp, s.err
}

// newStatFixture builds a RemoteHandler over a real store with a seeded owner +
// node, plus a second non-owner user (distinct email — store.User.Email has a
// unique index) for the OwnerMismatch branch. Only the Hub (RemoteProxy) is stubbed.
func newStatFixture(t *testing.T, hub api.RemoteProxy) (*api.RemoteHandler, *store.User, *store.User, string) {
	t.Helper()
	s := store.MigratedStoreFromTemplate(t)
	t.Cleanup(func() { s.Close() })

	ownerEmail := "owner-stat@test.com"
	owner := &store.User{DisplayName: "Owner", Role: "member", Email: &ownerEmail, PasswordHash: "x"}
	if err := s.CreateUser(owner); err != nil {
		t.Fatalf("create owner: %v", err)
	}

	otherEmail := "other-stat@test.com"
	other := &store.User{DisplayName: "Other", Role: "member", Email: &otherEmail, PasswordHash: "x"}
	if err := s.CreateUser(other); err != nil {
		t.Fatalf("create other: %v", err)
	}

	node, err := s.CreateRemoteNode(owner.ID, "stat-node")
	if err != nil {
		t.Fatalf("create node: %v", err)
	}

	h := &api.RemoteHandler{
		Store:  s,
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		Hub:    hub,
	}
	return h, owner, other, node.ID
}

// serveStat drives the stat route through the real mux (so {nodeId} PathValue is
// populated and route registration is exercised). injectMw is the test stand-in
// for auth.AuthMiddleware: it injects the ctx user, matching the production
// "ctx already has a user -> short-circuit" semantics.
func serveStat(t *testing.T, h *api.RemoteHandler, u *store.User, nodeID, query string) *httptest.ResponseRecorder {
	t.Helper()
	mux := http.NewServeMux()
	injectMw := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if u != nil {
				r = r.WithContext(auth.ContextWithUser(r.Context(), u))
			}
			next.ServeHTTP(w, r)
		})
	}
	h.RegisterRoutes(mux, injectMw)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/remote/nodes/"+nodeID+"/stat"+query, nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

func TestHandleNodeStat(t *testing.T) {
	t.Run("OwnerMismatch", func(t *testing.T) {
		h, _, other, nodeID := newStatFixture(t, &stubRemoteProxy{online: true})
		rec := serveStat(t, h, other, nodeID, "?path=/x")
		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected 403, got %d (body %s)", rec.Code, rec.Body.String())
		}
	})

	t.Run("Offline", func(t *testing.T) {
		h, owner, _, nodeID := newStatFixture(t, &stubRemoteProxy{online: false})
		rec := serveStat(t, h, owner, nodeID, "?path=/x")
		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("expected 503, got %d (body %s)", rec.Code, rec.Body.String())
		}
	})

	t.Run("Timeout", func(t *testing.T) {
		h, owner, _, nodeID := newStatFixture(t, &stubRemoteProxy{online: true, err: context.DeadlineExceeded})
		rec := serveStat(t, h, owner, nodeID, "?path=/x")
		if rec.Code != http.StatusGatewayTimeout {
			t.Fatalf("expected 504, got %d (body %s)", rec.Code, rec.Body.String())
		}
	})

	t.Run("Success", func(t *testing.T) {
		body := json.RawMessage(`{"name":"README.md","type":"file","size":42}`)
		h, owner, _, nodeID := newStatFixture(t, &stubRemoteProxy{online: true, resp: body})
		rec := serveStat(t, h, owner, nodeID, "?path=/README.md")
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d (body %s)", rec.Code, rec.Body.String())
		}
		if rec.Body.String() != string(body) {
			t.Fatalf("expected passthrough body %s, got %s", body, rec.Body.String())
		}
	})

	t.Run("AgentError_FileNotFound", func(t *testing.T) {
		body := json.RawMessage(`{"error":"file_not_found"}`)
		h, owner, _, nodeID := newStatFixture(t, &stubRemoteProxy{online: true, resp: body})
		rec := serveStat(t, h, owner, nodeID, "?path=/missing")
		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d (body %s)", rec.Code, rec.Body.String())
		}
	})
}
