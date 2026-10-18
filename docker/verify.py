"""Compose end-to-end verification. Python 3.10+ stdlib; no Go/Scala on host.

Uses a fresh project, real service containers and the unchanged simulator.
Never reuses or removes another project's containers, networks or volumes.
"""
import argparse
import json
import os
from pathlib import Path
import re
import subprocess
import sys
import uuid

ROOT = Path(__file__).resolve().parents[1]
COUNTS = {
    ("tic-tac-toe", "ttt-win"): 7,
    ("tic-tac-toe", "ttt-draw"): 11,
    ("snake", "snake-positive"): 9,
    ("snake", "snake-negative"): 9,
    ("racing", "race-sprint"): 7,
    ("racing", "race-endurance"): 7,
}


def require(condition, message):
    if not condition:
        raise AssertionError(message)


def expected_matches(run_id):
    return {(game, run_id + "-" + name): count for (game, name), count in COUNTS.items()}


def check_demo(report, run_id):
    expected = expected_matches(run_id)
    matches = report["matches"]
    require(len(matches) == 6, "expected six matches")
    require({(m["gameId"], m["matchId"]) for m in matches} == set(expected),
            "unexpected match identities")
    for match in matches:
        key = match["gameId"], match["matchId"]
        require(match["verified"] is True and not match.get("error"), str(key) + ": not verified")
        require(match["accepted"] == expected[key], str(key) + ": wrong accepted count")
        require(match["queries"] == 10 and len(match["results"]) == 10, str(key) + ": wrong queries")
        require([r["state"] for r in match["results"]] == ["open"] * 8 + ["closed"] * 2,
                str(key) + ": wrong live/final transitions")
        for result in match["results"]:
            require((result["gameId"], result["matchId"]) == key, "mixed result identities")
        # The simulator itself compared every row/value/pattern against independent
        # expectations. Additionally confirm all repeated responses here.
        for first, second in zip(match["results"][::2], match["results"][1::2]):
            require(first == second, str(key) + ": repeated results differ")
    require(report["peakRequests"] >= 2, "no overlapping HTTP requests observed")
    return {"matches": 6, "eventsAccepted": 50, "resultsChecked": 60,
            "openResults": 48, "closedResults": 12, "peakRequests": report["peakRequests"]}


def check_duplicate(report, run_id):
    matches = report["matches"]
    require(len(matches) == 6 and
            {(m["gameId"], m["matchId"]) for m in matches} == set(expected_matches(run_id)),
            "duplicate report identities differ")
    for match in matches:
        require(match["accepted"] == 0 and match["queries"] == 0 and match["verified"] is False,
                "duplicate execution changed state or claimed success")
        message = match.get("error", "")
        require("HTTP 409:" in message and "eventId already exists in this match" in message,
                "failure is not the expected duplicate rejection: " + message)


def check_negative(report):
    checks = report["checks"]
    require(len(checks) == 3 and {c["name"] for c in checks} == {
        "envelope_rejected_without_mutation", "invalid_domain_independent_pattern",
        "unknown_game_empty_results"}, "negative checks missing")
    require(all(c["passed"] is True and not c.get("error") for c in checks),
            "a negative check failed")


class Docker:
    def __init__(self, project, evidence):
        self.project = project
        self.evidence = evidence
        self.env = dict(os.environ, GO_PORT="0", GO_BIND_ADDRESS="127.0.0.1")
        self.compose = ["compose", "--project-name", project,
                        "--file", str(ROOT / "compose.yaml")]

    def call(self, args, name, expected=0, timeout=60):
        command = ["docker", *args]
        print("$ " + " ".join(command), flush=True)
        try:
            result = subprocess.run(command, cwd=ROOT, env=self.env, text=True,
                                    encoding="utf-8", errors="replace",
                                    stdout=subprocess.PIPE, stderr=subprocess.STDOUT, timeout=timeout)
        except subprocess.TimeoutExpired as error:
            output = error.stdout or b""
            if isinstance(output, bytes):
                output = output.decode("utf-8", errors="replace")
            (self.evidence / (name + ".log")).write_text(output, encoding="utf-8")
            raise RuntimeError(name + ": timed out; Docker engine may be unavailable") from error
        (self.evidence / (name + ".log")).write_text(result.stdout, encoding="utf-8")
        if result.returncode != expected:
            raise RuntimeError(name + ": exit " + str(result.returncode) +
                               ", expected " + str(expected) + "\n" + result.stdout[-4000:])
        return result.stdout

    def dc(self, args, name, **kwargs):
        return self.call([*self.compose, *args], name, **kwargs)

    def resources(self):
        containers = self.call(["ps", "-aq", "--filter",
                               "label=com.docker.compose.project=" + self.project], "containers")
        networks = self.call(["network", "ls", "-q", "--filter",
                             "label=com.docker.compose.project=" + self.project], "networks")
        images = self.call(["image", "ls", "-q", "--filter",
                            "reference=" + self.project + "-*"], "images-remaining")
        return containers.strip() or networks.strip() or images.strip()

    def remove_owned_task(self, name):
        # Only remove a named container if its Compose ownership label matches.
        result = subprocess.run(
            ["docker", "inspect", "--format",
             '{{index .Config.Labels "com.docker.compose.project"}}', name],
            cwd=ROOT, env=self.env, capture_output=True, text=True, timeout=15)
        if result.returncode == 0:
            require(result.stdout.strip() == self.project, "refusing to remove foreign container " + name)
            self.call(["rm", "--force", name], "remove-" + name)

    def task(self, kind, run_id, expected=0, mode="demo"):
        name = self.project + "-" + kind
        report = self.evidence / (kind + ".json")
        try:
            self.dc(["run", "--no-deps", "--name", name, "simulator",
                     "--run-id", run_id, "--mode", mode, "--verify=true",
                     "--report", "/reports/result.json"], kind, expected=expected, timeout=150)
            self.call(["cp", name + ":/reports/result.json", str(report)], "copy-" + kind)
            return json.loads(report.read_text(encoding="utf-8"))
        finally:
            self.remove_owned_task(name)

    def bad_profiles(self):
        name = self.project + "-bad-profiles"
        try:
            output = self.dc(["run", "--no-deps", "--name", name,
                              "-e", "SCALA_PROFILES_DIR=/app/no-such-profiles",
                              "scala-analysis"], "bad-profiles", expected=1)
            require("Cannot start analysis service: profile_directory at profiles" in output,
                    "failure is not the expected profile validation")
        finally:
            self.remove_owned_task(name)

    def health(self):
        ids = self.dc(["ps", "-q", "go-ingestion", "scala-analysis"], "service-ids").split()
        require(len(ids) == 2, "expected two running services")
        containers = json.loads(self.call(["inspect", *ids], "service-inspect"))
        for container in containers:
            require(container["State"]["Running"] and
                    container["State"]["Health"]["Status"] == "healthy", "unhealthy service")
            require(container["Config"]["User"] == "10001:10001", "runtime user must be unprivileged")
        self.dc(["exec", "-T", "go-ingestion", "wget", "-q", "-O", "-",
                 "http://scala-analysis:9000/health"], "internal-health")
        self.dc(["exec", "-T", "scala-analysis", "sh", "-c",
                 "command -v wget && test -r /app/app.jar && "
                 "test ! -e /tmp/sbt-launch.jar && test ! -x /opt/java/openjdk/bin/javac"],
                "scala-runtime")
        return containers


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--project", default="gamestats-check-" + uuid.uuid4().hex[:8])
    parser.add_argument("--run-id", default="docker-demo-01")
    parser.add_argument("--evidence", type=Path)
    args = parser.parse_args()
    require(re.fullmatch(r"[a-z0-9][a-z0-9_-]{0,39}", args.project), "invalid project name")
    require(re.fullmatch(r"[A-Za-z0-9][A-Za-z0-9_-]{0,43}", args.run_id),
            "run-id must be 1..44 characters (reserves suffix -new)")
    evidence = (args.evidence or ROOT / "docker/evidence" / args.project).resolve()
    require(not evidence.exists() or not any(evidence.iterdir()),
            "evidence directory must be empty to avoid overwriting earlier runs")
    evidence.mkdir(parents=True, exist_ok=True)
    docker = Docker(args.project, evidence)
    owned = False
    summary = {"project": args.project, "containerIntegrationExecuted": False, "status": "blocked"}
    try:
        docker.call(["compose", "version"], "compose-version")
        docker.dc(["--profile", "tools", "config", "--format", "json"], "compose-config")
        engine = docker.call(["info", "--format", "{{.OSType}}"], "engine", timeout=15).strip()
        require(engine == "linux", "Linux container engine required; no configuration changed")
        docker.call(["version", "--format", "json"], "docker-version")
        require(not docker.resources(), "project already exists; refusing to touch its resources")
        owned = True
        docker.dc(["--profile", "tools", "build"], "build", timeout=3600)
        docker.dc(["up", "-d", "--wait", "--wait-timeout", "120",
                   "go-ingestion", "scala-analysis"], "up", timeout=150)
        docker.health()
        docker.bad_profiles()
        summary["containerIntegrationExecuted"] = True
        first = check_demo(docker.task("demo", args.run_id), args.run_id)
        check_negative(docker.task("negative", args.run_id, mode="negative"))
        check_duplicate(docker.task("duplicate", args.run_id, expected=1), args.run_id)
        second_id = args.run_id + "-new"
        second = check_demo(docker.task("new-id", second_id), second_id)
        docker.health()
        docker.dc(["logs", "--no-color"], "services")
        docker.dc(["images"], "images")
        summary.update(status="passed", containerIntegrationExecuted=True, firstRun=first,
                       newRun=second, negativeChecks=3, duplicateMatchesRejected=6, invalidProfilesRejected=True)
    except (AssertionError, KeyError, TypeError, ValueError, RuntimeError, OSError, subprocess.SubprocessError) as error:
        summary["status"] = "failed" if owned else "blocked"
        summary["error"] = str(error)
        print("NOT PASSED: " + str(error), file=sys.stderr)
    finally:
        if owned:
            try:
                docker.dc(["logs", "--no-color"], "final-services")
            except (RuntimeError, OSError) as error:
                summary["logCollectionError"] = str(error)
            try:
                docker.dc(["--profile", "tools", "down", "--rmi", "local", "--timeout", "10"], "down", timeout=90)
                require(not docker.resources(), "project resources remain after down")
                summary["resourcesReleased"] = True
            except (AssertionError, RuntimeError, OSError) as error:
                summary.update(status="failed", cleanupError=str(error))
        (evidence / "summary.json").write_text(json.dumps(summary, indent=2) + "\n", encoding="utf-8")
    print(json.dumps(summary, indent=2))
    return 0 if summary["status"] == "passed" else 1


if __name__ == "__main__":
    sys.exit(main())