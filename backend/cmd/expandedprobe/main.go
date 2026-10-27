package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
 "encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/adobe"
)

type Account struct {
	Arm           string `json:"arm"`
	IntervalSec   int    `json:"interval_sec"`
	ID            int64  `json:"id"`
	Email         string `json:"email"`
	Token         string `json:"token"`
	OriginalToken string `json:"original_token"`
	Proxy         string `json:"proxy"`
	ProxyID       int64  `json:"proxy_id"`
	Concurrency   int    `json:"concurrency"`
	ARPSessionID  string `json:"arp_session_id"`
}
type Config struct {
	StartAt     string    `json:"start_at"`
	Round       int       `json:"round"`
	Accounts    []Account `json:"accounts"`
	DurationSec int       `json:"duration_sec"`
	MaxJobs     int       `json:"max_jobs"`
	Model       string    `json:"model"`
	GuardDir    string    `json:"guard_dir"`
}

var outputMu sync.Mutex

func event(kind string, id int64, fields map[string]any) {
	outputMu.Lock()
	defer outputMu.Unlock()
	if fields == nil {
		fields = map[string]any{}
	}
	fields["event"] = kind
	fields["account_id"] = id
	fields["at"] = time.Now().UTC().Format(time.RFC3339Nano)
	_ = json.NewEncoder(os.Stdout).Encode(fields)
}
func hash(s string) string { b := sha256.Sum256([]byte(s)); return hex.EncodeToString(b[:]) }

func tokenMeta(token string) map[string]any {
 out:=map[string]any{"token_sha256":hash(token)}
 parts:=strings.Split(token,".");if len(parts)<2{return out};raw,err:=base64.RawURLEncoding.DecodeString(parts[1]);if err!=nil{return out}
 var c map[string]any;if json.Unmarshal(raw,&c)!=nil{return out}
 for _,k:=range []string{"sid","user_id"}{if v,ok:=c[k].(string);ok{out[k+"_sha256"]=hash(v)}}
 for _,k:=range []string{"client_id","account_type"}{if v,ok:=c[k].(string);ok{out[k]=v}}
 return out
}
type jobKey struct{}
type countingTransport struct {
	base     adobe.Transport
	id       int64
	submits  atomic.Int64
	requests atomic.Int64
}

func (t *countingTransport) Do(ctx context.Context, r *adobe.Request) (*adobe.Response, error) {
	start := time.Now()
	u, _ := url.Parse(r.URL)
	stage := "poll"
	if strings.Contains(u.Path, "generate-async") {
		stage = "submit"
		t.submits.Add(1)
	}
	t.requests.Add(1)
	event("http_start", t.id, map[string]any{"stage": stage, "job": ctx.Value(jobKey{}), "host": u.Host, "token_sha256": hash(strings.TrimPrefix(r.Headers["authorization"], "Bearer ")), "arp_sha256": hash(r.Headers["x-arp-session-id"]), "user_agent": r.Headers["user-agent"]})
	response, err := t.base.Do(ctx, r)
	fields := map[string]any{"stage": stage, "job": ctx.Value(jobKey{}), "host": u.Host, "elapsed_ms": time.Since(start).Milliseconds()}
	if response != nil {
		fields["status"] = response.StatusCode
  if response.StatusCode>=200 && response.StatusCode<300 && stage=="submit" {fields["accepted"]=true}

		if response.StatusCode >= 400 {
			var d map[string]any
			if json.Unmarshal(response.Body, &d) == nil {
				for _, k := range []string{"error_code", "error", "message"} {
					if v, ok := d[k].(string); ok && len(v) < 250 && !strings.Contains(v, "http") && !strings.Contains(v, "eyJ") {
						fields[k] = v
					}
				}
			}
		}
	}
	if err != nil {
		fields["transport_error"] = fmt.Sprintf("%T", err)
	}
	event("http_finish", t.id, fields)
	return response, err
}

type gate struct {
	mu                      sync.Mutex
	next, max, active, peak int
	deadline, cooldown      time.Time
	interval                time.Duration
	stopped                 bool
	reason                  string
}

func (g *gate) take(ctx context.Context) (int, bool) {
	for {
		g.mu.Lock()
		if g.stopped || g.next >= g.max || !time.Now().Before(g.deadline) {
			g.mu.Unlock()
			return 0, false
		}
		wait := time.Until(g.cooldown)
		if wait <= 0 {
			g.next++
			if g.interval > 0 {
				g.cooldown = time.Now().Add(g.interval)
			}
			g.active++
			if g.active > g.peak {
				g.peak = g.active
			}
			n := g.next
			g.mu.Unlock()
			return n, true
		}
		g.mu.Unlock()
		select {
		case <-ctx.Done():
			return 0, false
		case <-time.After(wait):
		}
	}
}
func (g *gate) finish(code int) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.active--
	if code == http.StatusUnauthorized {
		g.stopped = true
		g.reason = "upstream_auth_401"
	}
	if code == 429 {
		until := time.Now().Add(15 * time.Second)
		if until.After(g.cooldown) {
			g.cooldown = until
		}
	}
}
func (g *gate) stop(reason string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.stopped = true
	if g.reason == "" {
		g.reason = reason
	}
}
func (g *gate) isStopped() bool { g.mu.Lock(); defer g.mu.Unlock(); return g.stopped }
func guard(cfg Config, a Account) bool {
	for _, n := range []string{"abort-all", fmt.Sprintf("abort-%d", a.ID)} {
		if _, err := os.Stat(filepath.Join(cfg.GuardDir, n)); err == nil {
			return false
		}
	}
	return true
}

func validate(a Account, t adobe.Transport, label, token string) bool {
	if token == "" {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	r, e := t.Do(ctx, &adobe.Request{Method: "POST", URL: "https://ims-na1.adobelogin.com/ims/validate_token/v1?jslVersion=v2-v0.54.0-3-g58cfcb7", Headers: map[string]string{"content-type": "application/x-www-form-urlencoded;charset=UTF-8", "origin": "https://firefly.adobe.com", "referer": "https://firefly.adobe.com/", "user-agent": adobe.DefaultIdentity.UserAgent}, Body: []byte(url.Values{"type": {"access_token"}, "client_id": {"clio-playground-web"}, "token": {token}}.Encode()), Timeout: 15 * time.Second})
	f := tokenMeta(token)
 f["token_source"]=label
	valid := false
	if e != nil {
		f["transport_error"] = fmt.Sprintf("%T", e)
	}
	if r != nil {
		f["http_status"] = r.StatusCode
		var d map[string]any
		if json.Unmarshal(r.Body, &d) == nil {
			for _, k := range []string{"valid", "reason", "expires_at", "expires_at_str"} {
				if v, ok := d[k]; ok {
					f[k] = v
				}
			}
			valid = d["valid"] == true
		}
	}
	event("validate", a.ID, f)
	return valid
}
func egress(a Account, t adobe.Transport) string {
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()
	r, e := t.Do(ctx, &adobe.Request{Method: "GET", URL: "https://api.ipify.org?format=json", Timeout: 12 * time.Second})
	if e != nil || r == nil || r.StatusCode != 200 {
		event("egress", a.ID, map[string]any{"verified": false})
		return ""
	}
	var d struct {
		IP string `json:"ip"`
	}
	if json.Unmarshal(r.Body, &d) != nil || d.IP == "" {
		event("egress", a.ID, map[string]any{"verified": false})
		return ""
	}
	h := hash(d.IP)
	event("egress", a.ID, map[string]any{"verified": true, "ip_sha256": h})
	return h
}

func runAccount(cfg Config, a Account, start time.Time) {
	end := start.Add(time.Duration(cfg.DurationSec) * time.Second)
	g := &gate{max: cfg.MaxJobs, deadline: end, interval: time.Duration(a.IntervalSec) * time.Second}
	ctx := context.Background()
	base := adobe.NewImpersonateTransport(adobe.DefaultIdentity, a.Proxy)
	tr := &countingTransport{base: base, id: a.ID}
	client := adobe.NewClient(adobe.ClientConfig{Transport: tr, ProxyURL: a.Proxy})
	validator := adobe.NewImpersonateTransport(adobe.DefaultIdentity, a.Proxy)
	if !validate(a, validator, "sub2", a.Token) {
		g.stop("baseline_not_verified")
	}
	validate(a, validator, "okad_original", a.OriginalToken)
	initialIP := egress(a, validator)
	if initialIP == "" {
		g.stop("egress_unverified")
	}
	conf, err := adobe.ResolveImage(adobe.ImageRequest{ModelID: cfg.Model, Size: "1024x1024"})
	if err != nil {
		event("fatal", a.ID, map[string]any{"reason": "model_resolution_failed"})
		return
	}
	if d := time.Until(start); d > 0 {
		time.Sleep(d)
	}
	event("session", a.ID, tokenMeta(a.Token))
 event("resolved_model",a.ID,map[string]any{"model":cfg.Model,"upstream_model_id":conf.UpstreamModelID,"upstream_model_version":conf.UpstreamModelVersion,"size":conf.SizePixels})
 event("account_start", a.ID, map[string]any{"round": cfg.Round, "concurrency": a.Concurrency, "arm": a.Arm, "interval_sec": a.IntervalSec, "proxy_id": a.ProxyID, "token_sha256": hash(a.Token), "deadline": end.UTC().Format(time.RFC3339Nano), "max_jobs": cfg.MaxJobs})
	var wg sync.WaitGroup
	var successes atomic.Int64
	for w := 0; w < a.Concurrency; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				if !guard(cfg, a) {
					g.stop("external_guard_changed")
					return
				}
				job, ok := g.take(ctx)
				if !ok {
					return
				}
				began := time.Now()
				event("job_start", a.ID, map[string]any{"job": job, "round": cfg.Round})
				seed := 12345 + job
				result, err := client.GenerateImage(context.WithValue(ctx, jobKey{}, job), adobe.GenerateImageInput{Token: a.Token, ARPSessionID: a.ARPSessionID, Timeout: 180 * time.Second, Options: adobe.ImagePayloadOptions{Prompt: "A simple red apple on a plain white background.", AspectRatio: conf.AspectRatio, OutputResolution: conf.OutputResolution, UpstreamModelID: conf.UpstreamModelID, UpstreamModelVersion: conf.UpstreamModelVersion, PayloadKind: conf.PayloadKind, SizePixels: conf.SizePixels, Seed: &seed}})
				code := 0
				f := map[string]any{"job": job, "elapsed_ms": time.Since(began).Milliseconds(), "success": err == nil && result != nil && len(result.Bytes) > 0}
				if err != nil {
					var request *adobe.RequestError
					if errors.As(err, &request) {
						code = request.StatusCode
						f["status"] = code
						f["error_type"] = request.ErrorType
					}
					var op *adobe.OperationError
					if errors.As(err, &op) {
						f["stage"] = op.Stage
						f["attempts"] = op.Attempts
					}
					f["error_class"] = fmt.Sprintf("%T", err)
				}
				if result != nil {
					f["image_bytes"] = len(result.Bytes)
					f["timings"] = result.Timings
					result.Bytes = nil
					if err == nil {
						successes.Add(1)
					}
				}
				g.finish(code)
				event("job_finish", a.ID, f)
			}
		}()
	}
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()
	complete := false
	for time.Now().Before(end) {
		select {
		case <-done:
			complete = true
			done = nil
		case <-ticker.C:
			if !guard(cfg, a) {
				g.stop("external_guard_changed")
			}
			if currentIP := egress(a, validator); currentIP == "" || currentIP != initialIP {
				g.stop("egress_changed_or_unverified")
			}
			valid := validate(a, validator, "sub2", a.Token)
			if !valid {
				g.stop("validation_no_longer_valid")
				validate(a, validator, "okad_original", a.OriginalToken)
			}
			g.mu.Lock()
			event("progress", a.ID, map[string]any{"jobs_started": g.next, "active": g.active, "peak_active": g.peak, "successes": successes.Load(), "submit_attempts": tr.submits.Load(), "stopped": g.stopped, "reason": g.reason})
			g.mu.Unlock()
		case <-time.After(time.Until(end)):
		}
		if g.isStopped() {
			break
		}
	}
	g.stop("window_ended")
	if !complete {
		wg.Wait()
	}
	validate(a, validator, "sub2_final", a.Token)
	validate(a, validator, "okad_original_final", a.OriginalToken)
	g.mu.Lock()
	event("account_done", a.ID, map[string]any{"round": cfg.Round, "concurrency": a.Concurrency, "jobs_started": g.next, "peak_active": g.peak, "successes": successes.Load(), "submit_attempts": tr.submits.Load(), "http_requests": tr.requests.Load(), "reason": g.reason})
	g.mu.Unlock()
}
func main() {
	var cfg Config
	if json.NewDecoder(os.Stdin).Decode(&cfg) != nil {
		os.Exit(2)
	}
	if len(cfg.Accounts) != 1 || cfg.MaxJobs != 6 || cfg.DurationSec != 1200 || cfg.Model != "gpt-image-2.5-sunburst" {
		os.Exit(3)
	}
	seen := map[int64]bool{}
	for _, a := range cfg.Accounts {
		if (a.ID != 1842 && a.ID != 1833 && a.ID != 1807 && a.ID != 1817) || seen[a.ID] || (a.Concurrency != 1 && a.Concurrency != 3) || a.Token == "" || a.Proxy == "" {
			os.Exit(4)
		}
		if a.IntervalSec != 0 && a.IntervalSec != 180 {
			os.Exit(6)
		}
		if a.IntervalSec == 180 && a.Concurrency != 1 {
			os.Exit(7)
		}
		seen[a.ID] = true
	}
	event("round_start", 0, map[string]any{"round": cfg.Round, "model": cfg.Model, "duration_sec": cfg.DurationSec, "max_jobs_per_account": cfg.MaxJobs})
	start, parseErr := time.Parse(time.RFC3339, cfg.StartAt)
	if parseErr != nil || time.Until(start) < 15*time.Second || time.Until(start) > 120*time.Second {
		os.Exit(5)
	}
	var wg sync.WaitGroup
	for _, a := range cfg.Accounts {
		wg.Add(1)
		go func(a Account) { defer wg.Done(); runAccount(cfg, a, start) }(a)
	}
	wg.Wait()
	event("round_done", 0, map[string]any{"round": cfg.Round})
}
