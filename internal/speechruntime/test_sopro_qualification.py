import tempfile
import unittest
from pathlib import Path
from types import SimpleNamespace

from sopro_qualification import ARTIFACTS, validate_inputs


class SoproValidationTests(unittest.TestCase):
    def test_requires_consent_and_all_local_artifacts(self):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            sample = root / "reference.wav"
            sample.write_bytes(b"audio")
            args = SimpleNamespace(model=str(root), text="Hello", language="en",
                                   voice_sample=str(sample), voice_consent=False,
                                   output=str(root / "output.wav"))
            with self.assertRaisesRegex(ValueError, "permission"):
                validate_inputs(args)
            args.voice_consent = True
            with self.assertRaisesRegex(ValueError, "Missing Sopro artifact"):
                validate_inputs(args)
            for name in ARTIFACTS:
                (root / name).write_bytes(b"model")
            self.assertEqual(validate_inputs(args)[0], root.resolve())

    def test_rejects_unsupported_language(self):
        args = SimpleNamespace(text="Hello", language="es")
        with self.assertRaisesRegex(ValueError, "language"):
            validate_inputs(args)


if __name__ == "__main__":
    unittest.main()
