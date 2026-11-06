from copy import deepcopy
from unittest import TestCase
import resume


class ResumeTests(TestCase):
    def setUp(self):
        self.progress = {'step': 'failed', 'state': 'attention_required', 'error': "Command ['drain-old'] returned non-zero"}
        self.prepared = {'old_id': 'old', 'canary_id': 'candidate', 'candidate_sha': resume.SHA}
        self.old = {'Id': 'old', 'Config': {'Image': resume.OLD}, 'State': {'Health': {'Status': 'healthy'}, 'OOMKilled': False}}
        self.canary = deepcopy(self.old)
        self.canary.update(Id='candidate', Config={'Image': 'adobe-relay:' + resume.SHA})

    def test_only_same_failed_old_drain_can_resume(self):
        resume.validate_resume(self.progress, self.prepared, self.old, self.canary, resume.r.RID, True)

    def test_changed_container_route_or_lease_is_rejected(self):
        for side in ['old', 'canary']:
            old, canary = deepcopy(self.old), deepcopy(self.canary)
            (old if side == 'old' else canary)['Id'] = 'different'
            with self.assertRaises(AssertionError):
                resume.validate_resume(self.progress, self.prepared, old, canary, resume.r.RID, True)
        for owner, routed in [('another-release', True), ('', True), (resume.r.RID, False)]:
            with self.assertRaises(AssertionError):
                resume.validate_resume(self.progress, self.prepared, self.old, self.canary, owner, routed)

    def test_unhealthy_or_oom_candidate_is_rejected(self):
        for health, oom in [('unhealthy', False), ('healthy', True)]:
            candidate = deepcopy(self.canary)
            candidate['State'] = {'Health': {'Status': health}, 'OOMKilled': oom}
            with self.assertRaises(AssertionError):
                resume.validate_resume(self.progress, self.prepared, self.old, candidate, resume.r.RID, True)
