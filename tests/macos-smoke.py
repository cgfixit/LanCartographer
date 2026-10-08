#!/usr/bin/env python3
"""Exercise the packaged CLI against an owned loopback HTTP service only."""
import http.server
import json
import os
from pathlib import Path
import plistlib
import re
import subprocess
import sys
import tempfile
import threading


class Fixture(http.server.BaseHTTPRequestHandler):
    requests = 0

    def do_GET(self):
        type(self).requests += 1
        body = b"<html><title>LanCartographer loopback fixture</title></html>"
        self.send_response(200)
        self.send_header("Content-Type", "text/html")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def log_message(self, *args):
        pass


def main():
    dist = Path(sys.argv[1] if len(sys.argv) > 1 else "dist").resolve()
    cli = dist / "lancartographer-cli"
    gui = dist / "LanCartographer.app/Contents/MacOS/LanCartographer"
    info = plistlib.loads((dist / "LanCartographer.app/Contents/Info.plist").read_bytes())
    minimum = info["LSMinimumSystemVersion"]
    for binary in (cli, gui):
        headers = subprocess.check_output(["otool", "-l", str(binary)], text=True)
        versions = re.findall(r"^\s*minos ([\d.]+)$", headers, re.MULTILINE)
        assert versions == [minimum], (binary, versions, minimum)
    print(f"PASS: Mach-O deployment targets match bundle minimum macOS {minimum}")
    # A discovery port exercises both liveness and inventory. An occupied
    # port fails the check; it is never stopped or taken over.
    port = 8080
    server = http.server.ThreadingHTTPServer(("127.0.0.1", port), Fixture)
    worker = threading.Thread(target=server.serve_forever, daemon=True)
    worker.start()
    try:
        with tempfile.TemporaryDirectory(prefix="lancartographer-smoke-") as tmp:
            env = dict(os.environ, APPDATA=tmp)
            args = ["--range", "127.0.0.1/32", "--ports", str(port), "--timeout", "100ms"]
            for binary, prefix in ((cli, []), (gui, ["--cli"])):
                before = Fixture.requests
                refused = subprocess.run([str(binary), *prefix, *args], env=env,
                                         capture_output=True, text=True, timeout=10)
                assert refused.returncode == 2, refused
                assert "requires confirmation" in refused.stderr, refused.stderr
                assert Fixture.requests == before, "refused scan contacted the HTTP fixture"
                dest = Path(tmp) / "inventory.json"
                done = subprocess.run([str(binary), *prefix, *args, "--i-am-authorized", "--json", str(dest)],
                                      env=env, capture_output=True, text=True, timeout=10)
                assert done.returncode == 0, done.stderr
                hosts = json.loads(dest.read_text())
                assert len(hosts) == 1 and hosts[0]["IP"] == "127.0.0.1", hosts
                ports = hosts[0]["Ports"]
                assert any(p["Number"] == port and p["State"] == "open" and
                           p.get("Title") == "LanCartographer loopback fixture" for p in ports), ports
                assert hosts[0].get("MAC", "") != "needs elevation", hosts
                print(f"PASS: {binary.name}: authorized scan, HTTP title, JSON export; refusal exits 2")
    finally:
        server.shutdown()
        server.server_close()
        worker.join()


if __name__ == "__main__":
    main()
