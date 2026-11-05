"""Send one explicitly requested completion email after all release gates pass."""
import datetime
from email.message import EmailMessage
from email.utils import formataddr, format_datetime, make_msgid
import importlib.util
import json
import os
from pathlib import Path
import smtplib
import ssl
import sys

ROOT = Path('/opt/adobe-relay/releases/adobe-auto-medium-20261105')
RECIPIENT = 'tjt740@gmail.com'


def main():
    os.umask(0o077)
    receipt = ROOT / 'release-notification.json'
    if sys.argv[1:] == ['status']:
        print(receipt.read_text() if receipt.exists() else json.dumps({'state':'not_started'}))
        return
    assert sys.argv[1:] == ['send']
    sha = (ROOT / 'expected-sha').read_text().strip()
    done = json.loads((ROOT / 'release-completion.json').read_text())
    core = json.loads((ROOT / 'final-verification.json').read_text())
    live = json.loads((ROOT / 'after-live-test.json').read_text())
    assert done['application_sha'] == core['application_sha'] == live['application_sha'] == sha
    assert live['success'] and live['requested_quality'] == 'auto' and live['response_quality'] == 'medium'
    assert all(done['preserved'].values()) and done['rollback_guard']['passed']
    spec = importlib.util.spec_from_file_location('audit', ROOT / 'audit.py')
    a = importlib.util.module_from_spec(spec); spec.loader.exec_module(a)
    app = a.inspect(a.APP)
    assert app['Config']['Image'] == 'adobe-relay:' + sha and app['State']['Health']['Status'] == 'healthy'
    settings_before = a.sql("SELECT md5(value) FROM settings WHERE key='ops_adobe_alert_mail_private'")
    mail = json.loads(a.sql("SELECT value FROM settings WHERE key='ops_adobe_alert_mail_private'"))
    choices = [mail, *mail.get('additional', [])]
    selected = [x for x in choices if x.get('recipient', '').lower() == RECIPIENT]
    assert len(selected) == 1, 'Requested recipient must have one existing configured transport'
    config = {k.lower():v for k,v in selected[0]['smtp'].items()}
    assert config['host'] and config['username'] and config['password']
    port = int(config['port']); assert 1 <= port <= 65535
    cutover = json.loads((ROOT / 'rolling-route-canonical.json').read_text())['at']
    beijing = datetime.datetime.fromisoformat(cutover).astimezone(datetime.timezone(datetime.timedelta(hours=8)))
    message = EmailMessage()
    message['Subject'] = '[发布完成] Adobe 图片 auto 默认 medium（2026-11-05）'
    message['From'] = formataddr((config.get('fromname') or 'Sub2 发布通知', config['from']))
    message['To'] = RECIPIENT
    message['Date'] = format_datetime(datetime.datetime.now(datetime.timezone.utc))
    message['Message-ID'] = make_msgid(domain='creative.cheap')
    message.set_content(f'''Adobe 图片质量调整已发布到主服务器 https://creative.cheap 。

完成情况：
- auto、未填写或空质量默认提交 medium，并返回 quality=medium。
- 显式 low、高质量档位及模型上限保留；账号检测继续使用 low。
- 完整后端检查、565 项前端检查、镜像及产物门禁通过。
- 发布前 medium 与发布后 auto 各进行一次真实出图，均成功返回 1024×1024 PNG；发布后 auto 返回 medium，耗时 {live['duration_seconds']} 秒。
- 双站健康、实时流、清理后告警心跳及两分钟被动观察通过，0 OOM／重启；临时实例、NAT 和租约已清理。
- 现有数据、人工设置、资源、SMTP 和告警配置保持。

正式实例接回时间：北京时间 {beijing:%Y-%m-%d %H:%M:%S}
应用版本：{sha[:12]}
变更记录：https://github.com/tjt740/adobe-relay/pull/134

真实请求为小样本验证，不代表全池或峰值吞吐保证。本邮件为你要求的单次发布完成通知，不修改自动告警规则。
''')
    out = {'state':'sending','at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'recipient':RECIPIENT,'application_sha':sha,'message_id':str(message['Message-ID']),'attempts':1,'automatic_retry':False}
    with os.fdopen(os.open(receipt, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600), 'w') as f:
        json.dump(out, f, ensure_ascii=False, indent=2)
    try:
        context = ssl.create_default_context()
        client = smtplib.SMTP_SSL(config['host'], port, timeout=30, context=context) if port == 465 else smtplib.SMTP(config['host'], port, timeout=30)
        with client:
            client.ehlo()
            if port != 465:
                assert client.has_extn('starttls'), 'Configured SMTP must support TLS'
                client.starttls(context=context); client.ehlo()
            client.login(config['username'], config['password'])
            refused = client.send_message(message, from_addr=config['from'], to_addrs=[RECIPIENT])
            assert not refused
            out['state'] = 'smtp_accepted'
            out['accepted_at'] = datetime.datetime.now(datetime.timezone.utc).isoformat()
    except Exception as error:
        out['state'] = 'requires_manual_review'
        out['error_type'] = type(error).__name__
        raise
    finally:
        receipt.write_text(json.dumps(out, ensure_ascii=False, indent=2))
        print(json.dumps(out), flush=True)
    assert settings_before == a.sql("SELECT md5(value) FROM settings WHERE key='ops_adobe_alert_mail_private'")
    done['mail_sent'] = True
    done['notification'] = {k:out[k] for k in ['state','recipient','message_id','accepted_at','attempts']}
    (ROOT / 'release-completion.json').write_text(json.dumps(done, ensure_ascii=False, indent=2))


if __name__ == '__main__':
    main()
