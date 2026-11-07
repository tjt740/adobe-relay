"""A blocked production probe must never become a successful billing proof."""
def validate_blocked(rows,sha):
 assert len(rows)==2 and {r['name'] for r in rows}=={'flare-xhigh','sunburst-auto-inherit'}
 assert {r['model'] for r in rows}=={'gpt-image-2.6-flare','gpt-image-2.6-sunburst'}
 for row in rows:
  assert row['application_sha']==sha and not row['success'] and row['http_status']==408 and row['outer_posts']==1
  assert row['error']['stage']=='submit' and row['error']['source']=='upstream' and row['error']['upstream_status']==408
  assert not row.get('images') and not row.get('usage')
 return {'all_passed':False,'attempted_cases':2,'successful_cases':0,'outer_posts':2,'upstream_statuses':[408],'live_billing_verified':False,'reason':'Both submitted test combinations were rejected by upstream; successful generation and billing validation remain pending.'}
