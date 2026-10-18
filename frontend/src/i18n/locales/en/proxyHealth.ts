export default {
  expand: 'Show details', collapse: 'Hide details', sourceSummary: '{total} subscriptions, {abnormal} need attention',
  title: 'Subscription health and group capacity', updated: 'Collected', manage: 'Manage subscription URLs',
  source: 'Subscription {index}', remaining: '{remaining} remaining / {total}', expires: 'Expires',
  trafficUnknown: 'Traffic remaining unknown (missing or stale provider data)',
  nodes: '{healthy} reachable · {failed} failed · {unknown} unconfirmed / {total} nodes',
  excluded: 'New assignments from this subscription are blocked; qualified alternatives are selected automatically.',
  fetchFailed: 'Subscription URL refresh failed: {reason}. Existing nodes are evaluated by connectivity.',
  observed: 'Traffic observed', noSources: 'No Clash subscriptions configured. Manual proxies still follow allocation policy.',
  readFailed: 'Unable to update health. Previously displayed data may be stale; please refresh.',
  capacity: '{count} groups have fewer than 2 available candidates',
  groupCapacity: '{available} available candidates · {enabled} enabled · {waiting} waiting for a proxy',
  paused: 'No enabled accounts; manual settings are preserved',
  evidenceHint: 'Connectivity does not prove account authorization, credits, or generation success. Group counts use recorded state; model limits and concurrency further filter candidates. Traffic refreshes about every 5–6 minutes with failure backoff, without generation or login.',
  fetchReasons: { invalid_url: 'Invalid URL', connection_failed: 'Connection failed or timed out', read_failed: 'Download failed', invalid_subscription: 'Invalid subscription response', unknown: 'Unconfirmed cause' },
  states: { checking: 'Awaiting checks', healthy: 'Reachable', degraded: 'Some nodes failed', unavailable: 'All nodes unreachable', fetch_failed: 'Subscription URL refresh failed', stale: 'Stale observations', exhausted: 'Subscription traffic exhausted', expired: 'Subscription expired' },
  watermarks: { checking: 'Pending', healthy: 'Healthy', degraded: 'Partial', unavailable: 'Offline', fetch_failed: 'Error', stale: 'Unverified', exhausted: 'Offline', expired: 'Offline' }
}
