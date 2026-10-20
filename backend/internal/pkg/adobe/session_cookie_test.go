//go:build unit

package adobe

import (
	"context"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestIMSRefreshPreservesRotatedCookiesForNextExchange(t *testing.T) {
	api := &fakeTransport{handler: func(req *Request, n int) (*Response, error) {
		if n == 0 {
			r := jsonResponse(t, 200, map[string]any{"access_token": "token-a"}, nil)
			r.SetCookies = []string{"ims_sid=new; Domain=.services.adobe.com; Path=/; Secure; HttpOnly", "aux_sid=aux-new; Domain=.adobe.com; Path=/; Secure", "remove=; Path=/; Max-Age=0", "ignore=bad; Domain=unrelated.example; Path=/", "otherpath=bad; Path=/other", "keep2=present; Path=/; Max-Age=3600; Expires=Sat, 01 Jan 2000 00:00:00 GMT"}
			return r, nil
		}
		require.Equal(t, "ims_sid=new; aux_sid=aux-new; keep=yes; keep2=present", req.Headers["cookie"])
		return jsonResponse(t, 200, map[string]any{"access_token": "token-b"}, nil), nil
	}}
	client := testClient(api, nil)
	a, err := client.RefreshAccessTokenFromCookie(context.Background(), "ims_sid=old; aux_sid=aux-old; keep=yes; remove=old", RefreshOptions{SkipAccountFetch: true})
	require.NoError(t, err)
	require.True(t, a.CookieUpdated)
	b, err := client.RefreshAccessTokenFromCookie(context.Background(), a.Cookie, RefreshOptions{SkipAccountFetch: true})
	require.NoError(t, err)
	require.Equal(t, "token-b", b.AccessToken)
	require.Equal(t, a.Cookie, b.Cookie)
	require.False(t, b.CookieUpdated)
}

func TestExplicitSessionDoesNotAppendCachedIMSIDs(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			http.SetCookie(w, &http.Cookie{Name: "ims_sid", Value: "cached", Path: "/"})
		} else {
			require.Equal(t, "ims_sid=authoritative", r.Header.Get("Cookie"))
			require.Equal(t, 1, strings.Count(r.Header.Get("Cookie"), "ims_sid="))
			w.Header().Add("Set-Cookie", "ims_sid=next; Path=/")
			w.Header().Add("Set-Cookie", "aux_sid=next-aux; Path=/")
		}
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()
	tr := NewImpersonateTransport(DefaultIdentity, "")
	_, err := tr.Do(context.Background(), &Request{Method: http.MethodGet, URL: server.URL})
	require.NoError(t, err)
	response, err := tr.Do(context.Background(), &Request{Method: http.MethodGet, URL: server.URL, Headers: map[string]string{"cookie": "ims_sid=authoritative"}})
	require.NoError(t, err)
	require.Len(t, response.SetCookies, 2)
}
