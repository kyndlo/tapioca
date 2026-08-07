#!/usr/bin/env python3
"""Validate that agent docs, website routes, examples, and plugins agree."""

from __future__ import annotations

import json
from pathlib import Path
import re
import sys


ROOT = Path(__file__).resolve().parents[1]
EXPECTED_ENDPOINTS = (
    "/health",
    "/v1/models",
    "/v1/chat/completions",
    "/v1/responses",
    "/v1/messages",
)


def fail(message: str) -> None:
    raise AssertionError(message)


def read(path: str) -> str:
    return (ROOT / path).read_text(encoding="utf-8")


def main() -> int:
    server = read("internal/server/server.go")
    api_docs = read("docs/agents/api-reference.md")
    page_contract = read("website/app/llm/agent-content.ts")
    skill = read("plugins/tapioca-local-ai/skills/use-tapioca/SKILL.md")
    workflows = read("docs/agents/workflows.md")
    media_reference = read(
        "plugins/tapioca-local-ai/skills/use-tapioca/references/media-and-safety.md"
    )

    registered = set(re.findall(r'mux\.HandleFunc\("([^"]+)"', server))
    for endpoint in EXPECTED_ENDPOINTS:
        if endpoint not in registered:
            fail(f"{endpoint} is no longer registered by the Tapioca server")
        if endpoint not in api_docs:
            fail(f"{endpoint} is missing from docs/agents/api-reference.md")
        if endpoint not in page_contract:
            fail(f"{endpoint} is missing from the website agent contract")

    for path in sorted((ROOT / "docs/agents/examples").glob("*.json")):
        json.loads(path.read_text(encoding="utf-8"))

    for name, content in {
        "website machine contract": page_contract,
        "agent workflows": workflows,
        "agent skill media reference": media_reference,
    }.items():
        for marker in ("minimax-h3", "adapter inspect", "17n+5"):
            if marker not in content:
                fail(f"{name} is missing MiniMax-H3 marker {marker!r}")

    codex = json.loads(
        read("plugins/tapioca-local-ai/.codex-plugin/plugin.json")
    )
    claude = json.loads(
        read("plugins/tapioca-local-ai/.claude-plugin/plugin.json")
    )
    marketplace = json.loads(read(".agents/plugins/marketplace.json"))
    if codex["name"] != claude["name"]:
        fail("Codex and Claude plugin names differ")
    if "use-tapioca" not in skill:
        fail("shared skill is missing its canonical name")
    entries = {entry["name"]: entry for entry in marketplace["plugins"]}
    entry = entries.get(codex["name"])
    if not entry:
        fail("plugin is missing from the Codex marketplace")
    source = ROOT / entry["source"]["path"]
    if source.resolve() != (ROOT / "plugins/tapioca-local-ai").resolve():
        fail("Codex marketplace points at the wrong plugin directory")

    required = (
        "docs/agents/agent-guide.md",
        "docs/agents/api-reference.md",
        "docs/agents/workflows.md",
        "docs/agents/safety.md",
        "docs/agents/install-integrations.md",
        "website/app/llm/page.tsx",
        "website/app/llms.txt/route.ts",
        "website/app/llms-full.txt/route.ts",
    )
    missing = [path for path in required if not (ROOT / path).is_file()]
    if missing:
        fail(f"missing agent artifacts: {', '.join(missing)}")

    print("agent contract validation passed")
    return 0


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except (AssertionError, KeyError, json.JSONDecodeError) as exc:
        print(f"agent contract validation failed: {exc}", file=sys.stderr)
        raise SystemExit(1)
