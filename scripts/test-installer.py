"""Isolated installer smoke test. No services, tasks, trust stores or firewall changes.

Run after installer/build.ps1:
installer/dist/payload/python/python.exe scripts/test-installer.py
"""
import base64
import json
import os
from pathlib import Path
import shutil
import socket
import ssl
import subprocess
import tempfile
import time
import urllib.error
import urllib.request
import pymysql

from apply_migrations import parse_env_value


def free_port():
    with socket.socket() as listener:
        listener.bind(("127.0.0.1", 0))
        return listener.getsockname()[1]


def main():
    project = Path(__file__).resolve().parent.parent
    stage = project / "installer" / "dist" / "payload"
    parent = (project / "installer" / "dist").resolve()
    root = Path(tempfile.mkdtemp(prefix=".test-", dir=parent)).resolve()
    assert root.parent == parent and root.name.startswith(".test-")
    processes, logs = [], []
    flags = subprocess.CREATE_NO_WINDOW
    try:
        setup = stage / "backend-go" / "AUCCProvision.exe"
        subprocess.run([str(setup), "prepare", str(root), "localhost"], check=True, creationflags=flags)
        values = {}
        for line in (root / ".env").read_text(encoding="utf-8").splitlines():
            key, value = line.split("=", 1)
            values[key] = parse_env_value(value, values)
        db_port, http_port = free_port(), free_port()
        ini = (root / "my.ini").read_text().replace("port=3308", f"port={db_port}")
        ini = ini.replace(str(root / "mariadb").replace("\\", "/"), str(stage / "mariadb").replace("\\", "/"))
        (root / "my.ini").write_text(ini, encoding="utf-8")
        shutil.copytree(stage / "migrations", root / "migrations")
        (root / "backend-go").mkdir()
        logs.append(open(root / "initialize.log", "wb"))
        subprocess.run([str(stage / "mariadb" / "bin" / "mariadb-install-db.exe"),
                        f"--datadir={root / 'data'}", f"--port={db_port}"],
                       stdout=logs[-1], stderr=subprocess.STDOUT, check=True, creationflags=flags)
        logs.append(open(root / "database.log", "wb"))
        database = subprocess.Popen([str(stage / "mariadb" / "bin" / "mariadbd.exe"),
                                     f"--defaults-file={root / 'my.ini'}", f"--init-file={root / 'database-init.sql'}"],
                                    stdout=logs[-1], stderr=subprocess.STDOUT, creationflags=flags)
        processes.append(database)
        for _ in range(60):
            assert database.poll() is None, "Bundled database exited during initialization"
            try:
                with socket.create_connection(("127.0.0.1", db_port), timeout=1):
                    break
            except OSError:
                time.sleep(1)
        else:
            raise AssertionError("Database startup timed out")
        env = dict(os.environ)
        env.update(values)
        env.update(DB_PORT=str(db_port), HTTP_ADDR=f"127.0.0.1:{http_port}", FRONTEND_DIST_DIR=str(stage / "frontend" / "dist"))
        python = stage / "python" / "python.exe"
        for _ in range(2):
            result = subprocess.run([str(python), str(root / "migrations" / "apply_migrations.py")],
                                    env=env, capture_output=True, creationflags=flags)
            assert result.returncode == 0, "Bundled migration or migration retry failed"
        root_password = (root / "database-admin.txt").read_text().strip().split(": ", 1)[1]
        with pymysql.connect(host="127.0.0.1", port=db_port, user="root", password=root_password) as connection:
            with connection.cursor() as cursor:
                cursor.execute("SELECT User, Host FROM mysql.global_priv WHERE User = '' OR User = 'root'")
                assert cursor.fetchall() == (("root", "127.0.0.1"),), "Bundled database retained anonymous or remote root accounts"
        logs.append(open(root / "backend.log", "wb"))
        backend = subprocess.Popen([str(stage / "backend-go" / "AUCCServer.exe")], env=env,
                                   cwd=root / "backend-go", stdout=logs[-1], stderr=subprocess.STDOUT, creationflags=flags)
        processes.append(backend)
        context = ssl.create_default_context(cafile=str(root / "tls" / "ca.pem"))
        origin = f"https://127.0.0.1:{http_port}"
        opener = urllib.request.build_opener(urllib.request.ProxyHandler({}), urllib.request.HTTPSHandler(context=context))

        def request(path, body=None):
            data = None if body is None else json.dumps(body).encode()
            req = urllib.request.Request(origin + path, data=data, headers={"Content-Type": "application/json"})
            try:
                with opener.open(req, timeout=3) as response:
                    return response.status, response.read()
            except urllib.error.HTTPError as error:
                return error.code, error.read()

        last_error = ""
        for _ in range(60):
            assert backend.poll() is None, "Bundled backend failed to start"
            try:
                if request("/health/ready")[0] == 200:
                    break
            except (OSError, urllib.error.URLError) as error:
                last_error = str(error)
                time.sleep(1)
        else:
            raise AssertionError("HTTPS readiness timed out: " + last_error)
        assert request("/")[0] == 200, "Built frontend not served"
        assert request("/api/auth/login", {"username": "admin", "password": values["SADMIN_PASSWORD"]})[0] == 200, "Generated admin login failed"
        local = root / "local" / "AUCC Agent"
        local.mkdir(parents=True)
        (local / "config.json").write_text(json.dumps({
            "server_url": origin.replace("https:", "wss:") + "/api/ws/agent",
            "server_ca_file": str(root / "tls" / "ca.pem"), "machine_name": "AGENT-SMOKE",
            "auto_enroll": True, "enrollment_token": values["AGENT_ENROLLMENT_TOKEN"],
        }), encoding="utf-8")
        subprocess.run([str(stage / "client-template" / "dist" / "AUCCAgent.exe"), "--enroll"],
                       env=dict(env, LOCALAPPDATA=str(root / "local")), check=True, creationflags=flags, timeout=20)
        assert "AGENT-SMOKE" in request("/computers")[1].decode(), "Actual Agent binary failed to enroll automatically"
        secret = base64.urlsafe_b64encode(os.urandom(32)).decode().rstrip("=")
        identity = {"name": "AUTO-TEST", "hwid": "AUTO-HWID", "secret": secret,
                    "enrollment_token": values["AGENT_ENROLLMENT_TOKEN"]}
        assert request("/api/agent/enroll", identity)[0] == 200, "Private installer was not auto-approved"
        assert request("/api/agent/enroll", identity)[0] == 200, "Enrollment retry failed"
        assert request("/api/agent/enroll", dict(identity, secret=base64.urlsafe_b64encode(os.urandom(32)).decode().rstrip("=")))[0] == 409, "Installer key replaced an existing station credential"
        assert request("/api/agent/enroll", dict(identity, name="PENDING-TEST", hwid="PENDING-HWID", enrollment_token="wrong"))[0] == 202, "Invalid installer key bypassed admin approval"
        assert "AUTO-TEST" in request("/computers")[1].decode() and "PENDING-TEST" not in request("/computers")[1].decode(), "Public stations do not match approval status"
        with pymysql.connect(host="127.0.0.1", port=db_port, user=values["DB_USER"], password=values["DB_PASS"], database="aucc", autocommit=True) as connection:
            with connection.cursor() as cursor:
                cursor.execute("UPDATE computers SET is_active=0, status='disabled' WHERE name='AUTO-TEST'")
        assert request("/api/agent/enroll", identity)[0] == 409, "Installer key reactivated a disabled station"
        subprocess.run([str(setup), "finish", str(root), "localhost"], check=True, creationflags=flags)
        before = (root / ".env").read_bytes()
        subprocess.run([str(setup), "prepare", str(root), "localhost"], check=True, creationflags=flags)
        assert (root / ".env").read_bytes() == before, "Repair changed installation credentials"
        shutil.copytree(stage / "client-template", root / "client-template")
        shutil.copytree(stage / "compiler", root / "compiler")
        result = subprocess.run(["powershell", "-NoProfile", "-ExecutionPolicy", "Bypass", "-File",
                                 str(project / "scripts" / "install-server.ps1"), "-Mode", "Export", "-Root", str(root)],
                                capture_output=True, creationflags=flags, timeout=210)
        assert result.returncode == 0, "Single-file client packaging failed: " + result.stderr.decode(errors="replace")
        assert (root / "AUCCClientSetup.exe").read_bytes()[:2] == b"MZ", "Client package is not a Windows executable"
        print("Installer smoke test passed: bundled DB/Python, migration retry, trusted HTTPS, frontend, admin login, actual Agent enrollment, private enrollment, collision rejection, disabled stations, repair and Client Setup compilation.")
    finally:
        for process in reversed(processes):
            if process.poll() is None:
                process.terminate()
                process.wait(timeout=15)
        for log in logs:
            log.close()
        assert root.parent == parent and root.name.startswith(".test-")
        shutil.rmtree(root)


if __name__ == "__main__":
    main()
