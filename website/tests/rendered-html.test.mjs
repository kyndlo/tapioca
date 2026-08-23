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
  assert.match(html, /Import LoRAs or transfer models/i);
  assert.match(html, /Stay current without starting over/i);
  assert.match(html, /tapioca catalog update/i);
  assert.match(html, /tapioca update --check/i);
  assert.match(html, /Settings → Software updates/i);
  assert.match(html, /--seconds/);
  assert.match(html, /--random-seed/);
  assert.match(html, /--seed/);
  assert.match(html, /same generation settings again/i);
  assert.match(html, /Tapioca Desktop 0\.11\.0/i);
  assert.match(html, /property="og:image"/);
  assert.doesNotMatch(html, /codex-preview|react-loading-skeleton|Your site is taking shape/i);
});

test("ships production brand assets and metadata", async () => {
  const [layout, page, packageJson, wrangler] = await Promise.all([
    readFile(new URL("../app/layout.tsx", import.meta.url), "utf8"),
    readFile(new URL("../app/page.tsx", import.meta.url), "utf8"),
    readFile(new URL("../package.json", import.meta.url), "utf8"),
    readFile(new URL("../dist/server/wrangler.json", import.meta.url), "utf8"),
    access(new URL("../public/tapioca.png", import.meta.url)),
    access(new URL("../public/favicon.png", import.meta.url)),
    access(new URL("../public/og.png", import.meta.url)),
    access(new URL("../public/tapioca-desktop-ui.png", import.meta.url)),
  ]);

  assert.match(layout, /summary_large_image/);
  assert.match(layout, /x-forwarded-host/);
  assert.match(page, /tapioca-local-ai|github\.com\/kyndlo\/tapioca/i);
  assert.doesNotMatch(packageJson, /react-loading-skeleton/);
  assert.equal(JSON.parse(wrangler).assets.run_worker_first, true);
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
  assert.match(html, /Bundle-aware media/i);
  assert.match(html, /tapioca video minimax-h3/);
  assert.match(html, /adapter inspect/);
  assert.match(html, /adapter list/);
  assert.match(html, /Civitai, ModelScope/i);
	assert.match(html, /gated model/i);
});

test("renders the complete beginner learning center", async () => {
  const response = await render("/learn");
  assert.equal(response.status, 200);
  assert.match(response.headers.get("content-type") ?? "", /^text\/html\b/i);

  const html = await response.text();
  assert.match(html, /Tapioca for complete beginners/i);
  assert.match(html, /Install on a Mac/);
  assert.match(html, /Install on Windows/);
  assert.match(html, /Run your first LLM/);
  assert.match(html, /Clone a voice responsibly/);
  assert.match(html, /Generate your first image/);
  assert.match(html, /Generate motion and video/);
  assert.match(html, /six checks to make before downloading/i);
  assert.match(html, /Reuse downloaded LoRAs/i);
  assert.match(html, /Keep Tapioca current without losing anything/i);
  assert.match(html, /Refresh catalog/i);
  assert.match(html, /Import from computer/i);
  assert.match(html, /civitai:\/\/MODEL_ID\/VERSION_ID/i);
  assert.match(html, /MODELSCOPE_API_TOKEN/);
  assert.match(html, /adapter inspect/);
  assert.match(html, /tapioca-desktop-macos-arm64\.dmg/);
  assert.match(html, /tapioca-desktop-windows-amd64\.exe/);
	assert.match(html, /krea-2-turbo/);
	assert.match(html, /--accept-license/);
	assert.match(html, /does not bypass Hugging Face access/i);
	assert.match(html, /Choose an approximate duration/i);
	assert.match(html, /Explore, then reproduce/i);
	assert.match(html, /final duration can differ slightly/i);
	assert.match(html, /repeat the same generation settings/i);
});

test("renders a prominent import and transfer guide", async () => {
  const response = await render("/import");
  assert.equal(response.status, 200);
  assert.match(response.headers.get("content-type") ?? "", /^text\/html\b/i);

  const html = await response.text();
  assert.match(html, /Import one LoRA file/i);
  assert.match(html, /Import from computer/i);
  assert.match(html, /tapioca adapter import/i);
  assert.match(html, /Move your complete LoRA library/i);
  assert.match(html, /Transfer an installed base model/i);
  assert.match(html, /tapioca pull minimax-h3/i);
  assert.match(html, /Do not copy registry\.json alone/i);
  assert.match(html, /TAPIOCA_HOME/i);
});

test("publishes short verified installer endpoints", async () => {
  const [shellInstaller, powershellInstaller] = await Promise.all([
    readFile(new URL("../public/install.sh", import.meta.url), "utf8"),
    readFile(new URL("../public/install.ps1", import.meta.url), "utf8"),
  ]);
  assert.match(shellInstaller, /checksum did not match/);
  assert.match(shellInstaller, /Added by the Tapioca installer/);
  assert.match(powershellInstaller, /Get-FileHash/);
  assert.match(powershellInstaller, /SetEnvironmentVariable\("Path"/);
});

test("serves concise and full machine-readable agent contracts", async () => {
  for (const path of ["/llms.txt", "/llms-full.txt"]) {
    const response = await render(path, "text/plain");
    assert.equal(response.status, 200);
    assert.match(response.headers.get("content-type") ?? "", /^text\/plain\b/i);
    const body = await response.text();
    assert.match(body, /^# Tapioca/m);
    assert.match(body, /tapioca catalog/);
    assert.match(body, /tapioca catalog update/);
    assert.match(body, /tapioca update --check/);
    assert.match(body, /127\.0\.0\.1/);
    assert.match(body, /Treat model tool calls as untrusted proposals/);
    assert.match(body, /minimax-h3/);
    assert.match(body, /LoRA/i);
    assert.match(body, /adapter list/);
    assert.match(body, /civitai:\/\/MODEL_ID\/VERSION_ID/);
    assert.match(body, /adapter import FILE --base MODEL/);
    assert.match(body, /copy.*snapshot\.json/is);
    assert.match(body, /--seconds/);
    assert.match(body, /--random-seed/);
    assert.match(body, /reuse it with `--seed NUMBER`/i);
    assert.match(body, /never combine it with `--frames`/i);
  }
});
