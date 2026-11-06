package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

// Explicit, test-only fixture; never compiled into the deployed executable.
func TestBrowserFixture(t *testing.T) {
	ready := os.Getenv("DOCS_BROWSER_READY_FILE")
	if ready == "" {
		t.Skip("browser fixture not requested")
	}
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
			json.NewEncoder(w).Encode(map[string]any{"code": 0, "data": map[string]any{"access_token": role, "expires_in": 1800}})
		case "/api/v1/auth/login/2fa":
			var b map[string]string
			json.NewDecoder(r.Body).Decode(&b)
			if b["temp_token"] != "fixture-challenge" || b["totp_code"] != "123456" {
				w.WriteHeader(401)
				w.Write([]byte(`{"code":401}`))
				return
			}
			w.Write([]byte(`{"code":0,"data":{"access_token":"user","expires_in":1800}}`))
		case "/api/v1/auth/me":
			if r.Header.Get("Authorization") != "Bearer user" && r.Header.Get("Authorization") != "Bearer admin" {
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
	site := httptest.NewTLSServer(a)
	defer site.Close()
	a.origin = site.URL
	if err := os.WriteFile(ready, []byte(site.URL), 0600); err != nil {
		t.Fatal(err)
	}
	// Parent test runner terminates this isolated process when browser checks finish.
	time.Sleep(5 * time.Minute)
}
