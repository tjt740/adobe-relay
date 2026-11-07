package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

// Explicit, test-only fixture; never compiled into the deployed executable.
func TestBrowserFixture(t *testing.T) {
	ready := os.Getenv("DOCS_BROWSER_READY_FILE")
	if ready == "" {
		t.Skip("browser fixture not requested")
	}
	var usedRefresh sync.Map
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/auth/login":
			var b map[string]string
			json.NewDecoder(r.Body).Decode(&b)
			if b["password"] != "fixture-password" {
				w.WriteHeader(401)
				w.Write([]byte(`{"code":401}`))
				return
			}
			if b["email"] == "2fa@example.com" {
				w.Write([]byte(`{"code":0,"data":{"requires_2fa":true,"temp_token":"fixture-challenge"}}`))
				return
			}
			role := "user"
			if b["email"] == "admin@example.com" {
				role = "admin"
			}
			json.NewEncoder(w).Encode(map[string]any{"code": 0, "data": map[string]any{"access_token": role, "refresh_token": "browser-refresh", "expires_in": 1800, "user": map[string]any{"id": 8, "status": "active", "role": role}}})
		case "/api/v1/auth/login/2fa":
			var b map[string]string
			json.NewDecoder(r.Body).Decode(&b)
			if b["temp_token"] != "fixture-challenge" || b["totp_code"] != "123456" {
				w.WriteHeader(401)
				w.Write([]byte(`{"code":401}`))
				return
			}
			w.Write([]byte(`{"code":0,"data":{"access_token":"user","refresh_token":"browser-refresh","expires_in":1800,"user":{"id":8,"role":"user","status":"active"}}}`))
		case "/api/v1/auth/refresh":
			var body map[string]string
			json.NewDecoder(r.Body).Decode(&body)
			_, reused := usedRefresh.LoadOrStore(body["refresh_token"], true)
			if !strings.HasPrefix(body["refresh_token"], "valid-refresh-") || reused {
				w.WriteHeader(401)
				w.Write([]byte(`{"code":401}`))
				return
			}
			json.NewEncoder(w).Encode(map[string]any{"code": 0, "data": map[string]any{"access_token": "user-fresh", "refresh_token": "rotated-" + body["refresh_token"], "expires_in": 1800}})
		case "/api/v1/auth/me":
			if r.Header.Get("Authorization") == "Bearer outage-token" {
				w.WriteHeader(503)
				w.Write([]byte(`{"code":503}`))
				return
			}
			if r.Header.Get("Authorization") == "Bearer inactive-token" {
				w.Write([]byte(`{"code":0,"data":{"id":8,"status":"disabled"}}`))
				return
			}
			if r.Header.Get("Authorization") != "Bearer user" && r.Header.Get("Authorization") != "Bearer admin" && r.Header.Get("Authorization") != "Bearer user-fresh" {
				w.WriteHeader(401)
				w.Write([]byte(`{"code":401}`))
				return
			}
			w.Write([]byte(`{"code":0,"data":{"id":8,"status":"active"}}`))
		case "/api/v1/auth/logout":
			w.Write([]byte(`{"code":0,"data":{}}`))
		default:
			w.WriteHeader(404)
		}
	}))
	defer backend.Close()
	a := newApp(backend.URL, "")
	target, _ := url.Parse(backend.URL)
	proxy := httputil.NewSingleHostReverseProxy(target)
	site := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/v1/") {
			proxy.ServeHTTP(w, r)
			return
		}
		a.ServeHTTP(w, r)
	}))
	defer site.Close()
	a.origin = site.URL
	if err := os.WriteFile(ready, []byte(site.URL), 0600); err != nil {
		t.Fatal(err)
	}
	// Parent test runner terminates this isolated process when browser checks finish.
	time.Sleep(5 * time.Minute)
}
