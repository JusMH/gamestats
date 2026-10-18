"""Verify the evaluation ZIP with anonymous pulls, without building source.

Maintainer tool: Python + Docker only. Evaluators need only Docker Desktop.
Extracts to an independent temporary directory and never removes shared images.
"""
import argparse
import json
import os
from pathlib import Path, PurePosixPath
import re
import subprocess
import sys
import tempfile
import uuid
import zipfile

sys.path.insert(0, str(Path(__file__).resolve().parents[1] / "docker"))
from verify import check_demo, check_duplicate, check_negative
from release import SERVICES, reference, validate_lock, require

EXPECTED_FILES = {"compose.yaml", "INSTALACION.md", "GUIA-DE-EVALUACION.md", "IMAGENES.json"}


def extract_package(package, target):
    with zipfile.ZipFile(package) as archive:
        require(len(archive.infolist()) == len(EXPECTED_FILES), "Unexpected ZIP contents")
        require(sum(x.file_size for x in archive.infolist()) < 1024 * 1024, "Oversized evaluation ZIP")
        names = [PurePosixPath(x.filename) for x in archive.infolist()]
        require(all(len(n.parts) == 2 and not n.is_absolute() and
                    '..' not in n.parts and '\\' not in str(n) for n in names), "Unsafe ZIP path")
        require(len({n.parts[0] for n in names}) == 1 and
                {n.name for n in names} == EXPECTED_FILES, "Unexpected ZIP structure")
        # Read/write individual files, no symlinks or arbitrary archive extraction.
        for info in archive.infolist():
            destination = Path(target) / info.filename
            destination.parent.mkdir(parents=True, exist_ok=True)
            destination.write_bytes(archive.read(info))
        folder = Path(target) / names[0].parts[0]
        lock = validate_lock(json.loads((folder / "IMAGENES.json").read_text(encoding="utf-8")))
        require(folder.name == "GameStats-Entrega-" + lock["version"], "Wrong distribution name")
        return folder, lock


def validate_config(config, lock):
    services = config["services"]
    require(set(services) == set(SERVICES), "Expected three separate services")
    require(not any(config.get(k) for k in ["volumes", "configs", "secrets"]), "Unexpected file dependencies")
    for name, service in services.items():
        require(service["image"] == reference(lock, name), "Image not pinned to published lock")
        require(service.get("platform") == "linux/amd64", "Unsupported platform")
        require(not any(service.get(k) for k in ["build", "volumes", "env_file", "configs", "secrets"]),
                "Distribution must not depend on source or host files")
    require(not services["scala-analysis"].get("ports"), "Scala must stay private")
    require(services["go-ingestion"]["ports"][0]["host_ip"] == "127.0.0.1", "Go must bind loopback")


class Runtime:
    def __init__(self, folder, project, evidence, config):
        self.folder, self.project, self.evidence = folder, project, evidence
        self.env = dict(os.environ, DOCKER_CONFIG=str(config), GO_PORT="0")
        self.env.pop("DOCKER_AUTH_CONFIG", None)
        self.env.pop("COMPOSE_FILE", None)
        self.env.pop("COMPOSE_PROFILES", None)
        self.compose = ["compose", "--project-name", project, "--file", str(folder / "compose.yaml")]

    def call(self, args, name, expected=0, timeout=180):
        result = subprocess.run(["docker", *args], cwd=self.folder, env=self.env, text=True,
                                encoding="utf-8", errors="replace", stdout=subprocess.PIPE,
                                stderr=subprocess.STDOUT, timeout=timeout)
        (self.evidence / (name + ".log")).write_text(result.stdout, encoding="utf-8")
        require(result.returncode == expected, f"{name}: exit {result.returncode}, expected {expected}; inspect log")
        return result.stdout

    def dc(self, args, name, **kwargs):
        return self.call([*self.compose, *args], name, **kwargs)

    def resources(self):
        return [self.call(["ps", "-aq", "--filter", "label=com.docker.compose.project=" + self.project], "containers"),
                self.call(["network", "ls", "-q", "--filter", "label=com.docker.compose.project=" + self.project], "networks")]

    def remove_task(self, name):
        result = subprocess.run(["docker", "inspect", "--format",
                                 '{{index .Config.Labels "com.docker.compose.project"}}', name],
                                cwd=self.folder, env=self.env, capture_output=True, text=True, timeout=15)
        if result.returncode == 0:
            require(result.stdout.strip() == self.project, "Refusing to remove a foreign container")
            self.call(["rm", "--force", name], "remove-" + name)

    def task(self, kind, run_id, expected=0, mode="demo"):
        name = self.project + "-" + kind
        try:
            self.dc(["run", "--no-deps", "--name", name, "simulator", "--run-id", run_id,
                     "--mode", mode, "--verify=true", "--report", "/reports/result.json"],
                    kind, expected=expected)
            destination = self.evidence / (kind + ".json")
            self.call(["cp", name + ":/reports/result.json", str(destination)], "copy-" + kind)
            return json.loads(destination.read_text(encoding="utf-8"))
        finally:
            self.remove_task(name)


def verify(package, evidence, project):
    require(re.fullmatch(r"[a-z0-9][a-z0-9_-]{0,30}", project), "Invalid project identifier")
    evidence = Path(evidence).resolve()
    require(not evidence.exists() or not any(evidence.iterdir()), "Evidence directory must be new/empty")
    evidence.mkdir(parents=True, exist_ok=True)
    summary = {"status": "failed", "anonymousPulls": False, "distributionIntegrationExecuted": False}
    with tempfile.TemporaryDirectory(prefix="gamestats-evaluation-") as temporary:
        folder, lock = extract_package(package, temporary)
        config = Path(temporary) / "anonymous-docker"
        config.mkdir()
        (config / "config.json").write_text("{}", encoding="utf-8")
        docker = Runtime(folder, project, evidence, config)
        owned = False
        try:
            require(docker.call(["info", "--format", "{{.OSType}}"], "engine", timeout=15).strip() == "linux",
                    "Linux Docker engine required")
            require(not any(x.strip() for x in docker.resources()), "Project already exists; refusing to touch it")
            configuration = json.loads(docker.dc(["--profile", "tools", "config", "--format", "json"], "config"))
            validate_config(configuration, lock)
            # Always contacts registry, including when layers are cached. No login,
            # no host credential helper: fresh empty DOCKER_CONFIG for this process.
            docker.dc(["--profile", "tools", "pull", "--policy", "always"], "anonymous-pull", timeout=900)
            summary["anonymousPulls"] = True
            owned = True
            docker.dc(["up", "-d", "--wait", "--wait-timeout", "120"], "up")
            ids = docker.dc(["ps", "-q", "go-ingestion", "scala-analysis"], "services").split()
            require(len(ids) == 2, "Expected two running services")
            containers = json.loads(docker.call(["inspect", *ids], "health"))
            require(all(c["State"]["Health"]["Status"] == "healthy" for c in containers), "Unhealthy service")
            summary["distributionIntegrationExecuted"] = True
            first = check_demo(docker.task("demo", project), project)
            check_negative(docker.task("negative", project, mode="negative"))
            check_duplicate(docker.task("duplicate", project, expected=1), project)
            second = check_demo(docker.task("new-id", project + "-new"), project + "-new")
            summary.update(status="passed", version=lock["version"], revision=lock["revision"],
                           firstRun=first, newRun=second, negativeChecks=3, duplicateMatchesRejected=6)
        except (ValueError, AssertionError, KeyError, OSError, subprocess.SubprocessError) as error:
            summary["error"] = str(error)
        finally:
            if owned:
                try:
                    docker.dc(["logs", "--no-color"], "services-final")
                except (ValueError, OSError, subprocess.SubprocessError) as error:
                    summary["logError"] = str(error)
                try:
                    docker.dc(["--profile", "tools", "down", "--timeout", "10"], "down")
                    require(not any(x.strip() for x in docker.resources()), "Project resources remain")
                    summary["resourcesReleased"] = True
                except (ValueError, OSError, subprocess.SubprocessError) as error:
                    summary.update(status="failed", cleanupError=str(error))
            (evidence / "summary.json").write_text(json.dumps(summary, indent=2) + "\n", encoding="utf-8")
    print(json.dumps(summary, indent=2))
    return 0 if summary["status"] == "passed" else 1


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--zip", required=True, type=Path)
    parser.add_argument("--evidence", required=True, type=Path)
    parser.add_argument("--project", default="evaluation-" + uuid.uuid4().hex[:8])
    args = parser.parse_args()
    return verify(args.zip, args.evidence, args.project)


if __name__ == "__main__":
    sys.exit(main())
