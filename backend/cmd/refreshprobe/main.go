package main

import (
 "context"
 "crypto/sha256"
 "encoding/base64"
 "encoding/hex"
 "encoding/json"
 "errors"
 "fmt"
 "net/url"
 "os"
 "path/filepath"
 "strings"
 "sync"
 "time"
 "github.com/Wei-Shaw/sub2api/internal/pkg/adobe"
)
type Account struct {ID int64 `json:"id"`; Token string `json:"token"`; Cookie string `json:"cookie"`; Proxy string `json:"proxy"`; ExpectedIP string `json:"expected_ip"`}
type Config struct {Accounts []Account `json:"accounts"`; GuardDir string `json:"guard_dir"`; Earliest string `json:"earliest"`; Deadline string `json:"deadline"`}
var mu sync.Mutex
func hash(s string)string {x:=sha256.Sum256([]byte(s));return hex.EncodeToString(x[:])}
func event(kind string,id int64,f map[string]any){mu.Lock();defer mu.Unlock();if f==nil{f=map[string]any{}};f["at"]=time.Now().UTC().Format(time.RFC3339Nano);f["event"]=kind;f["account_id"]=id;_ = json.NewEncoder(os.Stdout).Encode(f)}
func claims(t string)map[string]any {parts:=strings.Split(t,".");if len(parts)!=3{return nil};b,_:=base64.RawURLEncoding.DecodeString(parts[1]);var d map[string]any;_ =json.Unmarshal(b,&d);return d}
func meta(t string)map[string]any{d:=claims(t);return map[string]any{"token_hash":hash(t),"sid_hash":hash(fmt.Sprint(d["sid"])),"user_hash":hash(fmt.Sprint(d["user_id"])),"client_id":d["client_id"]}}
func valid(a Account,t adobe.Transport,label,token string)bool{
 ctx,cancel:=context.WithTimeout(context.Background(),15*time.Second);defer cancel()
 r,e:=t.Do(ctx,&adobe.Request{Method:"POST",URL:"https://ims-na1.adobelogin.com/ims/validate_token/v1?jslVersion=v2-v0.54.0-3-g58cfcb7",Headers:map[string]string{"content-type":"application/x-www-form-urlencoded;charset=UTF-8","origin":"https://firefly.adobe.com","referer":"https://firefly.adobe.com/","user-agent":adobe.DefaultIdentity.UserAgent},Body:[]byte(url.Values{"type":{"access_token"},"client_id":{"clio-playground-web"},"token":{token}}.Encode()),Timeout:15*time.Second})
 f:=meta(token);f["label"]=label;ok:=false
 if e!=nil{f["error_class"]=fmt.Sprintf("%T",e)}
 if r!=nil{f["status"]=r.StatusCode;var d map[string]any;if json.Unmarshal(r.Body,&d)==nil{for _,k:=range []string{"valid","reason","expires_at"}{if v,yes:=d[k];yes{f[k]=v}};ok=d["valid"]==true}}
 event("validate",a.ID,f);return ok
}
func egress(a Account,t adobe.Transport)bool{
 ctx,cancel:=context.WithTimeout(context.Background(),15*time.Second);defer cancel()
 r,e:=t.Do(ctx,&adobe.Request{Method:"GET",URL:"https://api.ipify.org?format=json",Timeout:15*time.Second});ok:=false;h:=""
 if e==nil&&r!=nil&&r.StatusCode==200{var d map[string]string;if json.Unmarshal(r.Body,&d)==nil&&d["ip"]!=""{h=hash(d["ip"]);ok=strings.HasPrefix(h,a.ExpectedIP)}}
 event("egress",a.ID,map[string]any{"stable":ok,"ip_hash":h});return ok
}
type result struct{Token,Cookie string;OK bool}
func refresh(a Account,cookie string,index int)result{
 event("refresh_start",a.ID,map[string]any{"index":index,"cookie_hash":hash(cookie)})
 c:=adobe.NewClient(adobe.ClientConfig{ProxyURL:a.Proxy});ctx,cancel:=context.WithTimeout(context.Background(),45*time.Second);defer cancel()
 r,e:=c.RefreshAccessTokenFromCookie(ctx,cookie,adobe.RefreshOptions{SkipAccountFetch:true})
 f:=map[string]any{"index":index,"success":e==nil&&r!=nil}
 if e!=nil{f["error_class"]=fmt.Sprintf("%T",e);var re *adobe.RequestError;if errors.As(e,&re){f["status"]=re.StatusCode;f["error_type"]=re.ErrorType}}
 out:=result{}
 if e==nil&&r!=nil{out=result{r.AccessToken,r.Cookie,true};f["token"]=meta(r.AccessToken);f["cookie_updated"]=r.CookieUpdated;f["cookie_hash"]=hash(r.Cookie)}
 event("refresh_finish",a.ID,f);return out
}
func run(a Account,deadline time.Time){
 t:=adobe.NewImpersonateTransport(adobe.DefaultIdentity,a.Proxy)
 if !egress(a,t)||!valid(a,t,"baseline_original",a.Token){event("skip",a.ID,map[string]any{"reason":"baseline_invalid"});return}
 tokens:=[]string{a.Token}
 switch a.ID{
 case 1739:event("idle_control",a.ID,nil)
 case 1741:
  cookie:=a.Cookie
  for i:=1;i<=2;i++{r:=refresh(a,cookie,i);if !r.OK{break};tokens=append(tokens,r.Token);cookie=r.Cookie;time.Sleep(2*time.Second)}
 case 1743:
  start:=make(chan struct{});ch:=make(chan result,2)
  for i:=1;i<=2;i++{go func(i int){<-start;ch<-refresh(a,a.Cookie,i)}(i)}
  close(start);for i:=0;i<2;i++{r:=<-ch;if r.OK{tokens=append(tokens,r.Token)}}
 }
 dedup:=map[string]bool{};unique:=[]string{}
 for _,s:=range tokens{if !dedup[s]{dedup[s]=true;unique=append(unique,s)}}
 for time.Now().Before(deadline){
  if !egress(a,t){event("stop",a.ID,map[string]any{"reason":"egress_changed_or_unverified"});return}
  for i,s:=range unique{valid(a,t,fmt.Sprintf("after_refresh_%d",i),s)}
  if time.Until(deadline)<60*time.Second{break};time.Sleep(60*time.Second)
 }
 event("account_done",a.ID,map[string]any{"observed_tokens":len(unique)})
}
func main(){
 var cfg Config;if json.NewDecoder(os.Stdin).Decode(&cfg)!=nil{os.Exit(2)}
 if len(cfg.Accounts)!=3{os.Exit(3)};seen:=map[int64]bool{}
 for _,a:=range cfg.Accounts{if (a.ID!=1739&&a.ID!=1741&&a.ID!=1743)||seen[a.ID]||a.Token==""||a.Cookie==""||len(a.ExpectedIP)!=24{os.Exit(4)};seen[a.ID]=true}
 earliest,e:=time.Parse(time.RFC3339,cfg.Earliest);deadline,e2:=time.Parse(time.RFC3339,cfg.Deadline)
 if e!=nil||e2!=nil||deadline.Sub(earliest)>11*time.Minute||time.Until(deadline)>25*time.Minute||time.Until(earliest)<time.Minute{os.Exit(5)}
 event("scheduled",0,map[string]any{"earliest":cfg.Earliest,"deadline":cfg.Deadline,"refresh_counts":map[int64]int{1739:0,1741:2,1743:2}})
 for time.Now().Before(earliest){time.Sleep(time.Second)}
 for time.Now().Before(deadline.Add(-5*time.Minute)){
  if _,e:=os.Stat(filepath.Join(cfg.GuardDir,"trial-finished.json"));e==nil{break};time.Sleep(2*time.Second)
 }
 if _,e:=os.Stat(filepath.Join(cfg.GuardDir,"trial-finished.json"));e!=nil{event("skip",0,map[string]any{"reason":"first_trial_not_finished"});return}
 event("phase_start",0,nil)
 var wg sync.WaitGroup;for _,a:=range cfg.Accounts{wg.Add(1);go func(a Account){defer wg.Done();run(a,deadline)}(a)};wg.Wait();event("phase_done",0,nil)
}
