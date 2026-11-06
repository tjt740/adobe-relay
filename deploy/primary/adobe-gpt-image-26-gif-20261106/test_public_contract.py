import copy
import unittest
from public_contract_guard import MODELS, validate


def proof():
    rows = []
    for model in MODELS:
        for mode in ['generate', 'edit'] + (['gif', 'reject-json', 'reject-multipart', 'reject-generation-reference'] if model in MODELS[-2:] else []):
            reject = mode.startswith('reject-')
            frames = 4 if model.endswith('star-swirl') else 16
            rows.append(dict(case=model + '-' + mode, model=model, application_sha='a' * 40,
                             success=True, outer_posts=1, api_key_id=999, expected_rejection=reject,
                             status=400 if reject else 200,
                             error=dict(code='gif_edit_unsupported', param='image', stage='validation', source='request'),
                             image=dict(width=896, height=896, format='jpeg' if mode == 'edit' else 'gif' if mode == 'gif' else 'png', frame_count=frames, delays_centiseconds=[20] * frames)))
    current = dict(application_sha='a' * 40, group_id=33, all_passed=True, cases=rows)
    legacy = dict(application_sha='a' * 40, all_passed=False, cases=[dict(success=False)] * 18)
    cleanup = dict(key_deleted=True, temporary_grant_removed=True, private_files_removed=True, other_allowed_groups_preserved=True, existing_customer_keys_changed=False)
    return current, legacy, cleanup, 'a' * 40


class PublicContractTests(unittest.TestCase):
    def test_preserves_failed_legacy_diagnostics(self):
        args = proof()
        original = copy.deepcopy(args[1])
        result = validate(*args)
        self.assertFalse(result['legacy_group_7_all_passed'])
        self.assertEqual(result['legacy_group_7_failures'], 18)
        self.assertEqual(args[1], original)

    def test_missing_or_duplicate_current_case_fails(self):
        for duplicate in [True, False]:
            args = proof()
            args[0]['cases'].pop()
            if duplicate:
                args[0]['cases'].append(args[0]['cases'][0])
            with self.assertRaises(AssertionError):
                validate(*args)

    def test_new_failed_case_cannot_be_overridden_by_summary(self):
        args = proof()
        args[0]['cases'][0]['success'] = False
        with self.assertRaises(AssertionError):
            validate(*args)

    def test_missing_cleanup_fails(self):
        args = proof()
        args[2]['temporary_grant_removed'] = False
        with self.assertRaises(AssertionError):
            validate(*args)

    def test_wrong_gif_frames_fail(self):
        args = proof()
        row = next(row for row in args[0]['cases'] if row['case'].endswith('-gif'))
        row['image']['frame_count'] = 1
        with self.assertRaises(AssertionError):
            validate(*args)

    def test_legacy_record_cannot_disappear(self):
        args = proof()
        args[1]['cases'].pop()
        with self.assertRaises(AssertionError):
            validate(*args)
