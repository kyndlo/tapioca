import tempfile
import unittest
import wave
from pathlib import Path
from unittest.mock import MagicMock, patch

from pocket_qualification import local_inputs, write_pcm_stream


class PocketQualificationTests(unittest.TestCase):
    def test_chunk_is_written_before_next_is_consumed(self):
        writer = MagicMock()
        writer.__enter__.return_value = writer
        def chunks():
            yield b"\x01\x00"
            writer.writeframes.assert_called_once_with(b"\x01\x00")
            yield b"\x02\x00"
        with patch("pocket_qualification.wave.open", return_value=writer):
            write_pcm_stream(chunks(), "unused.wav", 24000)
        self.assertEqual(writer.writeframes.call_count, 2)

    def test_stream_writes_each_chunk_before_requesting_next(self):
        with tempfile.TemporaryDirectory() as directory:
            output = Path(directory) / "out.wav"
            first = b"\x00\x00\x01\x00"
            def chunks():
                yield first
                self.assertTrue(output.exists())
                yield b"\x02\x00"
            result = write_pcm_stream(chunks(), output, 24000)
            with wave.open(str(output), "rb") as wav:
                self.assertEqual(wav.getnchannels(), 1)
                self.assertEqual(wav.getsampwidth(), 2)
                self.assertEqual(wav.getframerate(), 24000)
                self.assertEqual(wav.readframes(3), first + b"\x02\x00")
            self.assertEqual(result["samples"], 3)
            self.assertIsNotNone(result["first_audio_seconds"])

    def test_rejects_empty_or_malformed_pcm(self):
        with tempfile.TemporaryDirectory() as directory:
            for chunks in ([], [b"x"]):
                with self.assertRaises(ValueError):
                    write_pcm_stream(chunks, Path(directory) / "out.wav", 24000)

    def test_reference_requires_consent_before_runtime_import(self):
        with self.assertRaisesRegex(ValueError, "permission"):
            local_inputs("/missing", "/reference.wav")

    def test_missing_bundle_has_actionable_error(self):
        with tempfile.TemporaryDirectory() as directory:
            with self.assertRaisesRegex(ValueError, "Hugging Face terms"):
                local_inputs(directory)


if __name__ == "__main__":
    unittest.main()
