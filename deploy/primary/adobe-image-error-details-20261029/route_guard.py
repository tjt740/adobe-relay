"""Keep the excluded route's actual pre-release state and Adobe protections."""
def validate_subscription_exclusion(sources):
    # A retired node need not remain in the user's current subscription list.
    # The complete subscription contract is separately hash-checked by audit.py.
    matches = [source for source in sources if 1290 in source['proxy_ids']]
    assert all(1290 in source['blocked_proxy_ids'] for source in matches), 'Excluded node became usable in a current source'
    return {'source_ids': [source['id'] for source in matches], 'not_in_current_subscriptions': not matches}


def validate_excluded_route(review, current):
    assert review['proxy_id'] == 1290 and review['read_only']
    if review.get('backup_proxy_exists') is False:
        assert review['backup_status'] is None and review['backup_excluded'] is False
        assert current == {'excluded': False, 'status': None, 'bindings': 0, 'leases': 0}, 'Do not recreate a retired node or introduce bindings'
        return {'retired_proxy_absence_preserved': True, 'no_bindings_or_test_leases': True}
    assert review['backup_excluded'] is True, 'Cutover backup must already contain the exclusion'
    assert review['backup_status'] in ('active', 'inactive')
    expected = {'excluded': True, 'status': review['backup_status'], 'bindings': 0, 'leases': 0}
    assert current == expected, 'Excluded route or its cutover status changed: ' + repr(current)
    return {'permanent_exclusion_preserved': True, 'status_matches_cutover_backup': True, 'no_bindings_or_test_leases': True}
