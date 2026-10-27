from pathlib import Path
import json,hashlib,sys
p=Path(__file__).resolve().parent;b=Path(sys.argv[1]);changed=[]
def change(name,ops):
 f=b/name;s=f.read_text();before=hashlib.sha256(s.encode()).hexdigest()
 for old,new in ops:
  assert s.count(old)==1 or (name.endswith('transport.go') and old.startswith('\t timeout'.replace(' ','')) and s.count(old)==2),(name,old[:80],s.count(old));s=s.replace(old,new,1)
 f.write_text(s);changed.append({'file':name,'before':before,'after':hashlib.sha256(s.encode()).hexdigest()})
for src,dst in [('diagnostic.go','internal/pkg/adobe/diagnostic.go'),('account_diagnostic.go','internal/service/account_diagnostic.go'),('diagnostic_test.go','internal/pkg/adobe/diagnostic_test.go')]:
 f=b/dst;assert not f.exists();f.write_text((p/src).read_text());changed.append({'file':dst,'before':None,'after':hashlib.sha256(f.read_bytes()).hexdigest()})
change('internal/pkg/adobe/transport.go', [('func (t *impersonateTransport) Do(ctx context.Context, req *Request) (*Response, error) {','func (t *impersonateTransport) Do(ctx context.Context, req *Request) (diagResponse *Response, diagError error) {'), ('\ttimeout := req.Timeout\n\tif timeout <= 0 {','\tfinishDiagnostic := diagHTTP(req, t.proxyURL)\n\tdefer func(){finishDiagnostic(diagResponse,diagError)}()\n\ttimeout := req.Timeout\n\tif timeout <= 0 {')])
change('internal/service/adobe_gateway_token.go', [('func (p *AdobeTokenProvider) GetAccessToken(ctx context.Context, account *Account) (string, error) {','func (p *AdobeTokenProvider) GetAccessToken(ctx context.Context, account *Account) (diagToken string, diagError error) {\n\tDiagAccount("token_provider_input",account)\n\tdefer func(){if account!=nil && account.ID==1738 {adobe.DiagEvent("token_provider_output",map[string]any{"account_id":account.ID,"token":adobe.DiagToken(diagToken),"failed":diagError!=nil})}}()')])
change('internal/service/oauth_refresh_api.go', [('freshAccount, err := api.accountRepo.GetByID(ctx, account.ID)\n','freshAccount, err := api.accountRepo.GetByID(ctx, account.ID)\n\tDiagAccount("refresh_db_reread",freshAccount)\n'), ('newCredentials, refreshErr := executor.Refresh(ctx, freshAccount)','DiagAccount("refresh_upstream_start",freshAccount)\n\tnewCredentials, refreshErr := executor.Refresh(ctx, freshAccount)\n\tif newCredentials!=nil {diagnosticAccount:=*freshAccount;diagnosticAccount.Credentials=newCredentials;DiagAccount("refresh_upstream_result",&diagnosticAccount)}')])
change('internal/repository/account_repo_adobe_session.go', [ ('"fmt"','"fmt"\n "github.com/Wei-Shaw/sub2api/internal/pkg/adobe"'), ('n, err := result.RowsAffected()','n, err := result.RowsAffected()\n if id==1738 {adobe.DiagEvent("session_cas_result",map[string]any{"account_id":id,"applied":n>0,"failed":err!=nil,"before_cookie_sha256":adobe.DiagHash(before.Cookie),"before_runtime_sha256":adobe.DiagHash(before.SessionCookie),"before_proxy_id":before.ProxyID})}')])
change('internal/repository/scheduler_cache.go',[ ('\treturn &account, nil\n}', '\tservice.DiagAccount("scheduler_cache_decode", &account)\n\treturn &account, nil\n}')])
(p/'instrument-manifest.json').write_text(json.dumps(changed,indent=2))
