import unittest
from types import SimpleNamespace
from cpu_speech import audio8


class Audio8InputTests(unittest.TestCase):
    def test_language_rejected_before_loading_runtime(self):
        with self.assertRaisesRegex(ValueError, "Unsupported Audio8 language"):
            audio8(SimpleNamespace(language="unknown"))

    def test_reference_requires_both_consent_and_transcript(self):
        for consent, transcript in ((False, "hello"), (True, " ")):
            with self.assertRaisesRegex(ValueError, "permission and the exact reference transcript"):
                audio8(SimpleNamespace(language="en", voice_sample="ref.wav", voice_consent=consent, transcript=transcript))
