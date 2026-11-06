package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

const origin = "https://docs.creative.cheap"

func request(a *app, method, path, body string, cookie *http.Cookie) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, origin+path, strings.NewReader(body))
	r.Header.Set("User-Agent", "docs-test")
	r.Header.Set("X-Real-IP", "203.0.113.7")
	if method == "POST" {
		r.Header.Set("Origin", origin)
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	if cookie != nil {
		r.AddCookie(cookie)
	}
	w := httptest.NewRecorder()
	a.ServeHTTP(w, r)
	return w
}
func fixture(t *testing.T) (*app, *atomic.Int32, *atomic.Int32) {
	t.Helper()
	state := &atomic.Int32{}
	hits := &atomic.Int32{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Header().Set("Content-Type", "application/json")
		if r.Header.Get("X-Real-IP") != "203.0.113.7" || r.UserAgent() != "docs-test" {
			t.Error("binding headers not preserved")
		}
		switch r.URL.Path {
		case "/api/v1/auth/login":
			var b map[string]string
			json.NewDecoder(r.Body).Decode(&b)
			if b["password"] != "correct" {
				w.WriteHeader(401)
				w.Write([]byte(`{"code":401}`))
				return
			}
			if b["email"] == "2fa@example.com" {
				w.Write([]byte(`{"code":0,"data":{"requires_2fa":true,"temp_token":"private-challenge"}}`))
				return
			}
			w.Write([]byte(`{"code":0,"data":{"access_token":"private-jwt","refresh_token":"private-refresh","expires_in":1800}}`))
		case "/api/v1/auth/login/2fa":
			var b map[string]string
			json.NewDecoder(r.Body).Decode(&b)
			if b["temp_token"] != "private-challenge" || b["totp_code"] != "123456" {
				w.WriteHeader(401)
				w.Write([]byte(`{"code":401}`))
				return
			}
			w.Write([]byte(`{"code":0,"data":{"access_token":"private-jwt","expires_in":1800}}`))
		case "/api/v1/auth/me":
			if r.Header.Get("Authorization") != "Bearer private-jwt" {
				t.Error("wrong auth")
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
			default:
				w.Write([]byte(`{"code":0,"data":{"id":8,"status":"active"}}`))
			}
		case "/api/v1/auth/logout":
			w.Write([]byte(`{"code":0,"data":{}}`))
		default:
			t.Error("unexpected upstream path", r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	t.Cleanup(srv.Close)
	return newApp(srv.URL, origin), state, hits
}
func login(t *testing.T, a *app) *http.Cookie {
	t.Helper()
	w := request(a, "POST", "/auth/login", url.Values{"email": {"user@example.com"}, "password": {"correct"}}.Encode(), nil)
	if w.Code != 303 {
		t.Fatal(w.Code, w.Body.String())
	}
	return w.Result().Cookies()[0]
}
func TestAnonymousContentDenied(t *testing.T) {
	a, _, hits := fixture(t)
	for _, path := range []string{"/", "/guide.md", "/assets/guide.md", "/assets/../guide.md", "/guide.md?raw=1", "/api/v1/auth/me", "/.git/config"} {
		w := request(a, "GET", path, "", nil)
		if w.Code == 200 || strings.Contains(w.Body.String(), "frame_duration_ms") {
			t.Fatal(path, "content exposed")
		}
	}
	if hits.Load() != 0 {
		t.Fatal("anonymous reached upstream")
	}
}
func TestSessionValidationDownloadAndLogout(t *testing.T) {
	a, _, _ := fixture(t)
	c := login(t, a)
	if !c.Secure || !c.HttpOnly || c.SameSite != http.SameSiteStrictMode || c.Domain != "" || c.Path != "/" || strings.Contains(c.Value, "private") {
		t.Fatal("unsafe cookie")
	}
	for _, path := range []string{"/", "/guide.md"} {
		w := request(a, "GET", path, "", c)
		if w.Code != 200 || !strings.Contains(w.Body.String(), "frame_duration_ms") {
			t.Fatal("missing document", w.Code)
		}
		if strings.Contains(w.Body.String(), "private-jwt") || !strings.Contains(w.Header().Get("Cache-Control"), "no-store") {
			t.Fatal("credential or cache leak")
		}
	}
	w := request(a, "POST", "/auth/logout", "", c)
	if w.Code != 303 {
		t.Fatal(w.Code)
	}
	if request(a, "GET", "/", "", c).Code != 303 {
		t.Fatal("logout not enforced")
	}
}
func TestRevocationAndFailClosed(t *testing.T) {
	for _, status := range []int32{1, 2, 3, 4} {
		t.Run(string(rune('0'+status)), func(t *testing.T) {
			a, s, _ := fixture(t)
			c := login(t, a)
			s.Store(status)
			w := request(a, "GET", "/", "", c)
			if w.Code == 200 || strings.Contains(w.Body.String(), "frame_duration_ms") {
				t.Fatal("failure exposed document")
			}
			if status == 2 && w.Code != 503 {
				t.Fatal("outage must stay closed")
			}
		})
	}
}
func Test2FAAndRotation(t *testing.T) {
	a, _, _ := fixture(t)
	w := request(a, "POST", "/auth/login", "email=2fa%40example.com&password=correct", nil)
	c := w.Result().Cookies()[0]
	if request(a, "GET", "/", "", c).Code != 303 {
		t.Fatal("pending login opened document")
	}
	if request(a, "POST", "/auth/2fa", "totp_code=000000", c).Code != 401 {
		t.Fatal("bad code accepted")
	}
	w = request(a, "POST", "/auth/2fa", "totp_code=123456", c)
	if w.Code != 303 {
		t.Fatal(w.Code)
	}
	next := w.Result().Cookies()[0]
	if next.Value == c.Value || request(a, "GET", "/", "", next).Code != 200 || request(a, "GET", "/", "", c).Code == 200 {
		t.Fatal("2FA/session rotation invalid")
	}
}
func TestOriginExpiryAndBinding(t *testing.T) {
	a, _, hits := fixture(t)
	r := httptest.NewRequest("POST", origin+"/auth/login", strings.NewReader("email=a&password=correct"))
	r.Header.Set("Origin", "https://evil.example")
	w := httptest.NewRecorder()
	a.ServeHTTP(w, r)
	if w.Code != 403 || hits.Load() != 0 {
		t.Fatal("CSRF accepted")
	}
	c := login(t, a)
	a.mu.Lock()
	s := a.sessions[c.Value]
	s.Until = time.Now().Add(-time.Second)
	a.sessions[c.Value] = s
	a.mu.Unlock()
	if request(a, "GET", "/", "", c).Code == 200 {
		t.Fatal("expired accepted")
	}
	c = login(t, a)
	r = httptest.NewRequest("GET", origin+"/", nil)
	r.Header.Set("X-Real-IP", "203.0.113.8")
	r.Header.Set("User-Agent", "docs-test")
	r.AddCookie(c)
	w = httptest.NewRecorder()
	a.ServeHTTP(w, r)
	if w.Code == 200 {
		t.Fatal("binding bypass")
	}
}
func TestWrongPasswordAndNoTokenLeak(t *testing.T) {
	a, _, _ := fixture(t)
	w := request(a, "POST", "/auth/login", "email=user%40example.com&password=wrong", nil)
	if w.Code != 401 || len(w.Result().Cookies()) != 0 {
		t.Fatal("wrong password accepted")
	}
	w = request(a, "POST", "/auth/login", "email=2fa%40example.com&password=correct", nil)
	if strings.Contains(w.Body.String(), "private-") {
		t.Fatal("token leaked")
	}
}
func TestLoginRestart(t *testing.T) {
	a, _, _ := fixture(t)
	w := request(a, "POST", "/auth/login", "email=2fa%40example.com&password=correct", nil)
	c := w.Result().Cookies()[0]
	w = request(a, "GET", "/login?restart=1", "", c)
	if w.Code != 303 {
		t.Fatal(w.Code)
	}
	if strings.Contains(request(a, "GET", "/login", "", c).Body.String(), `action="/auth/2fa"`) {
		t.Fatal("pending challenge retained")
	}
}
func TestConcurrentReads(t *testing.T) {
	a, _, _ := fixture(t)
	c := login(t, a)
	for i := 0; i < 10; i++ {
		t.Run("reader", func(t *testing.T) {
			t.Parallel()
			if request(a, "GET", "/", "", c).Code != 200 {
				t.Error("read failed")
			}
		})
	}
}
