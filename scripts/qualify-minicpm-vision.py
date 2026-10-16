#!/usr/bin/env python3
"""Opt-in real-weight acceptance. macOS arm64 only; records evidence, never publishes.

Usage: python3 scripts/qualify-minicpm-vision.py --binary /path/to/tapioca
  --runtime /path/to/b11696/llama-server --work /path/to/persistent/work
Optional --reference /path/to/MiniCPM-V-4_7-1B-F16.gguf runs the same fixtures at F16.
The model pair is downloaded via Tapioca, then pulled again with networking disabled.
"""
import argparse
import base64
from datetime import datetime, timezone
import hashlib
import json
import os
from pathlib import Path
import platform
import signal
import socket
import subprocess
import threading
import time
import urllib.error
import urllib.request

ROOT = Path(__file__).resolve().parent.parent


def digest(path):
    with path.open('rb') as source:
        return hashlib.file_digest(source, 'sha256').hexdigest()


def port():
    with socket.socket() as s:
        s.bind(('127.0.0.1', 0))
        return s.getsockname()[1]


def verify_rejected_artifacts(binary, runtime, env, model):
    results = {}
    for label, file_key in [('language', 'path'), ('projector', 'projector_path')]:
        path = Path(model[file_key])
        with path.open('r+b') as target:
            original = target.read(1)
            target.seek(0)
            target.write(bytes([original[0] ^ 1]))
        try:
            launch = subprocess.run([binary, 'serve', model['name'], '--context', '4096', '--llama-server', runtime], env=env, capture_output=True, text=True, timeout=30)
            assert launch.returncode != 0 and 'SHA-256 mismatch' in launch.stderr, launch.stderr
            pull = subprocess.run([binary, 'pull', model['name']], env=env, capture_output=True, text=True, timeout=30)
            assert pull.returncode != 0 and 'SHA-256 mismatch' in pull.stderr, pull.stderr
            results[label + '_corruption_rejected_on_pull_and_launch'] = True
        finally:
            with path.open('r+b') as target:
                target.write(original)
    path = Path(model['projector_path'])
    missing = path.with_suffix('.missing')
    path.rename(missing)
    try:
        launch = subprocess.run([binary, 'serve', model['name'], '--context', '4096', '--llama-server', runtime], env=env, capture_output=True, text=True, timeout=30)
        assert launch.returncode != 0 and str(path) in launch.stderr, launch.stderr
        results['missing_projector_rejected'] = True
    finally:
        missing.rename(path)
    return results


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--binary', type=Path, required=True)
    parser.add_argument('--runtime', type=Path, required=True)
    parser.add_argument('--work', type=Path, required=True)
    parser.add_argument('--reference', type=Path)
    args = parser.parse_args()
    if (platform.system(), platform.machine()) != ('Darwin', 'arm64'):
        parser.error('Only macOS Apple Silicon is in this acceptance scope')
    work = args.work.resolve()
    work.mkdir(parents=True, exist_ok=True)
    home = work / 'home'
    home.mkdir(exist_ok=True)
    (home / 'catalog.json').write_bytes((ROOT / 'catalog/candidates/minicpm-v-4.7.json').read_bytes())
    env = {**os.environ, 'TAPIOCA_HOME': str(home)}
    binary, runtime = str(args.binary.resolve()), str(args.runtime.resolve())
    subprocess.run([binary, 'pull', 'minicpm-v-4.7'], env=env, check=True)
    # A validated pair must be reusable without contacting Hugging Face.
    subprocess.run([binary, 'pull', 'minicpm-v-4.7'], env={**env, 'HTTPS_PROXY': 'http://127.0.0.1:1'}, check=True)
    registry = json.loads((home / 'registry.json').read_text())
    model = registry['models']['minicpm-v-4.7:1b-q4-k-m']
    report = {'date': datetime.now(timezone.utc).date().isoformat(), 'hardware': subprocess.check_output(['sysctl', '-n', 'machdep.cpu.brand_string'], text=True).strip(),
              'system_memory_bytes': int(subprocess.check_output(['sysctl', '-n', 'hw.memsize'], text=True)),
              'runtime': subprocess.check_output([runtime, '--version'], stderr=subprocess.STDOUT, text=True).strip(),
              'hashes': {Path(model['path']).name: digest(Path(model['path'])), Path(model['projector_path']).name: digest(Path(model['projector_path']))},
              'results': [], 'limitations': ['Host RSS is sampled; GPU/unified-memory peak is not separately measured.', 'No 8/16 GiB hardware minimum qualification.', 'No Windows/Linux/Vulkan qualification.']}
    report['integrity_rejections'] = verify_rejected_artifacts(binary, runtime, env, model)
    cases = [('q4-cpu-4k', model['name'], 4096, True), ('q4-cpu-8k', model['name'], 8192, True), ('q4-metal-4k', model['name'], 4096, False), ('q4-metal-8k', model['name'], 8192, False)]
    if args.reference:
        reference = args.reference.resolve()
        expected = 'bc5a85f8973c3cbb29b3bd195cce5279361221a8628bd50a2387261289ea2fbc'
        assert reference.stat().st_size == 2024372320 and digest(reference) == expected, 'Invalid pinned F16 reference'
        ref = {**model, 'name': 'minicpm-reference:f16', 'path': str(reference), 'filename': reference.name,
               'integrity': {**model['integrity'], 'sha256': expected, 'size_bytes': 2024372320}}
        registry['models'][ref['name']] = ref
        (home / 'registry.json').write_text(json.dumps(registry))
        report['hashes'][reference.name] = expected
        cases.append(('f16-metal-4k', ref['name'], 4096, False))
    def image(name):
        data = (ROOT / 'internal/server/testdata/vision' / name).read_bytes()
        return {'type': 'image_url', 'image_url': {'url': 'data:image/png;base64,' + base64.b64encode(data).decode()}}
    for label, model_name, context, cpu in cases:
        api_port, upstream_port = port(), port()
        log = (work / (label + '.log')).open('w')
        command = [binary, 'serve', model_name, '--port', str(api_port), '--upstream-port', str(upstream_port), '--context', str(context), '--llama-server', runtime, '--verbose']
        if cpu:
            command.append('--cpu')
        proc = subprocess.Popen(command, env=env, stdout=log, stderr=log, start_new_session=True)
        stop = threading.Event()
        peak = [0]
        def sample():
            while not stop.wait(.1):
                try:
                    children = subprocess.check_output(['pgrep', '-P', str(proc.pid)], text=True).split()
                    if children:
                        rss = subprocess.check_output(['ps', '-o', 'rss=', '-p', ','.join(children)], text=True)
                        peak[0] = max(peak[0], sum(int(x) for x in rss.split()) * 1024)
                except subprocess.CalledProcessError:
                    pass
        sampler = threading.Thread(target=sample, daemon=True)
        sampler.start()
        base = f'http://127.0.0.1:{api_port}'
        row = {'case': label, 'context': context, 'requests': []}
        report['results'].append(row)
        try:
            deadline = time.monotonic() + 180
            while True:
                if proc.poll() is not None:
                    raise RuntimeError(f'{label} exited; see {log.name}')
                try:
                    with urllib.request.urlopen(base + '/health', timeout=1):
                        break
                except (OSError, urllib.error.URLError):
                    if time.monotonic() > deadline:
                        raise RuntimeError('Startup timeout')
                    time.sleep(.25)
            def chat(name, messages, expected, **options):
                payload = {'model': model_name, 'messages': messages, 'temperature': 0, 'max_tokens': 256, **options}
                request = urllib.request.Request(base + '/v1/chat/completions', data=json.dumps(payload).encode(), headers={'Content-Type': 'application/json'})
                start = time.monotonic()
                with urllib.request.urlopen(request, timeout=180) as response:
                    result = json.load(response)
                choice = result['choices'][0]
                answer = choice['message'].get('content') or ''
                reasoning_present = bool(choice['message'].get('reasoning_content'))
                expected_reasoning = options.get('chat_template_kwargs', {}).get('enable_thinking', False)
                passed = (bool(answer.strip())
                          and all(word.lower() in answer.lower() for word in expected)
                          and choice['finish_reason'] == 'stop'
                          and reasoning_present == expected_reasoning)
                row['requests'].append({'name': name, 'elapsed_seconds': round(time.monotonic()-start, 3), 'answer': answer, 'finish_reason': choice['finish_reason'], 'passed': passed,
                                        'reasoning_present': reasoning_present, 'usage': result.get('usage')})
                print(label, name, repr(answer), choice['finish_reason'], 'PASS' if passed else 'FAIL', flush=True)
                return answer
            scene = [{'role': 'user', 'content': [{'type': 'text', 'text': 'Describe the two shapes and their colors.'}, image('shapes.png')]}]
            answer = chat('description', scene, ['red', 'blue', 'circle', 'square'])
            chat('ocr', [{'role': 'user', 'content': [{'type': 'text', 'text': 'Read the text in this image exactly.'}, image('ocr.png')]}], ['TAPIOCA', '57', '12.50'])
            chat('multi-turn', [*scene, {'role': 'assistant', 'content': answer}, {'role': 'user', 'content': 'What color was the circle? Answer briefly.'}], ['red'])
            chat('multiple-images', [{'role': 'user', 'content': [{'type': 'text', 'text': 'What is in the first image? Read the text in the second image.'}, image('shapes.png'), image('ocr.png')]}], ['red', 'blue', 'TAPIOCA', '12.50'])
            chat('reasoning-on', [{'role': 'user', 'content': [{'type': 'text', 'text': 'What color is the circle? Answer briefly.'}, image('shapes.png')]}], ['red'], max_tokens=1024, chat_template_kwargs={'enable_thinking': True})
            payload = {'model': model_name, 'messages': scene, 'stream': True, 'max_tokens': 128, 'temperature': 0}
            request = urllib.request.Request(base + '/v1/chat/completions', data=json.dumps(payload).encode(), headers={'Content-Type': 'application/json'})
            with urllib.request.urlopen(request, timeout=180) as response:
                stream = response.read().decode()
            assert 'data: [DONE]' in stream and '"finish_reason":"stop"' in stream.replace(' ', ''), 'Stream did not end with EOS'
            row['streaming_eos'] = True
            payload['max_tokens'] = 1024
            request.data = json.dumps(payload).encode()
            with urllib.request.urlopen(request, timeout=180) as response:
                assert response.readline().startswith(b'data:')
                # Closing the stream must cancel the upstream request.
            row['cancel_then_reuse'] = bool(chat('after-cancellation', [{'role': 'user', 'content': 'Say OK.'}], []))
        finally:
            if proc.poll() is None:
                os.killpg(proc.pid, signal.SIGTERM)
            proc.wait(timeout=30)
            stop.set()
            sampler.join(timeout=3)
            row['sampled_peak_host_rss_bytes'] = peak[0]
            log.close()
            (work / 'acceptance.json').write_text(json.dumps(report, indent=2) + '\n')
    failures = [f"{r['case']}: {q['name']}" for r in report['results'] for q in r['requests'] if not q['passed']]
    if failures:
        raise SystemExit('Acceptance failures: ' + ', '.join(failures))
    print('Evidence:', work / 'acceptance.json')


if __name__ == '__main__':
    main()
