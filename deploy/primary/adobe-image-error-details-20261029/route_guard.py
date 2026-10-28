"""Keep the excluded route's actual pre-release state and Adobe protections."""
def validate_excluded_route(review, current):
    assert review['proxy_id'] == 1290 and review['read_only']
    assert review['backup_excluded'] is True, 'Cutover backup must already contain the exclusion'
    assert review['backup_status'] in ('active', 'inactive')
    expected = {'excluded': True, 'status': review['backup_status'], 'bindings': 0, 'leases': 0}
    assert current == expected, 'Excluded route or its cutover status changed: ' + repr(current)
    return {'permanent_exclusion_preserved': True, 'status_matches_cutover_backup': True, 'no_bindings_or_test_leases': True}
