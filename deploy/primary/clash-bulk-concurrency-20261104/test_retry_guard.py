from unittest import TestCase
from retry_guard import function


class RetryGuardTests(TestCase):
    def test_extracts_the_entire_budget_function(self):
        before = 'package x\nfunc budget() {\n if used { return }\n}\nfunc next() {}\n'
        self.assertEqual(function(before, 'func budget('), 'func budget() {\n if used { return }\n}\n')
        self.assertNotEqual(function(before, 'func budget('), function(before.replace('if used', 'if false'), 'func budget('))

    def test_missing_guard_is_not_silently_accepted(self):
        with self.assertRaises(ValueError):
            function('package x\n', 'func budget(')
