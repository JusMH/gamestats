"""Check the staged Scala artifact with local JDK 21, without sbt at runtime.

This checks packaging on the host; it is NOT a container integration test.
Run sbtw.ps1 dockerStage (or sbt dockerStage) first.
"""
from decimal import Decimal
import json
import os
from pathlib import Path
import socket
import subprocess
import tempfile
import time
from urllib.request import Request, urlopen

ROOT = Path(__file__).resolve().parents[1]
STAGE = ROOT / "scala-analysis/target/docker-stage"
FIXTURES = ROOT / "docs/fixtures/analysis-v1"


def load(data):
    return json.loads(data, parse_float=Decimal)


def main():
    assert (STAGE / "app.jar").is_file(), "run dockerStage first"
    jars = sorted((STAGE / "lib").glob("*.jar"))
    assert len(jars) == 5 and not any("munit" in jar.name for jar in jars), "wrong runtime JARs"
    with socket.socket() as address:
        address.bind(("127.0.0.1", 0))
        port = address.getsockname()[1]
    env = dict(os.environ, SCALA_HOST="127.0.0.1", SCALA_PORT=str(port),
               SCALA_PROFILES_DIR=str(ROOT / "docs/examples/profiles"))
    command = ["java", "-cp", str(STAGE / "app.jar") + os.pathsep + str(STAGE / "lib/*"),
               "gamestats.Main"]
    options = {"creationflags": subprocess.CREATE_NO_WINDOW} if os.name == "nt" else {}
    # No source/classes/cache path is present in this runtime classpath.
    with tempfile.TemporaryFile() as log:
        process = subprocess.Popen(command, cwd=STAGE, env=env, stdout=log, stderr=log, **options)
        try:
            deadline = time.monotonic() + 20
            while True:
                if process.poll() is not None:
                    raise RuntimeError("staged service exited before health")
                try:
                    with urlopen(f"http://127.0.0.1:{port}/health", timeout=1) as health:
                        assert load(health.read()) == {"status": "ok"}
                    break
                except OSError:
                    assert time.monotonic() < deadline, "staged service not ready"
                    time.sleep(0.05)
            cases = load((FIXTURES / "manifest.json").read_bytes())["cases"]
            for case in cases:
                request = Request(f"http://127.0.0.1:{port}/analyze",
                                  data=(FIXTURES / case["request"]).read_bytes(),
                                  headers={"Content-Type": "application/json"})
                with urlopen(request, timeout=5) as response:
                    actual = load(response.read())
                assert actual == load((FIXTURES / case["expected"]).read_bytes()), case["id"]
        finally:
            process.terminate()
            process.wait(timeout=10)
    with tempfile.TemporaryDirectory(dir=STAGE) as empty_profiles:
        env["SCALA_PROFILES_DIR"] = empty_profiles
        failure = subprocess.run(command, cwd=STAGE, env=env, capture_output=True,
                                 text=True, timeout=10, **options)
        assert failure.returncode == 1 and "Cannot start analysis service" in failure.stderr
    print("PASS: staged app.jar + 5 runtime JARs, health, 13 complete fixtures; empty profiles exit 1")
    print("Host packaging check only; Docker build/runtime still require a working engine.")


if __name__ == "__main__":
    main()