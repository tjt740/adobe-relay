"""Require the complete current-group matrix and retain separate legacy results."""
MODELS = ['gpt-image-2', 'gpt-image-2.5-flare', 'gpt-image-2.5-sunburst',
          'gpt-image-2.6-star-swirl', 'gpt-image-2.6-aurora']


def validate(current, legacy, cleanup, sha, expected_group_id=33):
    assert current['application_sha'] == legacy['application_sha'] == sha
    assert current['group_id'] == expected_group_id and current['all_passed'] is True
    if expected_group_id != 33:
        assert expected_group_id > 0 and expected_group_id != 7 and current['selected_account_id'] == 2933
        assert cleanup['selected_account_id'] == 2933 and cleanup['temporary_group_id'] == expected_group_id
        assert cleanup['temporary_group_deleted'] is True and cleanup['account_original_groups_preserved'] is True
    expected = {m + '-' + mode for m in MODELS for mode in ['generate', 'edit']}
    expected.update(m + '-' + mode for m in MODELS[-2:] for mode in ['gif', 'reject-json', 'reject-multipart', 'reject-generation-reference'])
    rows = current['cases']
    assert len(rows) == 18 and {row['case'] for row in rows} == expected
    assert len(legacy['cases']) == 18, 'Retain every original diagnostic result'
    assert legacy['all_passed'] == all(row['success'] for row in legacy['cases'])
    for row in rows:
        assert row['application_sha'] == sha and row['success'] is True and row['outer_posts'] == 1
        assert row['api_key_id'] != 12
        if '-reject-' in row['case']:
            error = row['error']
            assert row['expected_rejection'] is True and row['status'] == 400
            assert error['code'] == 'gif_edit_unsupported' and error['param'] == 'image'
            assert error['stage'] == 'validation' and error['source'] == 'request'
        else:
            assert row['expected_rejection'] is False and row['status'] == 200
            image = row['image']
            assert image['width'] > 0 and image['height'] > 0
            assert image['format'] == ('jpeg' if row['case'].endswith('-edit') else 'gif' if row['case'].endswith('-gif') else 'png')
            if image['format'] == 'gif':
                frames = 4 if row['model'].endswith('star-swirl') else 16
                assert image['frame_count'] == frames
                assert image['delays_centiseconds'] == [20] * frames
    assert all(cleanup[k] is True for k in ['key_deleted', 'temporary_grant_removed', 'private_files_removed', 'other_allowed_groups_preserved'])
    assert cleanup['existing_customer_keys_changed'] is False
    return {'scope': 'selected live account 2933 through an isolated regular API group; five models, 18 distinct public endpoint cases' if expected_group_id != 33 else 'current Adobe business group 33, five models, 18 distinct public endpoint cases',
            'approved_scope_all_passed': True, 'legacy_group_7_all_passed': legacy['all_passed'],
            'legacy_group_7_failures': sum(not row['success'] for row in legacy['cases']),
            'legacy_results_retained': True, 'temporary_test_access_cleaned': True,
            'all_account_pool_verified': False}
