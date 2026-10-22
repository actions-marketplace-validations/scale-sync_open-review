import tempfile
import unittest

from openreview import scan


class SmokeTest(unittest.TestCase):
    def test_empty_scan(self):
        with tempfile.TemporaryDirectory() as repo:
            result = scan(repo, ".")
        self.assertEqual(result["contractVersion"], 2)
        self.assertEqual(result["status"], "empty")
        self.assertEqual(result["findings"], [])
