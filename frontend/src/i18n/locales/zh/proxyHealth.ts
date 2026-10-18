export default {
  expand: '展开状态', collapse: '收起状态', sourceSummary: '共 {total} 条订阅，{abnormal} 条需要关注',
  title: '订阅健康与分组容量', updated: '采集于', manage: '管理订阅链接',
  source: '订阅 {index}', remaining: '剩余 {remaining} / {total}', expires: '到期时间',
  trafficUnknown: '剩余流量未知（未提供有效信息或数据已过期）',
  nodes: '可达 {healthy} · 故障 {failed} · 待确认 {unknown} / 共 {total} 节点',
  excluded: '已排除本订阅的新分配，后台自动寻找其他合格出口。',
  fetchFailed: '订阅链接更新失败：{reason}。已有节点按实际连通性处理。',
  observed: '流量记录时间', noSources: '未配置 Clash 订阅；手动代理仍按自动分配策略处理。',
  readFailed: '状态读取失败；已有内容可能过期，请刷新。',
  capacity: '{count} 个分组可用候选不足 2 个',
  groupCapacity: '可用候选 {available} · 已启用 {enabled} · 等待代理 {waiting}',
  paused: '当前没有启用账号，保留人工状态',
  evidenceHint: '连接可达不代表账号鉴权、额度或出图正常；分组候选按已有状态统计，模型限制和并发仍会进一步筛选。余额约每 5–6 分钟更新，失败退避，不主动出图或登录。',
  fetchReasons: { invalid_url: '链接格式无效', connection_failed: '连接失败或超时', read_failed: '下载失败', invalid_subscription: '返回内容不是有效订阅', unknown: '原因未确认' },
  states: { checking: '等待检测', healthy: '连接可达', degraded: '部分节点异常', unavailable: '节点全部不可达', fetch_failed: '订阅链接更新失败', stale: '记录已过期', exhausted: '订阅流量耗尽', expired: '订阅已到期' },
  watermarks: { checking: '待检测', healthy: '健康', degraded: '部分可用', unavailable: '不可用', fetch_failed: '更新失败', stale: '待确认', exhausted: '不可用', expired: '不可用' }
}
