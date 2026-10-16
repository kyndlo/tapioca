#!/usr/bin/env python3
"""Exercise acceptance-run cleanup without downloading model weights."""
import importlib.util
import os
from pathlib import Path
import signal
import subprocess
import sys
import unittest

spec = importlib.util.spec_from_file_location('qualification', Path(__file__).with_name('qualify-minicpm-vision.py'))
qualification = importlib.util.module_from_spec(spec)
spec.loader.exec_module(qualification)


@unittest.skipIf(os.name == 'nt', 'Qualification and process groups are Unix-only')
class CleanupTests(unittest.TestCase):
    def test_normal_shutdown_reaps_launcher(self):
        proc = subprocess.Popen([sys.executable, '-c', 'import time; time.sleep(60)'], start_new_session=True)
        try:
            qualification.stop_process_tree(proc, timeout=2)
            self.assertIsNotNone(proc.returncode)
        finally:
            if proc.poll() is None:
                os.killpg(proc.pid, signal.SIGKILL)
                proc.wait()

    def test_timeout_kills_separately_grouped_backend(self):
        backend = "import signal,time; signal.signal(signal.SIGTERM,signal.SIG_IGN); print('ready',flush=True); time.sleep(60)"
        launcher = "import subprocess,sys,signal,time; signal.signal(signal.SIGTERM,signal.SIG_IGN); child=subprocess.Popen([sys.executable,'-c',sys.argv[1]],start_new_session=True,stdout=subprocess.PIPE,text=True); child.stdout.readline(); print(child.pid,flush=True); time.sleep(60)"
        proc = subprocess.Popen([sys.executable, '-c', launcher, backend], start_new_session=True, stdout=subprocess.PIPE, text=True)
        backend_pid = int(proc.stdout.readline())
        try:
            qualification.stop_process_tree(proc, timeout=.05)
            self.assertEqual(proc.returncode, -signal.SIGKILL)
            status = subprocess.run(['ps', '-o', 'stat=', '-p', str(backend_pid)], capture_output=True, text=True)
            # Some hosts briefly retain an orphaned zombie before reaping it.
            self.assertTrue(not status.stdout.strip() or status.stdout.strip().startswith('Z'), status.stdout)
        finally:
            for pid in [backend_pid, proc.pid]:
                try:
                    os.killpg(pid, signal.SIGKILL)
                except ProcessLookupError:
                    pass
            proc.wait()
            proc.stdout.close()


if __name__ == '__main__':
    unittest.main()
