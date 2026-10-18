"""Static/config and verification-procedure tests. No Docker engine required."""
import copy
import json
import os
from pathlib import Path
import re
import subprocess
import sys
import tempfile
from types import SimpleNamespace
import unittest
from unittest.mock import patch

sys.path.insert(0, str(Path(__file__).resolve().parent))
from verify import ROOT, COUNTS, Docker, check_demo, check_duplicate, check_negative


def demo_report(run="test"):
    matches = []
    for (game, name), count in COUNTS.items():
        match_id = run + "-" + name
        results = [{"gameId": game, "matchId": match_id, "state": state,
                    "metrics": [], "patterns": []} for state in ["open"] * 8 + ["closed"] * 2]
        matches.append({"gameId": game, "matchId": match_id, "accepted": count,
                        "queries": 10, "verified": True, "results": results})
    return {"matches": matches, "peakRequests": 6}


class ConfigTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        env = dict(os.environ, GO_PORT="8080", GO_BIND_ADDRESS="127.0.0.1")
        result = subprocess.run(
            ["docker", "compose", "--file", str(ROOT / "compose.yaml"),
             "--profile", "tools", "config", "--format", "json"],
            cwd=ROOT, env=env, check=True, capture_output=True, text=True,
            encoding="utf-8", timeout=20)
        cls.config = json.loads(result.stdout)
        cls.services = cls.config["services"]

    def test_three_independent_services(self):
        self.assertEqual(set(self.services), {"go-ingestion", "scala-analysis", "simulator"})
        for name, service in self.services.items():
            self.assertEqual(Path(service["build"]["context"]).resolve(), ROOT)
            self.assertEqual(service["build"]["dockerfile"], name + "/Dockerfile")

    def test_network_and_public_port(self):
        go, scala = self.services["go-ingestion"], self.services["scala-analysis"]
        self.assertEqual(go["environment"]["SCALA_SERVICE_URL"], "http://scala-analysis:9000")
        self.assertEqual(scala["environment"], {
            "SCALA_HOST": "0.0.0.0", "SCALA_PORT": "9000", "SCALA_PROFILES_DIR": "/app/profiles"})
        self.assertFalse(scala.get("ports"))
        self.assertEqual(go["ports"][0]["host_ip"], "127.0.0.1")
        self.assertEqual(str(go["ports"][0]["published"]), "8080")
        self.assertEqual(go["ports"][0]["target"], 8080)

    def test_simulator_command_override_preserves_internal_url(self):
        sim = self.services["simulator"]
        self.assertEqual(sim["profiles"], ["tools"])
        self.assertEqual(sim["entrypoint"], [
            "/usr/local/bin/simulator", "--go-url", "http://go-ingestion:8080"])
        self.assertIn("--verify=true", sim["command"])
        self.assertEqual(sim["restart"], "no")

    def test_health_readiness(self):
        for name in ["go-ingestion", "scala-analysis"]:
            self.assertEqual(self.services["simulator"]["depends_on"][name]["condition"],
                             "service_healthy")
        self.assertEqual(self.services["go-ingestion"]["depends_on"]["scala-analysis"]["condition"],
                         "service_healthy")
        for name, port in [("go-ingestion", "8080"), ("scala-analysis", "9000")]:
            text = (ROOT / name / "Dockerfile").read_text()
            self.assertIn("HEALTHCHECK", text)
            self.assertIn("command -v wget", text)
            self.assertIn("/health", text)
            self.assertIn("EXPOSE " + port, text)

    def test_minimal_privileges(self):
        for name, service in self.services.items():
            self.assertNotEqual(service.get("network_mode"), "host")
            self.assertFalse(service.get("privileged", False))
            self.assertIn("ALL", service["cap_drop"])
            self.assertIn("no-new-privileges:true", service["security_opt"])
            self.assertIn("USER 10001:10001", (ROOT / name / "Dockerfile").read_text())
        for name in ["go-ingestion", "scala-analysis"]:
            self.assertTrue(self.services[name]["read_only"])
        self.assertFalse(self.config.get("volumes"))

    def test_bases_pinned_and_multistage(self):
        for name in self.services:
            text = (ROOT / name / "Dockerfile").read_text()
            bases = re.findall(r"^FROM (\S+) AS \w+$", text, re.M)
            self.assertEqual(len(bases), 2)
            for base in bases:
                self.assertRegex(base, r"^[^:]+:[^@]+@sha256:[a-f0-9]{64}$")
                self.assertNotIn(":latest", base)
            self.assertIn("COPY --from=build", text)
        scala = (ROOT / "scala-analysis/Dockerfile").read_text()
        self.assertIn("21.0.12_8-jdk", scala)
        self.assertIn("21.0.12_8-jre", scala)
        self.assertIn('"/app/app.jar:/app/lib/*"', scala)

    def test_launcher_matches_approved_wrapper(self):
        wrapper = (ROOT / "scala-analysis/sbtw.ps1").read_text()
        expected = re.search(r"taskHash='([A-F0-9]+)'", wrapper)[1].lower()
        dockerfile = (ROOT / "scala-analysis/Dockerfile").read_text()
        actual = re.search(r'echo "([a-f0-9]+)  /tmp/sbt-launch.jar"', dockerfile)[1]
        self.assertEqual(actual, expected)
        self.assertIn("sbt-launch/1.11.7/sbt-launch-1.11.7.jar", dockerfile)
        self.assertIn("sbt.version=1.11.7", (ROOT / "scala-analysis/project/build.properties").read_text())

    def test_context_denies_local_data_and_secrets(self):
        rules = (ROOT / ".dockerignore").read_text().splitlines()
        self.assertEqual([line for line in rules if line and not line.startswith("#")][0], "**")
        for rule in ["**/.git", "**/.env", "**/.env.*", "**/.cache", "**/.tools",
                     "**/target", "**/evidence", "**/bin", "**/*.key", "**/*.pem",
                     "**/.idea", "**/.vscode"]:
            self.assertIn(rule, rules)
        self.assertIn("!docs/examples/profiles/*.json", rules)
        self.assertNotIn("!docs/fixtures/**", rules)


class ProcedureTests(unittest.TestCase):
    def test_successful_demo_counts(self):
        result = check_demo(demo_report(), "test")
        self.assertEqual(result["eventsAccepted"], 50)
        self.assertEqual(result["resultsChecked"], 60)
        self.assertEqual(result["openResults"], 48)
        self.assertEqual(result["closedResults"], 12)

    def test_demo_mismatch_rejected(self):
        for change in ["identity", "accepted", "queries", "verified", "state", "repeat", "peak"]:
            with self.subTest(change=change):
                report = copy.deepcopy(demo_report())
                match = report["matches"][0]
                if change == "identity": match["matchId"] = "other"
                if change == "accepted": match["accepted"] = 0
                if change == "queries": match["queries"] = 0
                if change == "verified": match["verified"] = False
                if change == "state": match["results"][0]["state"] = "closed"
                if change == "repeat": match["results"][1]["metrics"] = [{"value": 999}]
                if change == "peak": report["peakRequests"] = 1
                with self.assertRaises(AssertionError):
                    check_demo(report, "test")

    def test_duplicate_requires_expected_http_reason(self):
        report = demo_report()
        for match in report["matches"]:
            match.update(accepted=0, queries=0, verified=False, results=[],
                         error="HTTP 409: eventId already exists in this match")
        check_duplicate(report, "test")
        for error in ["HTTP 503: upstream down", "HTTP 409: unrelated error", "timeout"]:
            with self.subTest(error=error):
                wrong = copy.deepcopy(report)
                wrong["matches"][0]["error"] = error
                with self.assertRaises(AssertionError):
                    check_duplicate(wrong, "test")

    def test_negative_cases_require_all_three_successes(self):
        report = {"checks": [{"name": name, "passed": True} for name in [
            "envelope_rejected_without_mutation", "invalid_domain_independent_pattern",
            "unknown_game_empty_results"]]}
        check_negative(report)
        report["checks"][1]["passed"] = False
        with self.assertRaises(AssertionError):
            check_negative(report)


class OwnershipTests(unittest.TestCase):
    def test_foreign_named_container_is_never_removed(self):
        with tempfile.TemporaryDirectory() as folder:
            docker = Docker("ours", Path(folder))
            with patch("verify.subprocess.run", return_value=SimpleNamespace(
                    returncode=0, stdout="foreign\n")), patch.object(docker, "call") as call:
                with self.assertRaises(AssertionError):
                    docker.remove_owned_task("some-name")
                call.assert_not_called()

    def test_only_matching_owner_can_be_removed(self):
        with tempfile.TemporaryDirectory() as folder:
            docker = Docker("ours", Path(folder))
            with patch("verify.subprocess.run", return_value=SimpleNamespace(
                    returncode=0, stdout="ours\n")), patch.object(docker, "call") as call:
                docker.remove_owned_task("ours-task")
                self.assertEqual(call.call_args.args[0], ["rm", "--force", "ours-task"])

    def test_missing_container_does_not_trigger_delete(self):
        with tempfile.TemporaryDirectory() as folder:
            docker = Docker("ours", Path(folder))
            with patch("verify.subprocess.run", return_value=SimpleNamespace(
                    returncode=1, stdout="")), patch.object(docker, "call") as call:
                docker.remove_owned_task("missing")
                call.assert_not_called()

    def test_wrong_profile_failure_is_not_success(self):
        with tempfile.TemporaryDirectory() as folder:
            docker = Docker("ours", Path(folder))
            with patch.object(docker, "dc", return_value="Cannot start analysis service: invalid_port at SCALA_PORT"), \
                    patch.object(docker, "remove_owned_task") as remove:
                with self.assertRaises(AssertionError):
                    docker.bad_profiles()
                remove.assert_called_once_with("ours-bad-profiles")


if __name__ == "__main__":
    unittest.main()
