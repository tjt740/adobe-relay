package main

import (
	"bytes"
	"crypto/rand"
	"embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
)

//go:embed assets/* guide.md
var files embed.FS

const cookieName = "__Host-docs_session"
const basePath = "/docs"

type session struct {
	Token, Refresh, Pending, IP, UA string
	Until                           time.Time
}
type app struct {
	upstream, origin string
	client           *http.Client
	mu               sync.Mutex
	sessions         map[string]session
	page             *template.Template
	doc              template.HTML
	gate             chan struct{}
}
type envelope struct {
	Code json.RawMessage `json:"code"`
	Data json.RawMessage `json:"data"`
}
type authData struct {
	Access      string `json:"access_token"`
	Refresh     string `json:"refresh_token"`
	Pending     string `json:"temp_token"`
	Requires2FA bool   `json:"requires_2fa"`
	Expires     int    `json:"expires_in"`
}
type pageData struct {
	Base              string
	Logged, TwoFA     bool
	AutoLogin, Shared bool
	Error             string
	Doc               template.HTML
}

func newApp(upstream, origin string) *app {
	source, err := files.ReadFile("guide.md")
	if err != nil {
		panic(err)
	}
	var out bytes.Buffer
	md := goldmark.New(goldmark.WithExtensions(extension.GFM), goldmark.WithParserOptions(parser.WithAutoHeadingID()))
	if err := md.Convert(source, &out); err != nil {
		panic(err)
	}
	return &app{upstream: upstream, origin: origin, client: &http.Client{Timeout: 15 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}, sessions: make(map[string]session), page: template.Must(template.ParseFS(files, "assets/page.html")), doc: template.HTML(out.String()), gate: make(chan struct{}, 32)}
}
func (a *app) api(r *http.Request, method, path, token string, body any) (int, json.RawMessage, error) {
	var b bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&b).Encode(body); err != nil {
			return 0, nil, err
		}
	}
	req, err := http.NewRequestWithContext(r.Context(), method, a.upstream+path, &b)
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", r.UserAgent())
	ip := clientIP(r)
	req.Header.Set("X-Real-IP", ip)
	req.Header.Set("X-Forwarded-For", ip)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	res, err := a.client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer res.Body.Close()
	var e envelope
	err = json.NewDecoder(io.LimitReader(res.Body, 256<<10)).Decode(&e)
	if err != nil {
		return res.StatusCode, nil, err
	}
	if res.StatusCode == 200 && !bytes.Equal(bytes.TrimSpace(e.Code), []byte("0")) {
		return 502, nil, fmt.Errorf("invalid upstream envelope")
	}
	return res.StatusCode, e.Data, nil
}
func clientIP(r *http.Request) string {
	// Listener is loopback-only; nginx overwrites this header, never appends client input.
	if ip := net.ParseIP(r.Header.Get("X-Real-IP")); ip != nil {
		return ip.String()
	}
	host, _, _ := net.SplitHostPort(r.RemoteAddr)
	return host
}
func (a *app) current(r *http.Request) (string, session, bool) {
	c, err := r.Cookie(cookieName)
	if err != nil {
		return "", session{}, false
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	s, ok := a.sessions[c.Value]
	if !ok || time.Now().After(s.Until) || s.IP != clientIP(r) || s.UA != r.UserAgent() {
		delete(a.sessions, c.Value)
		return "", session{}, false
	}
	return c.Value, s, true
}
func (a *app) issue(w http.ResponseWriter, r *http.Request, s session) bool {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return false
	}
	id := base64.RawURLEncoding.EncodeToString(raw)
	s.IP = clientIP(r)
	s.UA = r.UserAgent()
	a.mu.Lock()
	for key, v := range a.sessions {
		if time.Now().After(v.Until) {
			delete(a.sessions, key)
		}
	}
	if len(a.sessions) >= 10000 {
		a.mu.Unlock()
		return false
	}
	if c, err := r.Cookie(cookieName); err == nil {
		delete(a.sessions, c.Value)
	}
	a.sessions[id] = s
	a.mu.Unlock()
	http.SetCookie(w, &http.Cookie{Name: cookieName, Value: id, Path: "/", Secure: true, HttpOnly: true, SameSite: http.SameSiteStrictMode, MaxAge: max(1, int(time.Until(s.Until).Seconds()))})
	return true
}
func (a *app) clear(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(cookieName); err == nil {
		a.mu.Lock()
		delete(a.sessions, c.Value)
		a.mu.Unlock()
	}
	http.SetCookie(w, &http.Cookie{Name: cookieName, Value: "", Path: "/", Secure: true, HttpOnly: true, SameSite: http.SameSiteStrictMode, MaxAge: -1})
}
func (a *app) render(w http.ResponseWriter, status int, p pageData) {
	p.Base = basePath
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if err := a.page.Execute(w, p); err != nil {
		log.Print("page response interrupted")
	}
}
func (a *app) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == basePath {
		http.Redirect(w, r, basePath+"/", 308)
		return
	}
	if !strings.HasPrefix(r.URL.Path, basePath+"/") {
		http.NotFound(w, r)
		return
	}
	r = r.Clone(r.Context())
	r.URL.Path = strings.TrimPrefix(r.URL.Path, basePath)
	w.Header().Set("Cache-Control", "private, no-store, max-age=0")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Frame-Options", "DENY")
	// Native form POSTs serialize Origin as null under no-referrer.
	// Keep same-origin form requests compatible without leaking external referrers.
	w.Header().Set("Referrer-Policy", "same-origin")
	w.Header().Set("X-Robots-Tag", "noindex, nofollow, noarchive")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'self'; script-src 'self'; connect-src 'self'; img-src 'self' data:; form-action 'self'; frame-ancestors 'none'; base-uri 'none'")
	select {
	case a.gate <- struct{}{}:
		defer func() { <-a.gate }()
	default:
		http.Error(w, "服务繁忙，请稍后再试", 503)
		return
	}
	if r.Method == http.MethodPost {
		if r.Header.Get("Origin") != a.origin {
			a.render(w, http.StatusForbidden, pageData{Error: "请刷新登录页后重试。"})
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
		if err := r.ParseForm(); err != nil {
			http.Error(w, "请求内容无效", 400)
			return
		}
		switch r.URL.Path {
		case "/session", "/session/guide.md":
			a.sharedDocument(w, r)
			return
		case "/auth/login", "/auth/2fa":
			a.login(w, r)
			return
		case "/auth/logout":
			_, s, ok := a.current(r)
			a.clear(w, r)
			if ok && s.Refresh != "" {
				_, _, _ = a.api(r, "POST", "/api/v1/auth/logout", "", map[string]string{"refresh_token": s.Refresh})
			}
			http.Redirect(w, r, basePath+"/login?manual=1", http.StatusSeeOther)
			return
		default:
			http.NotFound(w, r)
			return
		}
	}
	if r.Method != "GET" && r.Method != "HEAD" {
		w.Header().Set("Allow", "GET, HEAD, POST")
		http.Error(w, "Method not allowed", 405)
		return
	}
	switch r.URL.Path {
	case "/assets/site.css", "/assets/site.js", "/assets/session.js":
		name := strings.TrimPrefix(r.URL.Path, "/")
		data, _ := files.ReadFile(name)
		if strings.HasSuffix(name, ".css") {
			w.Header().Set("Content-Type", "text/css; charset=utf-8")
		} else {
			w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
		}
		w.Write(data)
		return
	case "/healthz":
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte("ok\n"))
		return
	case "/robots.txt":
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte("User-agent: *\nDisallow: /\n"))
		return
	case "/login":
		if r.URL.Query().Get("restart") == "1" {
			a.clear(w, r)
			http.Redirect(w, r, basePath+"/login?manual=1", 303)
			return
		}
		_, s, ok := a.current(r)
		two := ok && s.Pending != ""
		a.render(w, 200, pageData{TwoFA: two, AutoLogin: !two && r.URL.Query().Get("manual") != "1"})
		return
	case "/", "/guide.md":
		_, s, ok := a.current(r)
		if !ok || s.Token == "" {
			target := basePath + "/login"
			if r.URL.Path == "/guide.md" {
				target += "?download=1"
			}
			http.Redirect(w, r, target, 303)
			return
		}
		status := a.validateIdentity(r, s.Token)
		if status == 503 {
			a.render(w, 503, pageData{Error: "暂时无法验证登录状态，请稍后重试。"})
			return
		}
		if status != 200 {
			a.clear(w, r)
			http.Redirect(w, r, basePath+"/login", 303)
			return
		}
		if r.URL.Path == "/guide.md" {
			a.download(w)
			return
		}
		a.render(w, 200, pageData{Logged: true, Doc: a.doc})
		return
	default:
		http.NotFound(w, r)
	}
}

func (a *app) validateIdentity(r *http.Request, token string) int {
	status, data, err := a.api(r, "GET", "/api/v1/auth/me", token, nil)
	if err != nil || status >= 500 || status == 429 {
		return http.StatusServiceUnavailable
	}
	var user struct {
		ID     int64  `json:"id"`
		Status string `json:"status"`
	}
	if status != 200 || json.Unmarshal(data, &user) != nil || user.ID <= 0 || user.Status != "active" {
		return http.StatusUnauthorized
	}
	return http.StatusOK
}

// Read-only reuse of a completed Sub2 login. Never mint a docs cookie or retain
// the shared token: each document/download requires fresh browser proof and /me.
func (a *app) sharedDocument(w http.ResponseWriter, r *http.Request) {
	header := r.Header.Get("Authorization")
	token, bearer := strings.CutPrefix(header, "Bearer ")
	if !bearer || token == "" || len(token) > 8192 || strings.ContainsAny(token, " \t\r\n") {
		http.Error(w, "请登录后查看文档。", http.StatusUnauthorized)
		return
	}
	status := a.validateIdentity(r, token)
	if status != http.StatusOK {
		http.Error(w, "暂时无法验证登录状态。", status)
		return
	}
	if r.URL.Path == "/session/guide.md" {
		a.download(w)
		return
	}
	a.render(w, http.StatusOK, pageData{Logged: true, Shared: true, Doc: a.doc})
}

func (a *app) download(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
	w.Header().Set("Content-Disposition", "attachment; filename=adobe-image-api.md")
	b, _ := files.ReadFile("guide.md")
	w.Write(b)
}
func (a *app) login(w http.ResponseWriter, r *http.Request) {
	path := "/api/v1/auth/login"
	body := map[string]string{"email": strings.TrimSpace(r.PostForm.Get("email")), "password": r.PostForm.Get("password")}
	two := r.URL.Path == "/auth/2fa"
	if two {
		_, s, ok := a.current(r)
		if !ok || s.Pending == "" {
			a.clear(w, r)
			a.render(w, 401, pageData{Error: "验证已过期，请重新登录。"})
			return
		}
		path += "/2fa"
		body = map[string]string{"temp_token": s.Pending, "totp_code": r.PostForm.Get("totp_code")}
	} else if len(body["email"]) > 254 || len(body["password"]) > 1024 || body["email"] == "" || body["password"] == "" {
		a.render(w, 400, pageData{Error: "请输入注册邮箱和密码。"})
		return
	}
	status, data, err := a.api(r, "POST", path, "", body)
	if err != nil || status != 200 {
		msg := "邮箱、密码或验证码不正确，或账号当前无法登录。"
		if status == 429 {
			msg = "尝试次数较多，请稍后重试。"
		} else if err != nil || status >= 500 {
			msg = "Sub2 登录服务暂时不可用，请稍后重试。"
		}
		a.render(w, 401, pageData{TwoFA: two, Error: msg})
		return
	}
	var d authData
	if json.Unmarshal(data, &d) != nil {
		a.render(w, 502, pageData{Error: "登录响应无效，请稍后重试。"})
		return
	}
	s := session{}
	if d.Requires2FA && d.Pending != "" {
		s.Pending = d.Pending
		s.Until = time.Now().Add(5 * time.Minute)
	} else if d.Access != "" && !d.Requires2FA {
		s.Token = d.Access
		s.Refresh = d.Refresh
		ttl := time.Hour
		if d.Expires > 0 && time.Duration(d.Expires)*time.Second < ttl {
			ttl = time.Duration(d.Expires) * time.Second
		}
		s.Until = time.Now().Add(ttl)
	} else {
		a.render(w, 502, pageData{Error: "登录未完成，请重新登录。"})
		return
	}
	if !a.issue(w, r, s) {
		a.render(w, 503, pageData{Error: "登录服务繁忙，请稍后重试。"})
		return
	}
	if s.Pending != "" {
		http.Redirect(w, r, basePath+"/login", 303)
	} else {
		http.Redirect(w, r, basePath+"/", 303)
	}
}
func main() {
	upstream := os.Getenv("SUB2_URL")
	if upstream == "" {
		upstream = "http://127.0.0.1:6666"
	}
	origin := os.Getenv("DOCS_ORIGIN")
	if origin == "" {
		origin = "https://creative.cheap"
	}
	addr := os.Getenv("DOCS_LISTEN")
	if addr == "" {
		addr = "127.0.0.1:6670"
	}
	host, _, err := net.SplitHostPort(addr)
	if err != nil || !net.ParseIP(host).IsLoopback() {
		log.Fatal("listener must be loopback")
	}
	server := &http.Server{Addr: addr, Handler: newApp(upstream, origin), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 20 * time.Second, WriteTimeout: 25 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16 << 10}
	log.Printf("docs listening on %s", addr)
	log.Fatal(server.ListenAndServe())
}
