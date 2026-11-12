"""Retain the independently authorized Okad releases before this handoff starts.

Never restore configuration or data. Preserve the original snapshot and receipts.
"""
import copy

OKAD = 'adobe-team-cookie-app-1'
APP = 'adobe-relay-app-1'
COMPOSE = '/opt/adobe-team-cookie/compose.yml'
RELEASES = ['import-sub2-groups-20261112', 'import-sub2-groups-transport-20261112']


def reviewed_baseline(before, current, receipts):
    assert [r['manifest']['release'] for r in receipts] == RELEASES
    previous = before['containers'][OKAD]
    compose = before['config_hashes'][COMPOSE]
    for receipt in receipts:
        manifest, prior, after = (receipt[k] for k in ('manifest', 'before', 'after'))
        assert manifest['container'] == prior['container'] == previous['id']
        assert manifest['source_image_id'] == prior['source_image_id'] == previous['image_id']
        assert prior['config_hashes'][COMPOSE] == compose
        assert {k:v for k,v in prior['config_hashes'].items() if k != COMPOSE} == {k:v for k,v in after['config_hashes'].items() if k != COMPOSE}
        assert after['image'] == 'adobe-team-cookie:' + manifest['release']
        assert prior['database'] == after['database'] and not after['changed_setting_keys']
        assert after['verification']['api'] == after['verification']['database'] == 'ok'
        assert after['other_containers']['/'+APP] == {'id':before['containers'][APP]['id'], 'image':before['containers'][APP]['image_id']}
        expected = dict(prior['source_hashes'])
        for name, change in manifest['files'].items():
            assert prior['source_hashes'].get(name) == change['before']
            expected[name] = change['after']
        assert after['source_hashes'] == expected
        previous = {'id':after['container'], 'image_id':after['image_id'], 'image':after['image']}
        compose = after['config_hashes'][COMPOSE]
    actual = current['containers'][OKAD]
    assert all(actual[k] == v for k,v in previous.items())
    assert actual['health'] == 'healthy' and actual['restarts'] == 0
    assert actual['resources'] == before['containers'][OKAD]['resources']
    assert actual['go_env'] == before['containers'][OKAD]['go_env']
    assert current['config_hashes'][COMPOSE] == compose
    adjusted = copy.deepcopy(before)
    adjusted['containers'][OKAD] = copy.deepcopy(actual)
    adjusted['config_hashes'][COMPOSE] = compose
    return adjusted
