# migrations/apply_migrations.py
import ipaddress
import os
import ssl
import sys
import pymysql

def split_sql(sql: str) -> list[str]:
    """Split MySQL statements without treating quoted semicolons as separators."""
    statements, current = [], []
    quote = None
    line_comment = block_comment = False
    i = 0
    while i < len(sql):
        ch = sql[i]
        nxt = sql[i + 1] if i + 1 < len(sql) else ""
        if line_comment:
            if ch == "\n":
                line_comment = False
                current.append(ch)
            i += 1
            continue
        if block_comment:
            if ch == "*" and nxt == "/":
                block_comment = False
                i += 2
            else:
                if ch == "\n":
                    current.append(ch)
                i += 1
            continue
        if quote:
            current.append(ch)
            if ch == "\\" and quote != "`" and nxt:
                current.append(nxt)
                i += 2
                continue
            if ch == quote:
                if nxt == quote:
                    current.append(nxt)
                    i += 2
                    continue
                quote = None
            i += 1
            continue
        if ch in ("'", '"', "`"):
            quote = ch
            current.append(ch)
            i += 1
        elif ch == "/" and nxt == "*":
            block_comment = True
            current.append(" ")
            i += 2
        elif ch == "#" or (ch == "-" and nxt == "-" and
                            (i + 2 == len(sql) or sql[i + 2].isspace())):
            line_comment = True
            i += 2 if ch == "-" else 1
        elif ch == ";":
            statement = "".join(current).strip()
            if statement:
                statements.append(statement)
            current.clear()
            i += 1
        else:
            current.append(ch)
            i += 1
    if quote or block_comment:
        raise ValueError("Unterminated SQL string or block comment")
    statement = "".join(current).strip()
    if statement:
        statements.append(statement)
    return statements


def run_sql_file(cursor, file_path: str):
    print(f"[*] Executing {os.path.basename(file_path)}...")
    with open(file_path, "r", encoding="utf-8") as f:
        content = f.read()
    for stmt in split_sql(content):
        cursor.execute(stmt)


def load_database_config() -> dict:
    env_path = os.path.join(os.path.dirname(__file__), "..", ".env")
    if os.path.exists(env_path):
        with open(env_path, "r", encoding="utf-8-sig") as env_file:
            for line in env_file:
                line = line.strip()
                if line and not line.startswith("#") and "=" in line:
                    key, value = line.split("=", 1)
                    value = value.strip()
                    if len(value) >= 2 and value[0] == value[-1] and value[0] in ("'", '"'):
                        value = value[1:-1]
                    os.environ.setdefault(key.strip(), value)

    required = ("DB_HOST", "DB_USER", "DB_PASS", "DB_NAME")
    missing = [key for key in required if not os.getenv(key)]
    if missing:
        raise ValueError("Missing required database settings: " + ", ".join(missing))
    try:
        port = int(os.getenv("DB_PORT", "3306"))
    except ValueError as exc:
        raise ValueError("DB_PORT must be an integer") from exc
    if port < 1 or port > 65535:
        raise ValueError("DB_PORT must be between 1 and 65535")

    host = os.environ["DB_HOST"]
    config = {
        "host": host,
        "user": os.environ["DB_USER"],
        "password": os.environ["DB_PASS"],
        "database": os.environ["DB_NAME"],
        "port": port,
    }
    tls_context = database_tls_context(host, os.getenv("DB_TLS_CA_FILE", ""))
    if tls_context is not None:
        config["ssl"] = tls_context
    return config


def database_tls_context(host: str, ca_file: str = ""):
    normalized_host = host.strip().strip("[]")
    try:
        is_loopback = ipaddress.ip_address(normalized_host).is_loopback
    except ValueError:
        is_loopback = normalized_host.lower() == "localhost"
    if is_loopback:
        return None

    try:
        context = ssl.create_default_context()
        if ca_file:
            context.load_verify_locations(cafile=ca_file)
    except (OSError, ssl.SSLError):
        raise ValueError("Unable to load verified database TLS roots") from None
    context.minimum_version = ssl.TLSVersion.TLSv1_2
    context.verify_mode = ssl.CERT_REQUIRED
    context.check_hostname = True
    return context


def column_exists(cursor, table: str, column: str) -> bool:
    # Identifiers are internal constants, not request/environment input.
    cursor.execute(f"SHOW COLUMNS FROM `{table}` LIKE %s", (column,))
    return cursor.fetchone() is not None


def index_exists(cursor, table: str, index: str) -> bool:
    # Identifiers are internal constants, not request/environment input.
    cursor.execute(f"SHOW INDEX FROM `{table}` WHERE Key_name = %s", (index,))
    return cursor.fetchone() is not None


def apply_migrations(cursor, connection):
    cursor.execute(
        "CREATE TABLE IF NOT EXISTS schema_migrations ("
        "version VARCHAR(64) PRIMARY KEY, "
        "applied_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP"
        ") ENGINE=InnoDB DEFAULT CHARSET=utf8mb4"
    )
    connection.commit()

    migrations_dir = os.path.dirname(__file__)
    migration_files = sorted(
        file_name for file_name in os.listdir(migrations_dir)
        if file_name.endswith(".up.sql") and file_name[:6].isdigit()
    )

    for file_name in migration_files:
        version = file_name[:6]
        cursor.execute("SELECT 1 FROM schema_migrations WHERE version = %s", (version,))
        if cursor.fetchone():
            continue

        # Older installations may already have these columns without a ledger.
        if version == "000003" and column_exists(cursor, "users", "token_version"):
            pass
        elif version == "000004" and column_exists(cursor, "computers", "agent_secret_hash"):
            pass
        elif version == "000005" and column_exists(cursor, "usage_logs", "session_ends_at"):
            if not index_exists(cursor, "usage_logs", "idx_usage_active_sessions"):
                cursor.execute(
                    "ALTER TABLE `usage_logs` ADD INDEX `idx_usage_active_sessions` "
                    "(`computer_id`, `end_time`, `session_ends_at`)"
                )
        elif version == "000007":
            # MySQL commits DDL separately. A retry must resume after any
            # column/table already created before the ledger insert.
            with open(os.path.join(migrations_dir, file_name), encoding="utf-8") as migration_file:
                statements = split_sql(migration_file.read())
            if not column_exists(cursor, "computers", "agent_version"):
                cursor.execute(statements[0])
            if not column_exists(cursor, "computers", "agent_update_error"):
                cursor.execute(statements[1])
            cursor.execute(statements[2])
        else:
            run_sql_file(cursor, os.path.join(migrations_dir, file_name))

        cursor.execute("INSERT INTO schema_migrations (version) VALUES (%s)", (version,))
        connection.commit()


def main():
    try:
        db_config = load_database_config()
    except ValueError as error:
        print(f"[ERROR] {error}")
        sys.exit(1)

    print("=== Database Migration Runner ===")
    conn = None
    try:
        conn = pymysql.connect(**db_config, autocommit=False)
        with conn.cursor() as cursor:
            init_file = os.path.join(os.path.dirname(__file__), "init_database.sql")
            run_sql_file(cursor, init_file)
            conn.commit()
            apply_migrations(cursor, conn)
        print("[+] Migration complete.")
    except Exception as e:
        print("[!] MySQL may already have committed DDL; rerun after resolving the reported error.")
        if conn is not None:
            conn.rollback()
        print(f"[ERROR] Migration failed ({type(e).__name__}); connection details redacted.")
        sys.exit(1)
    finally:
        if conn is not None:
            conn.close()


if __name__ == "__main__":
    main()
