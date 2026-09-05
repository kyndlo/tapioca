#!/usr/bin/env python3
"""Build a review-only model freshness report from Hugging Face and Reddit."""

from __future__ import annotations

import argparse
import concurrent.futures
import datetime as dt
import json
import re
import time
import urllib.error
import urllib.parse
import urllib.request
import xml.etree.ElementTree as ET
from pathlib import Path


USER_AGENT = "tapioca-model-watch/1.0 (+https://github.com/kyndlo/tapioca)"
HUGGING_FACE_API = "https://huggingface.co/api/models"
PIPELINES = {
    "Language": ("text-generation",),
    "Image": ("text-to-image",),
    "Speech/audio": ("text-to-speech",),
    "Video": ("text-to-video", "image-to-video"),
}
SUBREDDITS = (
    "LocalLLaMA",
    "LocalLLM",
    "StableDiffusion",
    "comfyui",
    "AudioAI",
    "accelerate",
)
MODEL_POST = re.compile(
    r"\b(release(?:d)?|new model|open[- ]?(?:source|weights)|hugging ?face|"
    r"llm|tts|text.to.speech|image model|video model|flux|qwen|wan|ltx|"
    r"minimax|krea|audio8|indextts)\b",
    re.IGNORECASE,
)


def request_bytes(url: str, *, method: str = "GET", retries: int = 0) -> bytes:
    for attempt in range(retries + 1):
        request = urllib.request.Request(
            url,
            method=method,
            headers={
                "User-Agent": USER_AGENT,
                "Accept": "application/json, application/atom+xml",
            },
        )
        try:
            with urllib.request.urlopen(request, timeout=30) as response:
                return response.read()
        except urllib.error.HTTPError as error:
            if error.code != 429 or attempt == retries:
                raise
            retry_after = error.headers.get("Retry-After", "")
            delay = int(retry_after) if retry_after.isdigit() else 5 * (attempt + 1)
            time.sleep(min(delay, 30))
    raise RuntimeError("unreachable")


def request_status(url: str) -> int:
    request = urllib.request.Request(
        url, method="HEAD", headers={"User-Agent": USER_AGENT}
    )
    try:
        with urllib.request.urlopen(request, timeout=15):
            pass
        return 200
    except urllib.error.HTTPError as error:
        return error.code
    except (OSError, TimeoutError):
        return 0


def catalog_inventory(catalog: dict) -> tuple[set[str], list[tuple[str, str, str]]]:
    repositories: set[str] = set()
    artifacts: list[tuple[str, str, str]] = []
    for model in catalog.get("models", {}).values():
        base_repo = model.get("repo", "")
        variant_repos = model.get("repos", {})
        if base_repo:
            repositories.add(base_repo)
        repositories.update(repo for repo in variant_repos.values() if repo)
        for variant, filename in model.get("files", {}).items():
            repo = variant_repos.get(variant, base_repo)
            if repo and filename:
                revision = model.get("downloads", {}).get(variant, {}).get("revision") or "main"
                artifacts.append((repo, filename, revision))
        for bundle in model.get("artifacts", {}).values():
            for artifact in bundle:
                repo = artifact.get("repo", "")
                filename = artifact.get("filename", "")
                if repo:
                    repositories.add(repo)
                if repo and filename:
                    artifacts.append((repo, filename, artifact.get("revision") or "main"))
    return repositories, artifacts


def hugging_face_models(pipeline: str, limit: int = 20) -> list[dict]:
    query = urllib.parse.urlencode(
        {
            "pipeline_tag": pipeline,
            "sort": "trendingScore",
            "direction": "-1",
            "limit": str(limit),
            "full": "true",
        }
    )
    return json.loads(request_bytes(f"{HUGGING_FACE_API}?{query}"))


def reddit_posts(subreddits: tuple[str, ...] = SUBREDDITS) -> list[dict[str, str]]:
    joined = "+".join(subreddits)
    url = f"https://www.reddit.com/r/{joined}/new/.rss"
    root = ET.fromstring(request_bytes(url, retries=2))
    namespace = {"atom": "http://www.w3.org/2005/Atom"}
    posts: list[dict[str, str]] = []
    for entry in root.findall("atom:entry", namespace):
        title = entry.findtext("atom:title", default="", namespaces=namespace).strip()
        if not MODEL_POST.search(title):
            continue
        link = entry.find("atom:link", namespace)
        categories = [
            category.attrib.get("term", "")
            for category in entry.findall("atom:category", namespace)
        ]
        subreddit = next(
            (category for category in categories if category in subreddits), "multi"
        )
        posts.append(
            {
                "subreddit": subreddit,
                "title": title,
                "url": link.attrib.get("href", "") if link is not None else "",
                "updated": entry.findtext(
                    "atom:updated", default="", namespaces=namespace
                ),
            }
        )
    return posts


def catalog_health(repositories: set[str], artifacts: list[tuple[str, str, str]]) -> list[str]:
    checks: list[tuple[str, str]] = []
    for repo in sorted(repositories):
        encoded = urllib.parse.quote(repo, safe="/")
        checks.append(
            (f"repository `{repo}`", f"https://huggingface.co/api/models/{encoded}")
        )
    for repo, filename, revision in sorted(set(artifacts)):
        encoded_repo = urllib.parse.quote(repo, safe="/")
        encoded_file = urllib.parse.quote(filename, safe="/")
        encoded_revision = urllib.parse.quote(revision, safe="")
        checks.append(
            (
                f"artifact `{repo}/{filename}`",
                f"https://huggingface.co/{encoded_repo}/resolve/{encoded_revision}/{encoded_file}",
            )
        )

    def check(item: tuple[str, str]) -> str | None:
        label, url = item
        status = request_status(url)
        if status != 200:
            return f"{label} returned HTTP {status or 'network error'}"
        return None

    with concurrent.futures.ThreadPoolExecutor(max_workers=8) as executor:
        results = executor.map(check, checks)
    return [failure for failure in results if failure]


def model_line(model: dict) -> str:
    model_id = model.get("id", "unknown")
    updated = str(model.get("lastModified", ""))[:10] or "unknown"
    downloads = int(model.get("downloads") or 0)
    likes = int(model.get("likes") or 0)
    return (
        f"- [{model_id}](https://huggingface.co/{model_id}) — updated {updated}; "
        f"{downloads:,} downloads; {likes:,} likes"
    )


def build_report(catalog_path: Path) -> str:
    catalog = json.loads(catalog_path.read_text(encoding="utf-8"))
    repositories, artifacts = catalog_inventory(catalog)
    generated = dt.datetime.now(dt.timezone.utc).replace(microsecond=0).isoformat()
    warnings: list[str] = []
    sections: list[str] = []

    for label, pipelines in PIPELINES.items():
        candidates: dict[str, dict] = {}
        for pipeline in pipelines:
            try:
                for model in hugging_face_models(pipeline):
                    model_id = model.get("id", "")
                    if model_id and model_id not in repositories:
                        candidates.setdefault(model_id, model)
            except (OSError, TimeoutError, ValueError, urllib.error.HTTPError) as error:
                warnings.append(f"Hugging Face `{pipeline}` query failed: {error}")
        ranked = sorted(
            candidates.values(),
            key=lambda model: (
                int(model.get("trendingScore") or 0),
                int(model.get("likes") or 0),
            ),
            reverse=True,
        )
        lines = [model_line(model) for model in ranked[:8]]
        sections.append(f"## {label} candidates\n\n" + ("\n".join(lines) or "No candidates found."))

    try:
        posts = reddit_posts()
    except (OSError, TimeoutError, ValueError, ET.ParseError, urllib.error.HTTPError) as error:
        posts = []
        warnings.append(f"Reddit combined feed failed: {error}")
    posts.sort(key=lambda post: post["updated"], reverse=True)
    reddit_lines = [
        f"- r/{post['subreddit']}: [{post['title']}]({post['url']}) — {post['updated'][:10]}"
        for post in posts[:20]
    ]
    sections.append("## Reddit field signals\n\n" + ("\n".join(reddit_lines) or "No matching posts found."))

    try:
        health_failures = catalog_health(repositories, artifacts)
    except Exception as error:  # keep the report useful if a provider changes behavior
        health_failures = [f"catalog health check failed: {error}"]
    health = (
        "All catalog repositories and exact artifacts responded successfully."
        if not health_failures
        else "\n".join(f"- {failure}" for failure in health_failures)
    )
    sections.append(f"## Current catalog health\n\n{health}")

    if warnings:
        sections.append("## Source warnings\n\n" + "\n".join(f"- {warning}" for warning in warnings))

    return (
        "# Daily model freshness report\n\n"
        f"Generated: `{generated}`\n\n"
        "This is a discovery report, not an automatic compatibility decision. Before adding a "
        "model, verify its official model card, license, exact artifacts, pinned-runtime support, "
        "hardware requirements, and Tapioca tests as required by `AGENTS.md`.\n\n"
        + "\n\n".join(sections)
        + "\n"
    )


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--catalog", type=Path, default=Path("catalog/catalog.json"))
    parser.add_argument("--output", type=Path)
    args = parser.parse_args()
    report = build_report(args.catalog)
    if args.output:
        args.output.write_text(report, encoding="utf-8")
    else:
        print(report, end="")


if __name__ == "__main__":
    main()
