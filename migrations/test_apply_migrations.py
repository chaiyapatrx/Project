import os
import ssl
import tempfile
import unittest
from unittest.mock import Mock, patch

from apply_migrations import apply_migrations, database_tls_context


class DatabaseTLSContextTests(unittest.TestCase):
    def test_loopback_hosts_keep_local_development_usable(self):
        for host in ("localhost", "127.0.0.1", "::1", "[::1]"):
            with self.subTest(host=host):
                self.assertIsNone(database_tls_context(host))

    def test_remote_host_requires_verified_tls(self):
        context = database_tls_context("db.example.invalid")
        self.assertIsInstance(context, ssl.SSLContext)
        self.assertGreaterEqual(context.minimum_version, ssl.TLSVersion.TLSv1_2)
        self.assertEqual(context.verify_mode, ssl.CERT_REQUIRED)
        self.assertTrue(context.check_hostname)

    def test_invalid_custom_ca_fails_closed(self):
        with tempfile.NamedTemporaryFile(delete=False) as ca_file:
            ca_file.write(b"not a certificate")
            path = ca_file.name
        try:
            with self.assertRaises(ValueError):
                database_tls_context("db.example.invalid", path)
        finally:
            os.unlink(path)

    def test_agent_update_migration_resumes_after_first_column(self):
        class Cursor:
            def __init__(self):
                self.calls = []

            def execute(self, sql, params=None):
                self.calls.append((sql, params))

            def fetchone(self):
                sql, params = self.calls[-1]
                if sql.startswith("SHOW COLUMNS") and params == ("agent_version",):
                    return ("agent_version",)
                return None

        cursor = Cursor()
        with patch("apply_migrations.os.listdir", return_value=["000007_agent_updates.up.sql"]):
            apply_migrations(cursor, Mock())
        statements = [sql for sql, _ in cursor.calls]
        self.assertFalse(any("ADD COLUMN `agent_version`" in sql for sql in statements))
        self.assertTrue(any("ADD COLUMN `agent_update_error`" in sql for sql in statements))
        self.assertTrue(any("CREATE TABLE IF NOT EXISTS `agent_releases`" in sql for sql in statements))


if __name__ == "__main__":
    unittest.main()
