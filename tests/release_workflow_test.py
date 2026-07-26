import unittest
from pathlib import Path


REPOSITORY_ROOT = Path(__file__).resolve().parents[1]
RELEASE_WORKFLOW = REPOSITORY_ROOT / ".github" / "workflows" / "release.yml"


class ReleaseWorkflowTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.workflow = RELEASE_WORKFLOW.read_text(encoding="utf-8")

    def test_builds_versioned_ulanzi_asset_on_macos(self):
        self.assertIn("- 'tests/release_workflow_test.py'", self.workflow)
        self.assertIn(
            "python3 -m unittest tests/release_workflow_test.py -v", self.workflow
        )

        job_start = self.workflow.index("  ulanzi-release:")
        publish_start = self.workflow.index("  publish:", job_start)
        ulanzi_job = self.workflow[job_start:publish_start]
        required_fragments = [
            "name: Build Ulanzi release asset",
            "runs-on: macos-latest",
            "plugins/unlanzi_d200x/command_executor",
            "run: ./build.sh",
            "life_tools_ulanzi_d200x_command_executor_${tag}.zip",
            "name: ulanzi-release-asset",
            "path: ${{ runner.temp }}/ulanzi-release/",
            "if-no-files-found: error",
        ]

        for fragment in required_fragments:
            with self.subTest(fragment=fragment):
                self.assertIn(fragment, ulanzi_job)

    def test_publish_combines_ulanzi_before_checksums_and_release_write(self):
        publish_start = self.workflow.index("  publish:")
        publish_job = self.workflow[publish_start:]
        self.assertIn("needs: [release, ulanzi-release]", publish_job)
        download_index = publish_job.index("name: Download Ulanzi release asset")
        checksum_index = publish_job.index(
            "sha256sum *.zip *.alfredworkflow > checksums.txt", download_index
        )
        release_index = publish_job.index('if gh release view "$tag"')

        self.assertLess(download_index, checksum_index)
        self.assertLess(checksum_index, release_index)


if __name__ == "__main__":
    unittest.main()
