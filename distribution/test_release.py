"""Synthetic metadata tests only: no push, Release creation or real GHCR digest claim."""
import copy
import json
import os
from pathlib import Path
import subprocess
import tempfile
import unittest
from unittest.mock import patch
from urllib.error import HTTPError
import zipfile

import release
import verify_package


def synthetic_lock(test_only=True):
    lock = json.loads((release.ROOT / "distribution/images-test.json").read_text())
    lock["testOnly"] = test_only
    return lock


def passing_run():
    return {"id": 7, "head_sha": "0" * 40, "head_branch": "main", "event": "push",
            "status": "completed", "conclusion": "success"}


def passing_jobs():
    return [{"name": name, "status": "completed", "conclusion": "success"} for name in release.REQUIRED_JOBS]


class ReleaseTests(unittest.TestCase):
    def test_stable_versions_only(self):
        self.assertEqual(release.version("v1.0.0"), "v1.0.0")
        for bad in ["main", "v1.0", "v1.0.0-rc1", "v01.0.0", "v1.0.0;echo bad", "../v1.0.0"]:
            with self.subTest(bad=bad), self.assertRaises(ValueError):
                release.version(bad)

    def test_incomplete_or_wrong_registry_platform_digest_rejected(self):
        for key in ["partial", "image", "platform", "digest", "revision"]:
            lock = synthetic_lock()
            if key == "partial":del lock["images"]["simulator"]
            if key == "image":lock["images"]["simulator"]["image"] = "example.org/other"
            if key == "platform":lock["platform"] = "linux/arm64"
            if key == "digest":lock["images"]["simulator"]["digest"] = "sha256:fake"
            if key == "revision":lock["revision"] = "main"
            with self.subTest(key=key), self.assertRaises(ValueError):
                release.validate_lock(lock, True)

    def test_test_data_cannot_be_packaged_as_real_delivery(self):
        with tempfile.TemporaryDirectory() as folder:
            lock = Path(folder) / "test.json"
            lock.write_text(json.dumps(synthetic_lock()))
            with self.assertRaises(ValueError):release.pack(lock, Path(folder) / "out")
            path = release.pack(lock, Path(folder) / "out", True)
            self.assertTrue(path.name.startswith("GameStats-PRUEBA-"))
            with zipfile.ZipFile(path) as archive:
                self.assertIn("NO ENTREGAR", archive.read(archive.namelist()[2]).decode())

    def test_zip_is_deterministic_and_refuses_overwrite(self):
        with tempfile.TemporaryDirectory() as folder:
            lock = Path(folder) / "test.json"
            lock.write_text(json.dumps(synthetic_lock()))
            first = release.pack(lock, Path(folder) / "a", True)
            second = release.pack(lock, Path(folder) / "b", True)
            self.assertEqual(first.read_bytes(), second.read_bytes())
            self.assertTrue((first.parent / "SHA256SUMS.txt").is_file())
            with self.assertRaises(ValueError):release.pack(lock, first.parent, True)

    def test_exact_main_ci_and_all_jobs_required(self):
        self.assertEqual(release.approved_ci([passing_run()], lambda _: passing_jobs(), "0"*40), 7)
        for key, value in [("head_sha","1"*40), ("head_branch","feature"), ("event","pull_request"),
                           ("status","in_progress"), ("conclusion","failure")]:
            run = passing_run();run[key] = value
            with self.subTest(key=key), self.assertRaises(ValueError):
                release.approved_ci([run], lambda _: passing_jobs(), "0"*40)
        for jobs in [passing_jobs()[:-1], [{**j,"conclusion":"skipped"} for j in passing_jobs()]]:
            with self.assertRaises(ValueError):release.approved_ci([passing_run()], lambda _: jobs, "0"*40)

    def test_existing_versions_refused(self):
        rows = [{"metadata":{"container":{"tags":["v1.0.0"]}}}]
        with patch.object(release,"pages",return_value=iter(rows)), self.assertRaisesRegex(ValueError,"already published"):
            release.check_unused("v1.0.0")

    def test_package_permission_errors_not_treated_as_absent(self):
        for code in [401,403,500]:
            with patch.object(release,"pages",side_effect=HTTPError("url",code,"error",{},None)):
                with self.assertRaises(ValueError):release.check_unused("v1.0.0")
        with patch.object(release,"pages",side_effect=HTTPError("url",404,"absent",{},None)):
            release.check_unused("v1.0.0")

    def test_all_builds_precede_push_and_effective_digests_are_recorded(self):
        with tempfile.TemporaryDirectory() as folder:
            record = Path(folder)/"approved.json"
            record.write_text(json.dumps({"version":"v1.0.0","revision":"0"*40,"ciRunId":7}))
            lock = Path(folder)/"images.json"
            def image_inspect(args, **kwargs):
                image = args[-1].rsplit(":",1)[0]
                return json.dumps([{"Os":"linux","Architecture":"amd64",
                                    "RepoDigests":[image+"@sha256:"+"a"*64]}])
            with patch.object(release,"release_context",return_value=({},"v1.0.0")), \
                    patch.object(release,"git",return_value="0"*40), \
                    patch.object(release,"require_unused_release"), \
                    patch.object(release,"check_unused") as unused, \
                    patch.object(release.subprocess,"run") as command, \
                    patch.object(release.subprocess,"check_output",side_effect=image_inspect):
                release.publish(record,lock)
                self.assertEqual([c.args[0][1] for c in command.call_args_list],
                                 ["build","build","build","push","push","push"])
                self.assertEqual(unused.call_count,2)
            data = release.validate_lock(json.loads(lock.read_text()))
            self.assertEqual(data["ciRunId"],7)
            self.assertTrue(all(row["digest"]=="sha256:"+"a"*64 for row in data["images"].values()))

    def test_build_failure_does_not_push_any_image(self):
        with tempfile.TemporaryDirectory() as folder:
            record=Path(folder)/"approved.json"
            record.write_text(json.dumps({"version":"v1.0.0","revision":"0"*40,"ciRunId":7}))
            with patch.object(release,"release_context",return_value=({},"v1.0.0")), \
                    patch.object(release,"git",return_value="0"*40), \
                    patch.object(release,"require_unused_release"), \
                    patch.object(release,"check_unused"), \
                    patch.object(release.subprocess,"run",side_effect=subprocess.CalledProcessError(1,"build")) as command:
                with self.assertRaises(subprocess.CalledProcessError):
                    release.publish(record,Path(folder)/"images.json")
                self.assertEqual(command.call_count,1)
                self.assertEqual(command.call_args.args[0][1],"build")

    def test_publish_refuses_local_or_pr_context(self):
        with patch.dict(os.environ,{"GITHUB_EVENT_NAME":"pull_request"}):
            with self.assertRaises(ValueError):release.release_context()
        with tempfile.TemporaryDirectory() as folder:
            event = Path(folder)/"event.json"
            data = {"inputs":{"version":"v1.0.0","reviewed_commit":"0"*40,"confirmation":"PUBLICAR v1.0.0"}}
            event.write_text(json.dumps(data))
            env={"GITHUB_EVENT_NAME":"workflow_dispatch","GITHUB_REPOSITORY":release.REPOSITORY,
                 "GITHUB_EVENT_PATH":str(event),"GITHUB_REF":"refs/heads/main","GITHUB_SHA":"0"*40}
            with patch.dict(os.environ,env):
                self.assertEqual(release.release_context()[1],"v1.0.0")
                for key,value in [("GITHUB_REF","refs/heads/feature"),("GITHUB_SHA","1"*40)]:
                    with patch.dict(os.environ,{key:value}), self.assertRaises(ValueError):release.release_context()
                data["inputs"]["confirmation"]="yes";event.write_text(json.dumps(data))
                with self.assertRaises(ValueError):release.release_context()


class PackageTests(unittest.TestCase):
    def test_zip_has_only_documents_manifest_and_pinned_compose(self):
        # Synthetic production-shaped data, temporary unit-test ZIP only; never pulled or delivered.
        with tempfile.TemporaryDirectory() as folder:
            lock = Path(folder)/"synthetic.json";lock.write_text(json.dumps(synthetic_lock(False)))
            archive = release.pack(lock, Path(folder)/"zip")
            target, data = verify_package.extract_package(archive, Path(folder)/"isolated")
            self.assertEqual({p.name for p in target.iterdir()}, verify_package.EXPECTED_FILES)
            text = (target/"compose.yaml").read_text()
            self.assertNotIn("build:",text)
            for service in release.SERVICES:self.assertIn(release.reference(data,service),text)

    def test_compose_configuration_without_sources(self):
        result = subprocess.run(["docker","compose","--file",str(release.ROOT/"distribution/compose.yaml"),
                                 "--profile","tools","config","--format","json"],
                                capture_output=True,text=True,check=True,timeout=20)
        config = json.loads(result.stdout)
        lock = synthetic_lock(False)
        for service in release.SERVICES:config["services"][service]["image"] = release.reference(lock,service)
        verify_package.validate_config(config,lock)
        bad = copy.deepcopy(config);bad["services"]["simulator"]["volumes"] = [{"source":"/src"}]
        with self.assertRaises(ValueError):verify_package.validate_config(bad,lock)

    def test_test_zip_rejected_by_real_distribution_verifier(self):
        with tempfile.TemporaryDirectory() as folder:
            lock = Path(folder)/"test.json";lock.write_text(json.dumps(synthetic_lock()))
            archive = release.pack(lock, Path(folder)/"zip", True)
            with self.assertRaises(ValueError):verify_package.extract_package(archive, Path(folder)/"isolated")

    def test_unsafe_or_extra_zip_files_rejected(self):
        with tempfile.TemporaryDirectory() as folder:
            archive = Path(folder)/"bad.zip"
            with zipfile.ZipFile(archive,"w") as z:z.writestr("../compose.yaml","bad")
            with self.assertRaises(ValueError):verify_package.extract_package(archive,Path(folder)/"target")

    def test_anonymous_runtime_ignores_host_auth(self):
        with tempfile.TemporaryDirectory() as folder:
            with patch.dict(os.environ,{"DOCKER_CONFIG":"host-auth","DOCKER_AUTH_CONFIG":"secret"}):
                runtime = verify_package.Runtime(Path(folder),"test-project",Path(folder),Path(folder)/"empty")
            self.assertNotEqual(runtime.env["DOCKER_CONFIG"],"host-auth")
            self.assertNotIn("DOCKER_AUTH_CONFIG",runtime.env)

    def test_does_not_remove_foreign_named_container(self):
        from types import SimpleNamespace
        with tempfile.TemporaryDirectory() as folder:
            runtime = verify_package.Runtime(Path(folder),"ours",Path(folder),Path(folder))
            with patch.object(verify_package.subprocess,"run",return_value=SimpleNamespace(returncode=0,stdout="other")), \
                    patch.object(runtime,"call") as call:
                with self.assertRaises(ValueError):runtime.remove_task("foreign")
                call.assert_not_called()


class FinalizationTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary.cleanup)
        self.folder = Path(self.temporary.name)
        self.lock = synthetic_lock(False)
        self.lock["ciRunId"] = 7
        (self.folder/"images.json").write_text(json.dumps(self.lock))
        release.pack(self.folder/"images.json",self.folder/"package")
        (self.folder/"verification").mkdir()
        self.summary = {"status":"passed","anonymousPulls":True,"distributionIntegrationExecuted":True,
                        "resourcesReleased":True,"version":"v1.0.0","revision":"0"*40}
        (self.folder/"verification/summary.json").write_text(json.dumps(self.summary))
        self.names = {"GameStats-Entrega-v1.0.0.zip","SHA256SUMS.txt","images.json"}
        self.draft = {"id":1,"draft":True,"prerelease":False,"target_commitish":"0"*40,"assets":[],
                      "body":"GameStats academic distribution\nPublication run: 123\nRevision: "+"0"*40+"\n"}
        self.published = dict(self.draft,draft=False,immutable=True,id=1,html_url="https://example.test/release",
                              assets=[{"name":name} for name in self.names])
        for active in [patch.dict(os.environ,{"GITHUB_RUN_ID":"123","APPROVED_CI":"7"}),
                       patch.object(release,"release_context",return_value=({"reviewed_commit":"0"*40},"v1.0.0")),
                       patch.object(release,"remote_tag_revision",return_value="0"*40)]:
            active.start();self.addCleanup(active.stop)

    def test_draft_creation_uses_returned_id_even_if_listing_lags(self):
        new_draft = dict(self.draft, tag_name="v1.0.0", assets=[])
        downloaded = []
        with patch.object(release, "find_release", return_value=None) as finder, \
             patch.object(release, "optional_api", return_value=None), \
             patch.object(release, "create_draft_release", return_value=new_draft) as create, \
             patch.object(release, "upload_asset", side_effect=lambda rid, path: {
                 "id": len(downloaded) + 10, "name": path.name, "state": "uploaded"
             }) as upload, \
             patch.object(release, "asset_matches", side_effect=lambda asset, path: asset["name"] == path.name), \
             patch.object(release, "publish_draft", return_value=self.published) as publish, \
             patch.object(release, "api", return_value=dict(self.published, tag_name="v1.0.0")):
            release.finalize(self.folder)
        finder.assert_called_once_with("v1.0.0")
        create.assert_called_once()
        self.assertEqual(upload.call_count, 3)
        self.assertTrue(all(c.args[0] == 1 for c in upload.call_args_list))
        publish.assert_called_once_with(1, "0" * 40)

    def test_resume_partial_draft_uploads_only_missing_assets(self):
        draft = dict(self.draft, assets=[{"id": 11, "name": "images.json"}])
        with patch.object(release, "find_release", return_value=draft), \
             patch.object(release, "asset_matches", return_value=True), \
             patch.object(release, "upload_asset", side_effect=lambda rid, path: {
                 "id": 12, "name": path.name, "state": "uploaded"
             }) as upload, \
             patch.object(release, "publish_draft") as publish, \
             patch.object(release, "api", return_value=dict(self.published, tag_name="v1.0.0")):
            release.finalize(self.folder)
        self.assertEqual(upload.call_count, 2)
        publish.assert_called_once_with(1, "0" * 40)

    def test_published_immutable_release_retry_is_read_only(self):
        with patch.object(release, "find_release", return_value=self.published), \
             patch.object(release, "api", return_value=dict(self.published, tag_name="v1.0.0")), \
             patch.object(release, "asset_matches", return_value=True), \
             patch.object(release, "upload_asset") as upload, \
             patch.object(release, "publish_draft") as publish:
            release.finalize(self.folder)
        upload.assert_not_called()
        publish.assert_not_called()

    def test_mismatched_existing_asset_is_never_overwritten(self):
        draft = dict(self.draft, assets=[{"id": 11, "name": name} for name in self.names])
        with patch.object(release, "find_release", return_value=draft), \
             patch.object(release, "asset_matches", return_value=False), \
             patch.object(release, "upload_asset") as upload, \
             patch.object(release, "publish_draft") as publish:
            with self.assertRaisesRegex(ValueError, "never overwrite"):
                release.finalize(self.folder)
        upload.assert_not_called()
        publish.assert_not_called()

    def test_unexpected_asset_refuses_publication(self):
        draft = dict(self.draft, assets=[{"id": 11, "name": "unexpected.bin"}])
        with patch.object(release, "find_release", return_value=draft), \
             patch.object(release, "upload_asset") as upload, \
             patch.object(release, "publish_draft") as publish:
            with self.assertRaisesRegex(ValueError, "Unexpected Release assets"):
                release.finalize(self.folder)
        upload.assert_not_called()
        publish.assert_not_called()

    def test_anonymous_verification_required_before_creating_release(self):
        self.summary["anonymousPulls"]=False
        (self.folder/"verification/summary.json").write_text(json.dumps(self.summary))
        with patch.object(release,"gh") as command,patch.object(release,"find_release") as remote:
            with self.assertRaises(ValueError):release.finalize(self.folder)
        command.assert_not_called();remote.assert_not_called()

    def test_existing_tag_or_release_rejected_before_images(self):
        for existing_release,existing_tag in [({},None),(None,{})]:
            with patch.object(release,"find_release",return_value=existing_release), \
                    patch.object(release,"optional_api",return_value=existing_tag),self.assertRaises(ValueError):
                release.require_unused_release("v1.0.0")

    def test_draft_lookup_uses_release_listing_and_rejects_ambiguity(self):
        draft=dict(self.draft,tag_name="v1.0.0")
        with patch.object(release,"pages",return_value=iter([draft])):
            self.assertTrue(release.find_release("v1.0.0")["draft"])
        with patch.object(release,"pages",return_value=iter([draft,draft])),self.assertRaises(ValueError):
            release.find_release("v1.0.0")

    def test_rerun_consumers_keep_successful_producer_artifacts(self):
        workflow=(release.ROOT/".github/workflows/academic-release.yml").read_text()
        self.assertIn("name: ${{ needs.images.outputs.artifactName }}",workflow)
        self.assertIn("name: ${{ needs.evaluation.outputs.artifactName }}",workflow)
        self.assertEqual(workflow.count("packages: write"),1)
        self.assertEqual(workflow.count("contents: write"),1)
        self.assertIn("workflow_dispatch:",workflow)
        self.assertNotIn("types: [published]",workflow)


if __name__ == "__main__":
    unittest.main()
