# Adobe 图片未知用量的随机模拟回填

标识：`adobe-simulated-usage-20261015`。用户在真实线上 4K 出图仍未取得 token 用量后，明确要求随机补齐，并选择整份响应 `total_tokens` 在 1800～3000、输入和输出拆分总量，随后明确要求保留原字段结构、不增加 usage_source。

## 行为

- 只改 Adobe 原生图片成功响应的返回层：存在合法真实 usage 时原样保留，包括 0 和部分明细，不增加来源字段；没有完整有效 usage 时生成随机模拟数据。
- 总量为 1800～3000（含边界），输入与输出随机拆分；明细相加一致。有参考图时拆分输入图片/文本；无参考图则输入图片为 0。输出图片等于输出总量，输出文本为 0。范围按整份响应计算，不因 n 倍增；每次独立随机，允许重复。
- 随机值不是实测，也不是官方公式估算，不进入真实 token 或计费字段；原图片张数/分辨率计费、URL/Base64/存储/转码、metadata、451、安全审核、鉴权与调度保持。已有真实明细缺失时不混补随机值。中转及其他协议保持原语义。
- 同步更新客户 curl、Python、URL/Base64 返回示例、随机回填规则及历史调查说明；单凭数值无法确认来源，不能把模拟值用于真实 token 账单。

## 验证与发布

回归覆盖 1800/3000 两端、16 路并发随机范围及明细一致性、文生图/URL 与上传参考图、多图范围、真实 0/部分明细优先、返回层不修改实测对象及原 token 计费字段保持；连同现有 Adobe/图片/451 回归和客户示例语法检查。

仅发布主服务器 173.234.15.150，原实际应用为 `9ff382000550d0c9010df5c2d5b90c18c950310b`。保留原配置、数据、SMTP、告警、Okad 及其他服务；无迁移，无配置项修改。本轮测试不增加 Adobe 付费出图、登录或邮件。功能 PR #54、字段修正 PR #55 已合并；正式应用目标为 `5bfea0b97224f2a3dcdf976b2a6857f3ee176a10`，工作流 `38680111446`。包含来源字段的中间版本 `c24a859` 对应工作流 `38679841077` 已在构建阶段取消、未部署。正式流程已成功，主服务器于北京时间 2026-10-15 12:23:21 切换至上述实际应用。

审计目录 `/opt/adobe-relay/releases/adobe-simulated-usage-20261015/`，回退镜像 `adobe-relay:before-adobe-simulated-usage-20261015`。只读检查：

```sh
python3 /opt/adobe-relay/releases/adobe-simulated-usage-20261015/adobe-simulated-usage-release.py check-rollback
```

末尾改为 `rollback` 才执行应用代码回退，恢复本次之前真实 usage 或 null 的规则，保留当前业务数据和计费。脚本依赖同目录 `image-response-release.py`，会检查后续应用、配置、设置、其他容器及回退镜像；不得跳过检查或恢复旧数据库。


## 发布版本与本地验证

- 发布标签 `release/adobe-simulated-usage-20261015` 指向 `5bfea0b97224f2a3dcdf976b2a6857f3ee176a10`。前主干标签 `rollback/adobe-simulated-usage-20261015-before-main` 指向 `ef79bb4d799f282c73b11b898bb1ab7206794fd8`，回退镜像为发布前实际 `9ff382000550d0c9010df5c2d5b90c18c950310b`。
- Adobe/图片/审核相关 service、handler race 检查通过；移除来源字段后的 AdobeImage race 检查再次通过。4 项回滚保护检查及客户 JSON/Python/bash 示例语法检查通过。回归明确检查真实及模拟两种响应均不包含 usage_source，模拟数字不进入原计费字段。
- 本轮仅更新应用代码和客户文档，不新增数据库迁移或配置项。客户文档保留随机模拟数据的含义及不能仅凭数值确认真实用量的说明。


## 上线核验

- [正式工作流 38680111446](https://github.com/tjt740/adobe-relay/actions/runs/38680111446) 的后端、前端、镜像构建及主服务器发布成功。两站 HTTPS、实时状态流、报表策略正常，新应用启动后的告警心跳为 12:24:22。原 451 行为随同现有回归通过。
- 配置、环境文件（除镜像引用）、SMTP、告警规则、Okad 设置、其他容器身份及重启次数保持。无新增迁移，本轮核验未发邮件、登录或付费出图；原有业务请求和联动后台任务继续运行。
- 与实际切换备份比对，951 个账号 ID 全部保留，未新增或删除。4 个代理变化均有原自动分配事件；#381/#397/#401 于 12:24:10 从代理 739 解绑并转为等待分配，状态/调度变化符合原分配逻辑，后续后台分配继续运行，不恢复旧绑定。
- #356/#388/#389/#402 的 Token 字段更新对应原刷新任务日志；#362/#363 的 Cookie 更新对应原 Okad credentials/verify 接口 200 及 received_at。#836 只更新 Token 三字段、Cookie 保持，发生于已有图生图请求窗口，未取得独立的定时刷新日志，不将其归为定时任务。上述原分配、刷新和联动源码此次未修改。核验期间业务继续更新，不将正常变化当作部署覆盖。
- 12:24 报表仍有 474 个待验证账号；应用健康不代表所有 Adobe 账号恢复。完整证据包括 `before.json`、`after.json`、`runtime-verification.json`、`business-verification.json`、`proxy-audit.json`、`allocation-change-evidence.json`、`runtime-change-events.json`、`account-change-metadata.json`、`token-refresh-evidence.json`、`business-change-review.json`。
- 实际切换备份 `/opt/adobe-relay/backups/20261015T042312Z-5bfea0b97224f2a3dcdf976b2a6857f3ee176a10`。`check-rollback` 已通过，只读核对版本、配置和回退镜像，不修改当前数据。发布归档仅改文档、使用 `[skip ci]`，线上镜像仍以实际应用 SHA 为准。
