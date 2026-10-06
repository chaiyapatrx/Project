import os
import ssl
import tempfile
import unittest
from unittest.mock import Mock, patch

from apply_migrations import apply_migrations, database_tls_context, parse_env_value, split_sql


class DatabaseTLSContextTests(unittest.TestCase):
    def test_env_values_match_backend_quoted_passwords(self):
        self.assertEqual(parse_env_value('"password # with=space" # note', {}), 'password # with=space')
        self.assertEqual(parse_env_value(r'"quoted\"pass\\word"', {}), 'quoted"pass\\word')
        self.assertEqual(parse_env_value('${FIRST}_suffix', {'FIRST': 'prefix'}), 'prefix_suffix')
        self.assertEqual(parse_env_value("'$FIRST' # note", {'FIRST': 'prefix'}), '$FIRST')
        self.assertEqual(parse_env_value(r'\$FIRST', {'FIRST': 'prefix'}), '$FIRST')
        with self.assertRaises(ValueError):
            parse_env_value('"unclosed', {})

    def test_sql_parser_preserves_quoted_semicolons_and_comments(self):
        self.assertEqual(split_sql("SELECT 'a;''b'; -- comment\nSELECT 2/* ; */;"),
                         ["SELECT 'a;''b'", "SELECT 2"])
        for sql in ("SELECT 'unfinished", "SELECT 1 /* unfinished"):
            with self.assertRaises(ValueError):
                split_sql(sql)

    def test_enrollment_migration_resumes_after_column(self):
        cursor = Mock()
        cursor.fetchone.side_effect = [None, ("pending_approval",), None]
        with patch("apply_migrations.os.listdir", return_value=["000006_agent_enrollment.up.sql"]):
            apply_migrations(cursor, Mock())
        statements = [call.args[0] for call in cursor.execute.call_args_list]
        self.assertFalse(any("ADD COLUMN" in sql for sql in statements))
        self.assertTrue(any("ADD INDEX" in sql for sql in statements))

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
