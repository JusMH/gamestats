"""Release controls and reproducible evaluation ZIP. Python 3.10+, stdlib only.

Only publish writes to GHCR; only finalize creates/publishes a Release. Both
require an explicitly approved main workflow_dispatch. Unit tests and pack never publish, create tags or change visibility.
"""
import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import subprocess
import tempfile
import zipfile
from urllib.error import HTTPError
from urllib.parse import quote, urlencode
from urllib.request import Request, urlopen

ROOT = Path(__file__).resolve().parents[1]
REPOSITORY = "JusMH/gamestats"
SERVICES = ("go-ingestion", "scala-analysis", "simulator")
REQUIRED_JOBS = {
    "Go / go-ingestion", "Go / simulator",
    "Scala / engine, HTTP and packaging",
    "Contracts / fixtures, infrastructure and workflow",
    "Docker / real multigame integration", "CI result",
}


def require(condition, message):
    if not condition:
        raise ValueError(message)


def version(value):
    require(re.fullmatch(r"v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)", value),
            "Expected stable version vMAJOR.MINOR.PATCH (initially v1.0.0)")
    return value


def image_name(service):
    require(service in SERVICES, "Unknown component")
    return "ghcr.io/jusmh/gamestats-" + service


def reference(lock, service):
    return image_name(service) + ":" + lock["version"] + "@" + lock["images"][service]["digest"]


def validate_lock(lock, test_data=False):
    require(lock.get("schemaVersion") == 1, "Unsupported image lock schema")
    version(lock["version"])
    require(lock.get("repository") == REPOSITORY, "Wrong source repository")
    require(re.fullmatch(r"[a-f0-9]{40}", lock["revision"]), "Invalid source revision")
    require(lock.get("platform") == "linux/amd64", "Only linux/amd64 verified")
    require(bool(lock.get("testOnly", False)) == test_data, "Test data requires explicit --test-data")
    require(set(lock["images"]) == set(SERVICES), "Expected all three images")
    for service in SERVICES:
        row = lock["images"][service]
        require(row.get("image") == image_name(service), "Unexpected GHCR image name")
        require(re.fullmatch(r"sha256:[a-f0-9]{64}", row["digest"]), "Invalid image digest")
    return lock


def pack(lock_path, output, test_data=False):
    lock = validate_lock(json.loads(Path(lock_path).read_text(encoding="utf-8")), test_data)
    prefix = "GameStats-PRUEBA-" if test_data else "GameStats-Entrega-"
    name = prefix + lock["version"]
    output = Path(output)
    output.mkdir(parents=True, exist_ok=True)
    target, sums = output / (name + ".zip"), output / "SHA256SUMS.txt"
    require(not target.exists() and not sums.exists(), "Output already exists; refusing overwrite")
    template = (ROOT / "distribution/compose.yaml").read_text(encoding="utf-8")
    template = template.replace("# Plantilla prevista: Imagenes aun NO publicadas. El ZIP generado fija digests reales.",
                                "# " + name + "; imagenes fijadas por digest, sin compilacion.")
    for service in SERVICES:
        original = image_name(service) + ":v1.0.0"
        require(template.count(original) == 1, "Compose template reference missing or duplicated")
        template = template.replace(original, reference(lock, service))
    files = {"compose.yaml": template,
             "IMAGENES.json": json.dumps(lock, indent=2, sort_keys=True) + "\n"}
    for manual in ["INSTALACION.md", "GUIA-DE-EVALUACION.md"]:
        text = (ROOT / "distribution" / manual).read_text(encoding="utf-8")
        files[manual] = text.replace("{{VERSION}}", lock["version"])
        if test_data:
            files[manual] = "# PRUEBA LOCAL: digests ficticios, NO ENTREGAR\n\n" + files[manual]
    # Stable paths, timestamp, order and file mode: same inputs yield same ZIP.
    with zipfile.ZipFile(target, "x", compression=zipfile.ZIP_DEFLATED) as archive:
        for filename, text in sorted(files.items()):
            info = zipfile.ZipInfo(name + "/" + filename, date_time=(2026, 1, 1, 0, 0, 0))
            info.compress_type = zipfile.ZIP_DEFLATED
            info.external_attr = 0o100644 << 16
            archive.writestr(info, text.encode("utf-8"))
    sums.write_text(hashlib.sha256(target.read_bytes()).hexdigest() + "  " + target.name + "\n",
                    encoding="utf-8")
    return target


def api(path):
    headers = {"Accept": "application/vnd.github+json", "User-Agent": "gamestats-release",
               "X-GitHub-Api-Version": "2022-11-28"}
    token = os.environ.get("GH_TOKEN")
    if token:
        headers["Authorization"] = "Bearer " + token
    with urlopen(Request("https://api.github.com/" + path, headers=headers), timeout=30) as response:
        return json.load(response)


def pages(path, key=None):
    for page in range(1, 101):
        data = api(path + ("&" if "?" in path else "?") + f"per_page=100&page={page}")
        rows = data[key] if key else data
        yield from rows
        if len(rows) < 100:
            return
    raise ValueError("Pagination limit exceeded; refusing incomplete verification")


def approved_ci(runs, jobs_for, revision):
    for run in runs:
        if (run.get("head_sha") == revision and run.get("head_branch") == "main" and
                run.get("event") == "push" and run.get("status") == "completed" and
                run.get("conclusion") == "success"):
            jobs = list(jobs_for(run["id"]))
            if (REQUIRED_JOBS <= {j["name"] for j in jobs} and
                    all(j.get("status") == "completed" and j.get("conclusion") == "success" for j in jobs)):
                return run["id"]
    raise ValueError("No successful complete main-push CI for the exact release commit; wait for CI")


def release_context():
    require(os.environ.get("GITHUB_EVENT_NAME") == "workflow_dispatch", "Only manual workflow_dispatch may publish")
    require(os.environ.get("GITHUB_REPOSITORY") == REPOSITORY, "Unexpected repository")
    require(os.environ.get("GITHUB_REF") == "refs/heads/main", "Run publication from main only")
    event = json.loads(Path(os.environ["GITHUB_EVENT_PATH"]).read_text(encoding="utf-8"))
    inputs = event["inputs"]
    tag = version(inputs["version"])
    revision = inputs["reviewed_commit"]
    require(re.fullmatch(r"[a-f0-9]{40}", revision), "Reviewed commit must be a full SHA")
    require(revision == os.environ.get("GITHUB_SHA"), "main changed or wrong reviewed commit; review before dispatch")
    require(inputs.get("confirmation") == "PUBLICAR " + tag, "Explicit human confirmation required")
    return inputs, tag


def optional_api(path):
    try:
        return api(path)
    except HTTPError as error:
        if error.code == 404:
            return None
        raise


def find_release(tag):
    # The tag endpoint describes published Releases. List explicitly so drafts
    # without a remote tag are also found during recovery.
    matches = [row for row in pages(f"repos/{REPOSITORY}/releases") if row.get("tag_name") == tag]
    require(len(matches) <= 1, "Ambiguous Releases for this version; review manually")
    return matches[0] if matches else None


def require_unused_release(tag):
    require(find_release(tag) is None,
            "Release already exists; do not replace a published or unrelated draft Release")
    require(optional_api(f"repos/{REPOSITORY}/git/ref/tags/{tag}") is None,
            "Tag already exists; this flow creates the tag only during finalization")


def git(*args):
    return subprocess.check_output(["git", *args], cwd=ROOT, text=True).strip()


def gate(output):
    inputs, tag = release_context()
    revision = git("rev-parse", "HEAD")
    require(inputs["reviewed_commit"] == revision, "Checkout differs from reviewed commit")
    require_unused_release(tag)
    subprocess.run(["git", "merge-base", "--is-ancestor", revision, "origin/main"], cwd=ROOT, check=True)
    runs = pages(f"repos/{REPOSITORY}/actions/workflows/ci.yml/runs?" +
                 urlencode({"head_sha": revision, "branch": "main", "event": "push", "status": "success"}),
                 "workflow_runs")
    ci_id = approved_ci(runs, lambda run: pages(f"repos/{REPOSITORY}/actions/runs/{run}/jobs", "jobs"), revision)
    record = {"version": tag, "revision": revision, "ciRunId": ci_id}
    Path(output).write_text(json.dumps(record, indent=2) + "\n", encoding="utf-8")
    with open(os.environ["GITHUB_OUTPUT"], "a", encoding="utf-8") as out:
        for key, value in record.items():
            out.write(f"{key}={value}\n")
    print("Approved exact commit CI:", ci_id)


def check_unused(tag):
    for service in SERVICES:
        path = "users/JusMH/packages/container/" + quote("gamestats-" + service, safe="") + "/versions"
        try:
            for row in pages(path):
                require(tag not in row.get("metadata", {}).get("container", {}).get("tags", []),
                        "Version already published: " + service + ":" + tag + "; do not overwrite")
        except HTTPError as error:
            if error.code != 404:
                raise ValueError("Cannot check existing package versions: HTTP " + str(error.code)) from None
            # GHCR package does not exist yet. Push still requires package permissions.


def publish(record_path, output):
    _, tag = release_context()
    record = json.loads(Path(record_path).read_text())
    require(tag == record["version"] and git("rev-parse", "HEAD") == record["revision"],
            "Approved release revision changed")
    lock = {"schemaVersion": 1, "version": tag, "revision": record["revision"],
            "repository": REPOSITORY, "platform": "linux/amd64", "ciRunId": record["ciRunId"], "images": {}}
    require_unused_release(tag)
    check_unused(tag)
    # Build all components before pushing any version; approved Dockerfiles unchanged.
    for service in SERVICES:
        subprocess.run(["docker", "build", "--platform", "linux/amd64", "--file", service + "/Dockerfile",
                        "--tag", image_name(service) + ":" + tag,
                        "--label", "org.opencontainers.image.source=https://github.com/" + REPOSITORY,
                        "--label", "org.opencontainers.image.version=" + tag,
                        "--label", "org.opencontainers.image.revision=" + record["revision"],
                        "--label", "org.opencontainers.image.title=GameStats " + service, "."], cwd=ROOT, check=True)
    require_unused_release(tag)
    check_unused(tag)
    for service in SERVICES:
        ref = image_name(service) + ":" + tag
        subprocess.run(["docker", "push", ref], check=True)
        info = json.loads(subprocess.check_output(["docker", "image", "inspect", ref], text=True))[0]
        require(info["Os"] == "linux" and info["Architecture"] == "amd64", "Wrong image platform")
        digests = [r.split("@", 1)[1] for r in info["RepoDigests"] if r.startswith(image_name(service) + "@")]
        require(len(digests) == 1, "Cannot determine actual published digest")
        lock["images"][service] = {"image": image_name(service), "digest": digests[0]}
        # Preserve partial provenance if a subsequent push fails; never package an incomplete lock.
        Path(output).write_text(json.dumps(lock, indent=2) + "\n", encoding="utf-8")
    validate_lock(lock)



def gh(*args):
    subprocess.run(["gh", *args, "--repo", REPOSITORY], check=True)


def remote_tag_revision(tag):
    row = api(f"repos/{REPOSITORY}/git/ref/tags/{tag}")["object"]
    for _ in range(5):
        if row["type"] == "commit":
            return row["sha"]
        require(row["type"] == "tag", "Unexpected tag object")
        row = api(f"repos/{REPOSITORY}/git/tags/{row['sha']}")["object"]
    raise ValueError("Tag indirection limit exceeded")


def asset_matches(tag, path):
    with tempfile.TemporaryDirectory(prefix="gamestats-release-asset-") as temporary:
        gh("release", "download", tag, "--pattern", path.name, "--dir", temporary)
        return hashlib.sha256((Path(temporary) / path.name).read_bytes()).digest() == hashlib.sha256(path.read_bytes()).digest()


def finalize(directory):
    inputs, tag = release_context()
    directory = Path(directory)
    lock = validate_lock(json.loads((directory / "images.json").read_text()))
    summary = json.loads((directory / "verification/summary.json").read_text())
    revision = inputs["reviewed_commit"]
    require(lock["version"] == tag and lock["revision"] == revision, "Manifest differs from human approval")
    require(lock["ciRunId"] == int(os.environ["APPROVED_CI"]), "Manifest differs from approved CI")
    require(summary.get("status") == "passed" and summary.get("anonymousPulls") is True and
            summary.get("distributionIntegrationExecuted") is True and summary.get("resourcesReleased") is True,
            "Successful anonymous evaluation and cleanup required")
    require(summary.get("version") == tag and summary.get("revision") == revision, "Wrong evaluation snapshot")
    archive = directory / "package" / ("GameStats-Entrega-" + tag + ".zip")
    checksum = directory / "package/SHA256SUMS.txt"
    require(checksum.read_text().strip() == hashlib.sha256(archive.read_bytes()).hexdigest() + "  " + archive.name,
            "ZIP checksum mismatch")
    with zipfile.ZipFile(archive) as packed:
        embedded = json.loads(packed.read("GameStats-Entrega-" + tag + "/IMAGENES.json"))
    require(embedded == lock, "ZIP manifest differs from effective image digests")
    files = [archive, checksum, directory / "images.json"]
    notes = f"GameStats academic distribution\nPublication run: {os.environ['GITHUB_RUN_ID']}\nRevision: {revision}\n"
    release = find_release(tag)
    if release is None:
        require(optional_api(f"repos/{REPOSITORY}/git/ref/tags/{tag}") is None, "Tag appeared before finalization")
        with tempfile.TemporaryDirectory() as temporary:
            note_file = Path(temporary) / "notes.md"
            note_file.write_text(notes, encoding="utf-8")
            gh("release", "create", tag, "--draft", "--target", revision,
               "--title", "GameStats " + tag, "--notes-file", str(note_file))
        release = find_release(tag)
    require(release is not None, "Draft creation not confirmed; retry failed job")
    require(release.get("body", "").replace("\r\n", "\n").strip() == notes.strip(),
            "Release belongs to another publication; refusing to mutate it")
    require(not release.get("prerelease"), "Unexpected prerelease")
    if release["draft"]:
        require(release["target_commitish"] == revision, "Draft targets another commit")
        tag_ref = optional_api(f"repos/{REPOSITORY}/git/ref/tags/{tag}")
        if tag_ref is not None:
            require(remote_tag_revision(tag) == revision, "Draft tag targets another commit")
    else:
        require(remote_tag_revision(tag) == revision, "Published tag targets another commit")
    existing = {row["name"] for row in release["assets"]}
    require(existing <= {path.name for path in files}, "Unexpected Release assets; review manually")
    for path in files:
        if path.name in existing:
            require(asset_matches(tag, path), "Existing asset differs; never overwrite: " + path.name)
        else:
            require(release["draft"], "Published Release is incomplete; immutable assets cannot be added")
            gh("release", "upload", tag, str(path))
    # Verify every attached byte before the irreversible publication operation.
    require(all(asset_matches(tag, path) for path in files), "Attached assets failed checksum verification")
    if release["draft"]:
        gh("release", "edit", tag, "--draft=false", "--target", revision)
    published = api(f"repos/{REPOSITORY}/releases/{release['id']}")
    require(not published["draft"] and remote_tag_revision(tag) == revision, "Release publication not confirmed")
    require({row["name"] for row in published["assets"]} == {p.name for p in files}, "Incomplete published Release")
    (directory / "release-finalization.json").write_text(json.dumps({
        "status": "passed", "version": tag, "revision": revision,
        "releaseId": published["id"], "url": published["html_url"],
        "immutable": published.get("immutable"), "assetsVerified": True,
    }, indent=2) + "\n", encoding="utf-8")

def main():
    parser = argparse.ArgumentParser(description=__doc__)
    commands = parser.add_subparsers(dest="command", required=True)
    g = commands.add_parser("gate")
    g.add_argument("--output", required=True)
    p = commands.add_parser("publish")
    p.add_argument("--record", required=True)
    p.add_argument("--output", required=True)
    f = commands.add_parser("finalize")
    f.add_argument("--directory", required=True)
    z = commands.add_parser("pack")
    z.add_argument("--lock", required=True)
    z.add_argument("--output", required=True)
    z.add_argument("--test-data", action="store_true")
    args = parser.parse_args()
    if args.command == "gate":
        gate(args.output)
    elif args.command == "publish":
        publish(args.record, args.output)
    elif args.command == "finalize":
        finalize(args.directory)
    else:
        print(pack(args.lock, args.output, args.test_data))


if __name__ == "__main__":
    main()
