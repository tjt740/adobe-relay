package adobe

import (
	"golang.org/x/net/publicsuffix"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"time"
)

// A raw Cookie header has no domain/path attributes. Apply only response cookies
// valid at this IMS endpoint, then replace by name; combining host-only input
// cookies with domain cookies in a jar would send both old and new ims_sid.
func refreshedCookie(original, endpoint string, response *Response) string {
	values := response.SetCookies
	if len(values) == 0 && response.Header("set-cookie") != "" {
		values = []string{response.Header("set-cookie")}
	}
	if len(values) == 0 {
		return original
	}
	u, err := url.Parse(endpoint)
	if err != nil || u.Hostname() == "" {
		return original
	}
	incoming := (&http.Request{Header: http.Header{"Cookie": []string{original}}}).Cookies()
	order := []string{}
	current := map[string]*http.Cookie{}
	for _, c := range incoming {
		if _, ok := current[c.Name]; !ok {
			order = append(order, c.Name)
		}
		current[c.Name] = c
	}
	updates := (&http.Response{Header: http.Header{"Set-Cookie": values}}).Cookies()
	for _, c := range updates {
		// Validate scoping even for deletion cookies without letting expiry hide
		// whether a cookie would have applied to this endpoint.
		scoped := *c
		scoped.Expires = time.Time{}
		scoped.MaxAge = 0
		jar, _ := cookiejar.New(&cookiejar.Options{PublicSuffixList: publicsuffix.List})
		jar.SetCookies(u, []*http.Cookie{&scoped})
		if len(jar.Cookies(u)) == 0 {
			continue
		}
		if c.MaxAge < 0 || (c.MaxAge == 0 && !c.Expires.IsZero() && c.Expires.Before(time.Now())) {
			delete(current, c.Name)
			continue
		}
		if _, ok := current[c.Name]; !ok {
			order = append(order, c.Name)
		}
		current[c.Name] = &http.Cookie{Name: c.Name, Value: c.Value, Quoted: c.Quoted}
	}
	out := &http.Request{Header: make(http.Header)}
	written := map[string]bool{}
	for _, name := range order {
		if c, ok := current[name]; ok && !written[name] {
			out.AddCookie(c)
			written[name] = true
		}
	}
	return out.Header.Get("Cookie")
}
