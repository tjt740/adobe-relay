package adobe

// Temporary experiment instrumentation. Only hashes and allowlisted metadata are emitted.
import (
 "crypto/sha256"
 "encoding/hex"
 "encoding/base64"
 "encoding/json"
 "fmt"
 "net/url"
 "os"
 "strings"
 "sync"
 "sync/atomic"
 "time"
)
var diagMu sync.Mutex
var diagSeq atomic.Uint64
func DiagHash(s string) string { if s=="" {return ""};h:=sha256.Sum256([]byte(s));return hex.EncodeToString(h[:]) }
func DiagToken(t string) map[string]any {
 m:=map[string]any{"sha256":DiagHash(t)};p:=strings.Split(t,".")
 if len(p)>1 { b,e:=base64.RawURLEncoding.DecodeString(p[1]);if e==nil {var c map[string]any;if json.Unmarshal(b,&c)==nil {for _,k:=range []string{"sid","user_id","sub"} {if v,ok:=c[k].(string);ok {m[k+"_sha256"]=DiagHash(v)}};for _,k:=range []string{"iat","exp","client_id"} {m[k]=c[k]}}} };return m
}
func DiagEvent(event string,m map[string]any) {
 path:=os.Getenv("ADOBE_DIAG_FILE");if path=="" {return};if m==nil {m=map[string]any{}}
 m["event"]=event;m["at"]=time.Now().UTC().Format(time.RFC3339Nano)
 diagMu.Lock();defer diagMu.Unlock();m["seq"]=diagSeq.Add(1);b,e:=json.Marshal(m);if e!=nil{return};f,e:=os.OpenFile(path,os.O_WRONLY|os.O_CREATE|os.O_APPEND,0600);if e!=nil{return};defer f.Close();f.Write(append(b,'\n'))
}
func diagHTTP(req *Request,proxy string) func(*Response,error) {
 if os.Getenv("ADOBE_DIAG_FILE")=="" {return func(*Response,error){}}
 id:=diagSeq.Add(1);u,_:=url.Parse(req.URL);path:=u.Path
 // Job and asset identifiers are hashed, query strings never leave this process.
 stage:="other";for _,s:=range []string{"check_token","token/v","profile","generate-async","status","result","job","credits"} {if strings.Contains(path,s){stage=s;break}}
 m:=map[string]any{"http_id":id,"host":u.Hostname(),"stage":stage,"path_sha256":DiagHash(path),"method":req.Method,"proxy_sha256":DiagHash(proxy)}
 for k,v:=range req.Headers {switch strings.ToLower(k) {case "authorization":m["token"]=DiagToken(strings.TrimPrefix(v,"Bearer "));case "cookie":m["cookie_sha256"]=DiagHash(v);case "x-arp-session-id":m["arp_sha256"]=DiagHash(v)}}
 var payload any
 if json.Unmarshal(req.Body,&payload)==nil {m["payload"]=diagFields(payload)}
 DiagEvent("http_start",m)
 return func(r *Response,err error) {n:=map[string]any{"http_id":id,"stage":stage,"host":u.Hostname(),"proxy_sha256":DiagHash(proxy),"error_type":fmt.Sprintf("%T",err)};if r!=nil {n["status"]=r.StatusCode;n["bytes"]=len(r.Body);n["set_cookie_hashes"]=[]string{};hs:=[]string{};for _,s:=range r.SetCookies {hs=append(hs,DiagHash(s))};n["set_cookie_hashes"]=hs;var d map[string]any;if json.Unmarshal(r.Body,&d)==nil {n["fields"]=diagFields(d);if t,ok:=d["access_token"].(string);ok {n["issued_token"]=DiagToken(t)}};s:=string(r.Body);for _,code:=range []string{"invalid_credentials","ErrInvalidOauthToken","invalid_token","invalid_grant"} {if strings.Contains(s,code){n[code]=true}}};DiagEvent("http_finish",n)}
}
func diagFields(v any) map[string]any {
 out:=map[string]any{};var visit func(any,string);visit=func(v any,p string) {switch x:=v.(type) {case map[string]any:for k,w:=range x {switch strings.ToLower(k) {case "model","modelid","modelversion","modelspecifier","width","height","detaillevel","numvariations","aspectratio","outputresolution","status","state","code","error","expires_in":switch z:=w.(type){case string:if len(z)<100 {out[p+k]=z};case float64,bool:out[p+k]=z}};switch w.(type){case map[string]any,[]any:visit(w,p+k+".")}};case []any:for i,w:=range x {if i<4 {visit(w,fmt.Sprintf("%s%d.",p,i))}}}};visit(v,"");return out
}
