package adobe
import("os";"path/filepath";"strings";"testing";"encoding/json")
func TestDiagnosticFullChainRedaction(t *testing.T){p:=filepath.Join(t.TempDir(),"events");t.Setenv("ADOBE_DIAG_FILE",p);done:=diagHTTP(&Request{Method:"POST",URL:"https://example.test/ims/token/v3?secret=QUERY_SECRET",Headers:map[string]string{"Authorization":"Bearer TOKEN_SECRET","Cookie":"COOKIE_SECRET"},Body:[]byte(`{"prompt":"PROMPT_SECRET","model":"test"}`)},"http://USER_SECRET:PASSWORD_SECRET@proxy");done(&Response{StatusCode:401,Body:[]byte(`{"error":"invalid_credentials"}`),SetCookies:[]string{"SESSION_SECRET"}},nil);b,e:=os.ReadFile(p);if e!=nil{t.Fatal(e)};for _,s:=range []string{"QUERY_SECRET","TOKEN_SECRET","COOKIE_SECRET","PROMPT_SECRET","USER_SECRET","PASSWORD_SECRET","SESSION_SECRET"}{if strings.Contains(string(b),s){t.Fatal("secret leaked",s)}};for _,l:=range strings.Split(strings.TrimSpace(string(b)),"\n"){var d map[string]any;if json.Unmarshal([]byte(l),&d)!=nil{t.Fatal("invalid JSON")}}}

func TestDiagnosticSubmitControlRequiresDiagnosticMode(t *testing.T) {
 t.Setenv("ADOBE_DIAG_FILE", "");t.Setenv("ADOBE_DIAG_SUBMIT_ONCE", "1")
 if got:=DiagSubmitAttempts(3);got!=3 {t.Fatalf("production default changed: %d",got)}
 t.Setenv("ADOBE_DIAG_FILE", filepath.Join(t.TempDir(),"events"))
 if got:=DiagSubmitAttempts(3);got!=1 {t.Fatalf("single submission control inactive: %d",got)}
 t.Setenv("ADOBE_DIAG_SUBMIT_ONCE", "0")
 if got:=DiagSubmitAttempts(3);got!=3 {t.Fatalf("baseline changed: %d",got)}
 t.Setenv("ADOBE_DIAG_ACCOUNT_IDS", " 1738, 1700 ")
 if !DiagAccountAllowed(1738)||!DiagAccountAllowed(1700)||DiagAccountAllowed(17) {t.Fatal("account scope guard failed")}
}
