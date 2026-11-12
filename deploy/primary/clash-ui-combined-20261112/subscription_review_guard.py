
def validate_addition(before,current):
 old_urls=before.get('url','').split();new_urls=current.get('url','').split()
 assert len(old_urls)==8 and len(new_urls)==9 and new_urls[:8]==old_urls and len(set(new_urls))==9
 old=before['nodes'];new=current['nodes'];assert len(old)==623 and len(new)==666
 def stable(n):return {k:v for k,v in n.items() if k not in ['source_id','original_name']}
 indexed={n['proxy_id']:n for n in new};assert len(indexed)==666
 assert all(n['proxy_id'] in indexed and stable(n)==stable(indexed[n['proxy_id']]) for n in old),'Existing node transport or state changed'
 assert current['revision']==before['revision']+1
 assert current.get('priorities')==before.get('priorities'),'Priorities changed beyond the confirmed addition'
 assert all((current.get('notes') or {}).get(k)==v for k,v in (before.get('notes') or {}).items())
 assert set(k for k in set(before)|set(current) if before.get(k)!=current.get(k)) <= {'nodes','notes','revision','settings_revision','sources','updated_at','url'}
