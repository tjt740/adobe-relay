package service
import "github.com/Wei-Shaw/sub2api/internal/pkg/adobe"
func DiagAccount(event string,a *Account) {
 if a==nil || !adobe.DiagAccountAllowed(a.ID){return}
 adobe.DiagEvent(event,map[string]any{"account_id":a.ID,"status":a.Status,"schedulable":a.Schedulable,"proxy_id":a.ProxyID,"source_cookie_sha256":adobe.DiagHash(a.GetCredential("cookie")),"runtime_cookie_sha256":adobe.DiagHash(a.GetCredential("adobe_session_cookie")),"runtime_source_sha256":a.GetCredential("adobe_session_cookie_source_hash"),"token":adobe.DiagToken(a.GetCredential("access_token")),"token_version":a.Credentials["_token_version"]})
}
