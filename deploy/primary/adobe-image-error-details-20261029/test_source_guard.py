from copy import deepcopy
from unittest import TestCase
from source_guard import ALLOWED, validate


class SourceGuardTests(TestCase):
    def setUp(self):
        self.manifest = {p: {'before': 'a' * 64, 'candidate': 'b' * 64} for p in ALLOWED}
        self.before = {p: v['before'] for p, v in self.manifest.items()}
        self.candidate = {p: v['candidate'] for p, v in self.manifest.items()}

    def test_accepts_only_the_reviewed_sources(self):
        validate(ALLOWED, self.before, self.candidate, self.manifest)

    def test_rejects_extra_migrations_scheduler_and_startup_changes(self):
        for extra in ['backend/migrations/255_unreviewed.sql', 'backend/internal/proxyfailover/allocator.go', 'Dockerfile', 'deploy/docker-entrypoint.sh', '.github/workflows/deploy-adobe-relay.yml']:
            with self.subTest(extra=extra), self.assertRaises(AssertionError):
                validate(ALLOWED | {extra}, self.before, self.candidate, self.manifest)

    def test_rejects_missing_reviewed_files(self):
        with self.assertRaises(AssertionError):
            validate(set(), self.before, self.candidate, self.manifest)

    def test_rejects_source_drift(self):
        for before, candidate in [({}, self.candidate), (self.before, {})]:
            with self.assertRaises(AssertionError):
                validate(ALLOWED, before, candidate, self.manifest)

    def test_rejects_manifest_scope_expansion(self):
        manifest = deepcopy(self.manifest)
        manifest['backend/migrations/255_unreviewed.sql'] = {'before': 'a', 'candidate': 'b'}
        with self.assertRaises(AssertionError):
            validate(manifest, self.before, self.candidate, manifest)
