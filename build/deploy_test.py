"""Run deploy.sh against a local fake SSH host: python3 build/deploy_test.py."""

import os
from pathlib import Path
import shlex
import shutil
import signal
import subprocess
import tempfile
import time
import unittest


ROOT = Path(__file__).resolve().parents[1]
APP = r'''#!/usr/bin/env python3
import os, signal, sys, time
from pathlib import Path
role = "ROLE"
pidfile = Path("mock.pid")
with Path("events").open("a") as log:
    log.write(role + " " + " ".join(sys.argv[1:]) + "\n")
if sys.argv[1:] == ["--help"]:
    sys.exit(1 if role == "new" and os.getenv("BAD_BINARY") else 0)
if sys.argv[1:3] == ["server", "stop"]:
    if os.getenv("FAIL_STOP"):
        sys.exit(1)
    if pidfile.exists():
        try:
            os.kill(int(pidfile.read_text()), signal.SIGTERM)
        except ProcessLookupError:
            pidfile.unlink(missing_ok=True)
        for _ in range(100):
            if not pidfile.exists():
                sys.exit(0)
            time.sleep(.01)
        sys.exit(1)
    sys.exit(0)
if sys.argv[1:3] == ["server", "status"]:
    if role == "new" and os.getenv("FAIL_HEALTH"):
        sys.exit(1)
    if not pidfile.exists():
        sys.exit(1)
    os.kill(int(pidfile.read_text()), 0)
    sys.exit(0)
if sys.argv[1:] == ["server"]:
    if role == "new" and os.getenv("FAIL_START"):
        sys.exit(1)
    def stop(*args):
        pidfile.unlink(missing_ok=True)
        sys.exit(0)
    signal.signal(signal.SIGTERM, stop)
    pidfile.write_text(str(os.getpid()))
    while True:
        time.sleep(.05)
sys.exit(2)
'''


def executable(path, content):
    path.write_text(content)
    path.chmod(0o755)


class DeployTest(unittest.TestCase):
    def test_deployment(self):
        cases = [
            ({}, 0, "new"),
            ({"MOCK_ARCH": "aarch64"}, 0, "new"),
            ({"MOCK_ARCH": "mips"}, 1, "old"),
            ({"FAIL_SSH": "1"}, 255, "old"),
            ({"FAIL_BUILD": "1"}, 1, "old"),
            ({"BAD_BINARY": "1"}, 1, "old"),
            ({"FAIL_STOP": "1"}, 1, "old"),
            ({"FAIL_START": "1"}, 1, "old"),
            ({"FAIL_HEALTH": "1"}, 1, "old"),
            ({"FIRST_DEPLOY": "1"}, 0, "new"),
            ({"FIRST_DEPLOY": "1", "FAIL_START": "1"}, 1, "new"),
        ]
        for overrides, exit_code, expected_role in cases:
            with self.subTest(overrides=overrides), tempfile.TemporaryDirectory() as tmp:
                root = Path(tmp)
                remote = root / "remote host's files"
                remote.mkdir()
                (root / "build").mkdir()
                shim = root / "bin"
                shim.mkdir()
                shutil.copy2(ROOT / "deploy.sh", root / "deploy.sh")
                (root / "main.go").write_text('var AppVer = "26102001"\n')
                (root / ".env").write_text(
                    "NAS_HOST=test@mock\nNAS_PORT=2233\nNAS_DIR="
                    + shlex.quote(str(remote)) + "\n"
                )
                first_deploy = bool(overrides.get("FIRST_DEPLOY"))
                if not first_deploy:
                    executable(remote / "wx_video_download", APP.replace('"ROLE"', '"old"'))
                executable(root / "new-app", APP.replace('"ROLE"', '"new"'))
                executable(shim / "ssh", '''#!/usr/bin/env python3
import os, sys
if "-O" in sys.argv:
    sys.exit(0)
if os.getenv("FAIL_SSH"):
    sys.exit(255)
os.execvp("bash", ["bash", "-c", sys.argv[-1]])
''')
                executable(shim / "uname", '#!/bin/bash\nif [[ "$1" == -s ]]; then echo Linux; else echo "${MOCK_ARCH:-x86_64}"; fi\n')
                executable(shim / "git", '#!/bin/bash\necho abc123\n')
                executable(shim / "sleep", '#!/bin/bash\n/bin/sleep .05\n')
                (root / "build/build-go.sh").write_text('''set -eu
test "${FAIL_BUILD:-}" != 1
printf '%s\n' "$CGO_ENABLED" "$GOOS" "$GOARCH" "$@" > build-args
while [[ "$1" != -o ]]; do shift; done
cp "$MOCK_APP" "$2"
''')
                env = dict(os.environ, PATH=str(shim) + os.pathsep + os.environ["PATH"],
                           MOCK_APP=str(root / "new-app"), **overrides)
                old_process = None if first_deploy else subprocess.Popen(
                    [str(remote / "wx_video_download"), "server"], cwd=remote, env=env)
                try:
                    if old_process is not None:
                        for _ in range(100):
                            if (remote / "mock.pid").exists():
                                break
                            time.sleep(.01)
                        self.assertTrue((remote / "mock.pid").exists())
                    result = subprocess.run(["bash", str(root / "deploy.sh")], cwd="/",
                                            env=env, capture_output=True, text=True, timeout=20)
                    self.assertEqual(result.returncode, exit_code, result.stdout + result.stderr)
                    self.assertIn('role = "' + expected_role + '"',
                                  (remote / "wx_video_download").read_text())
                    if exit_code == 0:
                        args = (root / "build-args").read_text().splitlines()
                        self.assertEqual(args[:3], ["0", "linux", "arm64" if overrides.get("MOCK_ARCH") == "aarch64" else "amd64"])
                        self.assertIn("embed_inject,sqlite_only,embed_frontend_inject", args)
                        self.assertIn("-ldflags=-s -w -X main.Mode=release -X main.AppVer=26102001-abc123", args)
                        self.assertEqual((remote / "wx_video_download.bak").exists(), not first_deploy)
                        self.assertTrue((remote / "mock.pid").exists())
                    if (overrides.get("FAIL_START") or overrides.get("FAIL_HEALTH")) and not first_deploy:
                        self.assertIn("Previous binary restored and server restarted", result.stderr)
                    if overrides.get("BAD_BINARY") or overrides.get("FAIL_BUILD"):
                        self.assertNotIn("old server stop", (remote / "events").read_text()
                                         if (remote / "events").exists() else "")
                finally:
                    if (remote / "mock.pid").exists():
                        os.kill(int((remote / "mock.pid").read_text()), signal.SIGTERM)
                    if old_process is not None:
                        old_process.wait(timeout=5)


if __name__ == "__main__":
    unittest.main()
