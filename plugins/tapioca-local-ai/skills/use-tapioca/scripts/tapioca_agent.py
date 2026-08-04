#!/usr/bin/env python3
"""Structured, dependency-free helper for agents operating Tapioca."""

from __future__ import annotations

import argparse
import json
import os
from pathlib import Path
import shutil
import signal
import subprocess
import time
from typing import Any
from urllib import error, request


def emit(ok: bool, **values: Any) -> None:
    print(json.dumps({"ok": ok, **values}, ensure_ascii=False))


def tapioca_binary() -> str | None:
    override = os.environ.get("TAPIOCA_BIN")
    return override or shutil.which("tapioca")


def run_cli(arguments: list[str]) -> subprocess.CompletedProcess[str]:
    binary = tapioca_binary()
    if not binary:
        raise FileNotFoundError("tapioca is not installed or not on PATH")
    return subprocess.run(
        [binary, *arguments],
        check=False,
        capture_output=True,
        text=True,
    )


def http_json(url: str, payload: Any | None = None, timeout: float = 10) -> Any:
    body = None if payload is None else json.dumps(payload).encode()
    headers = {"Accept": "application/json"}
    if body is not None:
        headers["Content-Type"] = "application/json"
        headers["Authorization"] = "Bearer tapioca-local"
    req = request.Request(url, data=body, headers=headers)
    with request.urlopen(req, timeout=timeout) as response:
        raw = response.read()
        return json.loads(raw) if raw else {"status": response.status}


def state_dir() -> Path:
    override = os.environ.get("TAPIOCA_AGENT_STATE")
    path = Path(override) if override else Path.home() / ".tapioca" / "agent-helper"
    path.mkdir(parents=True, exist_ok=True)
    return path


def state_file() -> Path:
    return state_dir() / "server.json"


def command_detect(_: argparse.Namespace) -> int:
    binary = tapioca_binary()
    if not binary:
        emit(False, error="tapioca is not installed or not on PATH")
        return 1
    result = run_cli(["version"])
    emit(
        result.returncode == 0,
        binary=binary,
        version=result.stdout.strip(),
        error=result.stderr.strip() or None,
    )
    return 0 if result.returncode == 0 else result.returncode


def command_cli(args: argparse.Namespace) -> int:
    result = run_cli([args.operation, *args.arguments])
    emit(
        result.returncode == 0,
        operation=args.operation,
        output=result.stdout,
        error=result.stderr or None,
        exit_code=result.returncode,
    )
    return result.returncode


def command_health(args: argparse.Namespace) -> int:
    url = f"http://{args.host}:{args.port}/health"
    try:
        payload = http_json(url, timeout=args.timeout)
        emit(True, url=url, response=payload)
        return 0
    except (error.URLError, TimeoutError, json.JSONDecodeError) as exc:
        emit(False, url=url, error=str(exc))
        return 1


def command_request(args: argparse.Namespace) -> int:
    endpoint = {
        "chat": "/v1/chat/completions",
        "responses": "/v1/responses",
        "messages": "/v1/messages",
    }[args.endpoint]
    payload = json.loads(Path(args.file).read_text(encoding="utf-8"))
    url = f"http://{args.host}:{args.port}{endpoint}"
    try:
        response = http_json(url, payload, timeout=args.timeout)
        emit(True, url=url, response=response)
        return 0
    except (error.URLError, TimeoutError, json.JSONDecodeError) as exc:
        emit(False, url=url, error=str(exc))
        return 1


def command_start(args: argparse.Namespace) -> int:
    try:
        http_json(f"http://{args.host}:{args.port}/health", timeout=1)
        emit(True, already_running=True, host=args.host, port=args.port)
        return 0
    except Exception:
        pass

    binary = tapioca_binary()
    if not binary:
        emit(False, error="tapioca is not installed or not on PATH")
        return 1

    log_path = state_dir() / "server.log"
    log = log_path.open("ab")
    kwargs: dict[str, Any] = {
        "stdin": subprocess.DEVNULL,
        "stdout": log,
        "stderr": subprocess.STDOUT,
    }
    if os.name == "nt":
        kwargs["creationflags"] = subprocess.CREATE_NEW_PROCESS_GROUP
    else:
        kwargs["start_new_session"] = True

    process = subprocess.Popen(
        [
            binary,
            "serve",
            args.model,
            "--host",
            args.host,
            "--port",
            str(args.port),
            "--context",
            str(args.context),
        ],
        **kwargs,
    )
    deadline = time.monotonic() + args.timeout
    while time.monotonic() < deadline:
        if process.poll() is not None:
            log.close()
            tail = log_path.read_text(encoding="utf-8", errors="replace")[-4000:]
            emit(False, error="tapioca serve exited before it became healthy", log=tail)
            return process.returncode or 1
        try:
            http_json(f"http://{args.host}:{args.port}/health", timeout=2)
            state_file().write_text(
                json.dumps(
                    {
                        "pid": process.pid,
                        "model": args.model,
                        "host": args.host,
                        "port": args.port,
                    }
                ),
                encoding="utf-8",
            )
            log.close()
            emit(
                True,
                pid=process.pid,
                model=args.model,
                host=args.host,
                port=args.port,
                log=str(log_path),
            )
            return 0
        except Exception:
            time.sleep(0.5)

    process.terminate()
    log.close()
    emit(False, error="timed out waiting for Tapioca health", log=str(log_path))
    return 1


def command_stop(_: argparse.Namespace) -> int:
    path = state_file()
    if not path.exists():
        emit(True, stopped=False, message="no managed Tapioca server is recorded")
        return 0
    state = json.loads(path.read_text(encoding="utf-8"))
    pid = int(state["pid"])
    expected = f"tapioca serve {state['model']}"
    if not managed_process_matches(pid, expected):
        emit(
            False,
            stopped=False,
            pid=pid,
            error="recorded PID no longer belongs to the managed Tapioca server",
        )
        return 1
    try:
        if os.name == "nt":
            os.kill(pid, signal.SIGTERM)
        else:
            os.killpg(pid, signal.SIGTERM)
    except ProcessLookupError:
        pass
    path.unlink(missing_ok=True)
    emit(True, stopped=True, pid=pid)
    return 0


def managed_process_matches(pid: int, expected: str) -> bool:
    """Avoid terminating an unrelated process if an old PID has been reused."""
    try:
        if os.name == "nt":
            result = subprocess.run(
                [
                    "powershell",
                    "-NoProfile",
                    "-Command",
                    (
                        f"(Get-CimInstance Win32_Process -Filter "
                        f"'ProcessId = {pid}').CommandLine"
                    ),
                ],
                check=False,
                capture_output=True,
                text=True,
            )
        else:
            result = subprocess.run(
                ["ps", "-p", str(pid), "-o", "command="],
                check=False,
                capture_output=True,
                text=True,
            )
    except OSError:
        return False
    command = " ".join(result.stdout.split())
    return result.returncode == 0 and expected in command


def parser() -> argparse.ArgumentParser:
    root = argparse.ArgumentParser(description=__doc__)
    sub = root.add_subparsers(dest="command", required=True)

    detect = sub.add_parser("detect")
    detect.set_defaults(func=command_detect)

    for operation in ("catalog", "list", "pull"):
        cli = sub.add_parser(operation)
        cli.add_argument("arguments", nargs="*")
        cli.set_defaults(func=command_cli, operation=operation)

    health = sub.add_parser("health")
    health.add_argument("--host", default="127.0.0.1")
    health.add_argument("--port", type=int, default=11435)
    health.add_argument("--timeout", type=float, default=10)
    health.set_defaults(func=command_health)

    start = sub.add_parser("start")
    start.add_argument("model")
    start.add_argument("--host", default="127.0.0.1")
    start.add_argument("--port", type=int, default=11435)
    start.add_argument("--context", type=int, default=65536)
    start.add_argument("--timeout", type=float, default=180)
    start.set_defaults(func=command_start)

    stop = sub.add_parser("stop")
    stop.set_defaults(func=command_stop)

    send = sub.add_parser("request")
    send.add_argument(
        "--endpoint", choices=("chat", "responses", "messages"), required=True
    )
    send.add_argument("--file", required=True)
    send.add_argument("--host", default="127.0.0.1")
    send.add_argument("--port", type=int, default=11435)
    send.add_argument("--timeout", type=float, default=300)
    send.set_defaults(func=command_request)
    return root


def main() -> int:
    args = parser().parse_args()
    try:
        return int(args.func(args))
    except (FileNotFoundError, json.JSONDecodeError, OSError) as exc:
        emit(False, error=str(exc))
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
