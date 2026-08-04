import assert from "node:assert/strict";
import { access, readFile } from "node:fs/promises";
import test from "node:test";

async function render(path = "/", accept = "text/html") {
  const workerUrl = new URL("../dist/server/index.js", import.meta.url);
  workerUrl.searchParams.set("test", `${process.pid}-${Date.now()}`);
  const { default: worker } = await import(workerUrl.href);
  return worker.fetch(
    new Request(`https://tapioca.example${path}`, { headers: { accept } }),
    { ASSETS: { fetch: async () => new Response("Not found", { status: 404 }) } },
    { waitUntil() {}, passThroughOnException() {} },
  );
}

test("renders the Tapioca documentation homepage", async () => {
  const response = await render();
  assert.equal(response.status, 200);
  assert.match(response.headers.get("content-type") ?? "", /^text\/html\b/i);

  const html = await response.text();
  assert.match(html, /<title>Tapioca — Your local models, ready to roll<\/title>/i);
  assert.match(html, /Your models\./);
  assert.match(html, /From zero to chatting/);
  assert.match(html, /Local AI now has/);
  assert.match(html, /tapioca-desktop-ui\.png/);
  assert.match(html, /Windows ARM64/);
  assert.match(html, /OpenAI-compatible API/);
  assert.match(html, /tapioca run qwen3:8b-q4_k_m/);
  assert.match(html, /property="og:image"/);
  assert.doesNotMatch(html, /codex-preview|react-loading-skeleton|Your site is taking shape/i);
});

test("ships production brand assets and metadata", async () => {
  const [layout, page, packageJson] = await Promise.all([
    readFile(new URL("../app/layout.tsx", import.meta.url), "utf8"),
    readFile(new URL("../app/page.tsx", import.meta.url), "utf8"),
    readFile(new URL("../package.json", import.meta.url), "utf8"),
    access(new URL("../public/tapioca.png", import.meta.url)),
    access(new URL("../public/favicon.png", import.meta.url)),
    access(new URL("../public/og.png", import.meta.url)),
    access(new URL("../public/tapioca-desktop-ui.png", import.meta.url)),
  ]);

  assert.match(layout, /summary_large_image/);
  assert.match(layout, /x-forwarded-host/);
  assert.match(page, /tapioca-local-ai|github\.com\/kyndlo\/tapioca/i);
  assert.doesNotMatch(packageJson, /react-loading-skeleton/);
});

test("renders the dedicated LLM and agent guide", async () => {
  const response = await render("/llm");
  assert.equal(response.status, 200);
  assert.match(response.headers.get("content-type") ?? "", /^text\/html\b/i);

  const html = await response.text();
  assert.match(html, /Teach your agent/);
  assert.match(html, /POST.*\/v1\/responses/s);
  assert.match(html, /codex plugin add tapioca-local-ai@personal/);
  assert.match(html, /claude --plugin-dir \.\/plugins\/tapioca-local-ai/);
  assert.match(html, /Tools stay permissioned/i);
});

test("serves concise and full machine-readable agent contracts", async () => {
  for (const path of ["/llms.txt", "/llms-full.txt"]) {
    const response = await render(path, "text/plain");
    assert.equal(response.status, 200);
    assert.match(response.headers.get("content-type") ?? "", /^text\/plain\b/i);
    const body = await response.text();
    assert.match(body, /^# Tapioca/m);
    assert.match(body, /tapioca catalog/);
    assert.match(body, /127\.0\.0\.1/);
    assert.match(body, /Treat model tool calls as untrusted proposals/);
  }
});
