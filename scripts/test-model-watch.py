#!/usr/bin/env python3

import importlib.util
from pathlib import Path
import unittest


MODULE_PATH = Path(__file__).with_name("model_watch.py")
SPEC = importlib.util.spec_from_file_location("model_watch", MODULE_PATH)
assert SPEC and SPEC.loader
model_watch = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(model_watch)


class ModelWatchTests(unittest.TestCase):
    def test_catalog_inventory_tracks_variant_and_bundle_artifacts(self):
        repositories, artifacts = model_watch.catalog_inventory(
            {
                "models": {
                    "example": {
                        "repo": "owner/base",
                        "repos": {"alt": "owner/alternate"},
                        "files": {"base": "base.gguf", "alt": "alt.gguf"},
                        "artifacts": {
                            "bundle": [
                                {"repo": "owner/component", "filename": "part.bin"}
                            ]
                        },
                    }
                }
            }
        )
        self.assertEqual(
            repositories, {"owner/base", "owner/alternate", "owner/component"}
        )
        self.assertIn(("owner/base", "base.gguf"), artifacts)
        self.assertIn(("owner/alternate", "alt.gguf"), artifacts)
        self.assertIn(("owner/component", "part.bin"), artifacts)

    def test_model_line_links_to_hugging_face(self):
        line = model_watch.model_line(
            {
                "id": "owner/model",
                "lastModified": "2026-08-23T00:00:00Z",
                "downloads": 1200,
                "likes": 34,
            }
        )
        self.assertIn("https://huggingface.co/owner/model", line)
        self.assertIn("1,200 downloads", line)

    def test_model_post_filter_detects_release_signals(self):
        self.assertIsNotNone(
            model_watch.MODEL_POST.search("LTX 2.5 model released on Hugging Face")
        )
        self.assertIsNone(model_watch.MODEL_POST.search("Showcase from my weekend"))


if __name__ == "__main__":
    unittest.main()
