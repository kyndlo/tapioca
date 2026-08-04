#!/usr/bin/env python3
"""Small black-box tests for the dependency-free agent helper."""

from __future__ import annotations

import json
import os
from pathlib import Path
import stat
import subprocess
import sys
import tempfile
import unittest


ROOT = Path(__file__).resolve().parents[1]
HELPER = (
    ROOT
    / "plugins/tapioca-local-ai/skills/use-tapioca/scripts/tapioca_agent.py"
)


class AgentHelperTest(unittest.TestCase):
    def run_helper(
        self, *arguments: str, env: dict[str, str] | None = None
    ) -> tuple[subprocess.CompletedProcess[str], dict[str, object]]:
        result = subprocess.run(
            [sys.executable, str(HELPER), *arguments],
            check=False,
            capture_output=True,
            text=True,
            env=env,
        )
        return result, json.loads(result.stdout)

    def test_detect_reports_missing_binary_as_json(self) -> None:
        env = os.environ.copy()
        env["PATH"] = ""
        env.pop("TAPIOCA_BIN", None)
        result, payload = self.run_helper("detect", env=env)
        self.assertEqual(result.returncode, 1)
        self.assertFalse(payload["ok"])

    def test_detect_and_catalog_wrap_cli_output(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            fake = Path(directory) / "tapioca"
            fake.write_text(
                "#!/bin/sh\n"
                'if [ "$1" = version ]; then echo "tapioca test"; exit 0; fi\n'
                'if [ "$1" = catalog ]; then echo "model-a"; exit 0; fi\n'
                "exit 2\n",
                encoding="utf-8",
            )
            fake.chmod(fake.stat().st_mode | stat.S_IXUSR)
            env = os.environ.copy()
            env["TAPIOCA_BIN"] = str(fake)

            detected, detection = self.run_helper("detect", env=env)
            self.assertEqual(detected.returncode, 0)
            self.assertEqual(detection["version"], "tapioca test")

            catalog, listing = self.run_helper("catalog", env=env)
            self.assertEqual(catalog.returncode, 0)
            self.assertEqual(listing["operation"], "catalog")
            self.assertIn("model-a", listing["output"])

    def test_stop_does_not_kill_a_reused_pid(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            state = Path(directory)
            (state / "server.json").write_text(
                json.dumps(
                    {
                        "pid": os.getpid(),
                        "model": "definitely-not-this-process",
                        "host": "127.0.0.1",
                        "port": 11435,
                    }
                ),
                encoding="utf-8",
            )
            env = os.environ.copy()
            env["TAPIOCA_AGENT_STATE"] = directory
            result, payload = self.run_helper("stop", env=env)
            self.assertEqual(result.returncode, 1)
            self.assertFalse(payload["stopped"])
            self.assertTrue((state / "server.json").exists())


if __name__ == "__main__":
    unittest.main()
