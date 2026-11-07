package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func sharedFixture(t *testing.T) (*app, *atomic.Int32, *atomic.Int32) {
	t.Helper()
	state, hits := &atomic.Int32{}, &atomic.Int32{}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if r.URL.Path != "/api/v1/auth/me" || r.Method != "GET" {
			t.Error("shared access must only validate /me")
			w.WriteHeader(500)
			return
		}
		if r.UserAgent() != "docs-test" || r.Header.Get("X-Real-IP") != "203.0.113.7" {
			t.Error("session binding changed")
		}
		token := r.Header.Get("Authorization")
		if token != "Bearer admin-token" && token != "Bearer user-token" && token != "Bearer completed-2fa-token" {
			w.WriteHeader(401)
			w.Write([]byte(`{"code":401}`))
			return
		}
		switch state.Load() {
		case 1:
			w.WriteHeader(401)
			w.Write([]byte(`{"code":401}`))
		case 2:
			w.WriteHeader(503)
			w.Write([]byte(`{"code":503}`))
		case 3:
			w.Write([]byte(`{"code":0,"data":{"id":8,"status":"disabled"}}`))
		case 4:
			w.Write([]byte(`{"code":0,"data":{}}`))
		case 5:
			w.WriteHeader(429)
			w.Write([]byte(`{"code":429}`))
		default:
			w.Write([]byte(`{"code":0,"data":{"id":8,"status":"active"}}`))
		}
	}))
	t.Cleanup(upstream.Close)
	return newApp(upstream.URL, origin), state, hits
}
func sharedRequest(a *app, path, token, source string) *httptest.ResponseRecorder {
	r := httptest.NewRequest("POST", origin+basePath+path, nil)
	r.Header.Set("Origin", source)
	r.Header.Set("User-Agent", "docs-test")
	r.Header.Set("X-Real-IP", "203.0.113.7")
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	a.ServeHTTP(w, r)
	return w
}
func TestSharedSessionRequiresFreshProofForEveryRead(t *testing.T) {
	a, state, hits := sharedFixture(t)
	guide, _ := files.ReadFile("guide.md")
	for _, token := range []string{"admin-token", "user-token", "completed-2fa-token"} {
		for _, path := range []string{"/session", "/session/guide.md"} {
			w := sharedRequest(a, path, token, origin)
			if w.Code != 200 || !strings.Contains(w.Body.String(), "frame_duration_ms") {
				t.Fatal("missing shared document", w.Code)
			}
			if len(w.Result().Cookies()) != 0 || strings.Contains(w.Body.String(), token) || !strings.Contains(w.Header().Get("Cache-Control"), "no-store") {
				t.Fatal("shared token/cookie/cache leak")
			}
			if path == "/session/guide.md" && !bytes.Equal(w.Body.Bytes(), guide) {
				t.Fatal("download changed")
			}
		}
	}
	if len(a.sessions) != 0 || hits.Load() != 6 {
		t.Fatal("shared tokens retained or reads not validated")
	}
	// A previous successful proof cannot authorize a normal navigation or another read.
	for _, path := range []string{"/", "/guide.md", "/session", "/session/guide.md"} {
		if request(a, "GET", path, "", nil).Code == 200 {
			t.Fatal("shared read minted persistent access")
		}
	}
	if sharedRequest(a, "/session", "", origin).Code != 401 {
		t.Fatal("missing proof accepted")
	}
	state.Store(1)
	if sharedRequest(a, "/session", "user-token", origin).Code != 401 {
		t.Fatal("revoked token accepted")
	}
	// Logging out of docs must not revoke the upstream browser session.
	count := hits.Load()
	w := request(a, "POST", "/auth/logout", "", nil)
	if w.Code != 303 || w.Header().Get("Location") != "/docs/login?manual=1" || hits.Load() != count {
		t.Fatal("logout affects Sub2 or loops into auto login")
	}
}
func TestSharedSessionRejectsPendingInvalidAndUntrustedSources(t *testing.T) {
	a, _, hits := sharedFixture(t)
	for _, source := range []string{"", "null", "https://evil.example", "https://creative.cheap.evil.example"} {
		if sharedRequest(a, "/session", "admin-token", source).Code != 403 {
			t.Fatal("shared CSRF accepted")
		}
	}
	for _, token := range []string{"", strings.Repeat("x", 8193), "token with spaces"} {
		if sharedRequest(a, "/session", token, origin).Code != 401 {
			t.Fatal("malformed token accepted")
		}
	}
	if hits.Load() != 0 {
		t.Fatal("invalid requests reached upstream")
	}
	for _, token := range []string{"forged-token", "pending-2fa-challenge"} {
		if sharedRequest(a, "/session", token, origin).Code != 401 {
			t.Fatal("incomplete/forged login accepted")
		}
	}
	for _, path := range []string{"/session?access_token=admin-token", "/session/guide.md?token=admin-token"} {
		if request(a, "POST", path, "access_token=admin-token&user_id=8&status=active", nil).Code != 401 {
			t.Fatal("query/form identity accepted")
		}
	}
}
func TestSharedSessionFailureRemainsClosed(t *testing.T) {
	for _, state := range []int32{1, 2, 3, 4, 5} {
		a, s, _ := sharedFixture(t)
		s.Store(state)
		for _, path := range []string{"/session", "/session/guide.md"} {
			w := sharedRequest(a, path, "user-token", origin)
			if w.Code == 200 || strings.Contains(w.Body.String(), "frame_duration_ms") || len(w.Result().Cookies()) != 0 {
				t.Fatal("unverified content exposed")
			}
			if (state == 2 || state == 5) && w.Code != 503 {
				t.Fatal("outage not distinguished")
			}
		}
	}
}
func TestSharedLoginFallbackAndManualOverride(t *testing.T) {
	a, _, _ := fixture(t)
	if !strings.Contains(request(a, "GET", "/login", "", nil).Body.String(), `data-auto-login="true"`) {
		t.Fatal("normal entry cannot reuse Sub2")
	}
	if strings.Contains(request(a, "GET", "/login?manual=1", "", nil).Body.String(), `data-auto-login="true"`) {
		t.Fatal("explicit logout re-enters automatically")
	}
	c := login(t, a)
	// A valid independent docs cookie never replaces missing browser proof.
	if request(a, "POST", "/session", "", c).Code != 401 {
		t.Fatal("cookie substituted for shared bearer")
	}
	w := request(a, "POST", "/auth/login", "email=2fa%40example.com&password=correct", nil)
	pending := w.Result().Cookies()[0]
	if strings.Contains(request(a, "GET", "/login", "", pending).Body.String(), `data-auto-login="true"`) {
		t.Fatal("automatic login interrupts pending 2FA")
	}
}
