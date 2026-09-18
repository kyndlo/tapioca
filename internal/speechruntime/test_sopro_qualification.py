import tempfile
import unittest
from pathlib import Path
from types import SimpleNamespace

from sopro_qualification import ARTIFACTS, validate_inputs, write_pcm_stream


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

    def test_stream_writes_complete_wav(self):
        with tempfile.TemporaryDirectory() as temp:
            output = Path(temp) / "speech.wav"
            result = write_pcm_stream([b"\x00\x00", b"\x01\x00"], output, 24000, 1.0,
                                      clock=lambda: 2.0)
            self.assertEqual(result["samples"], 2)
            self.assertEqual(output.read_bytes()[:4], b"RIFF")
            with self.assertRaisesRegex(ValueError, "incomplete PCM16"):
                write_pcm_stream([b"\x01"], output, 24000, 1.0, clock=lambda: 2.0)


if __name__ == "__main__":
    unittest.main()
