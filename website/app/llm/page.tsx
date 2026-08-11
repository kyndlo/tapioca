import type { Metadata } from "next";
import {
  agentClients,
  baseUrl,
  contractVersion,
  endpoints,
  operatingSteps,
  repo,
} from "./agent-content";

export const metadata: Metadata = {
  title: "Tapioca for LLMs and agents",
  description:
    "Machine-readable instructions, APIs, tool loops, and installable skills for agents using Tapioca.",
};

const chatExample = `curl ${baseUrl}/v1/chat/completions \\
  -H "Content-Type: application/json" \\
  -H "Authorization: Bearer tapioca-local" \\
  -d '{
    "model": "glm-4.7-flash:q8_0",
    "messages": [
      {"role": "user", "content": "Say hello in one sentence."}
    ],
    "stream": false
  }'`;

const toolExample = `{
  "type": "function",
  "function": {
    "name": "read_project_file",
    "description": "Read a UTF-8 file in the workspace",
    "parameters": {
      "type": "object",
      "properties": {"path": {"type": "string"}},
      "required": ["path"],
      "additionalProperties": false
    }
  }
}`;

const videoExample = `tapioca pull minimax-h3
tapioca adapter list
tapioca adapter inspect hf://OWNER/REPOSITORY
tapioca adapter inspect civitai://MODEL_ID/VERSION_ID
tapioca adapter import ./adapter.safetensors --base minimax-h3 --name local-adapter
tapioca video minimax-h3 \\
  --prompt "A cinematic tracking shot" \\
  --adapter 'local://local-adapter#adapter.safetensors@0.8' \\
  --preset low-memory \\
  --output adapted.mp4`;

export default function LlmGuide() {
  return (
    <main className="llmPage">
      <nav className="llmNav">
        {/* vinext currently mis-hydrates next/link in server-only pages. */}
        {/* eslint-disable-next-line @next/next/no-html-link-for-pages */}
        <a className="brand" href="/" aria-label="Tapioca home">
          <img src="/tapioca.png" alt="" />
          <span>tapioca</span>
        </a>
        <div>
          <a href="#quickstart">Quickstart</a>
          <a href="#api">API</a>
          <a href="#media">Media</a>
          <a href="#skills">Agent skills</a>
          <a href="/llms.txt">llms.txt</a>
          <a className="button small ghost" href={repo}>GitHub ↗</a>
        </div>
      </nav>

      <header className="llmHero">
        <div>
          <p className="kicker">Agent contract · {contractVersion}</p>
          <h1>Teach your agent<br /><em>to roll local.</em></h1>
          <p>
            A precise operating manual for LLMs, coding agents, and automation
            that need private models, tool calls, images, video, or speech
            through Tapioca.
          </p>
          <div className="actions">
            <a className="button primary" href="#quickstart">Read the contract →</a>
            <a className="button ghost" href="/llms-full.txt">Plain-text reference</a>
          </div>
        </div>
        <aside className="agentManifest">
          <div><span>agent.yml</span><b>● ready</b></div>
          <code><i>runtime</i> tapioca</code>
          <code><i>scope</i> localhost</code>
          <code><i>protocols</i> openai, anthropic</code>
          <code><i>tools</i> host-controlled</code>
          <code><i>privacy</i> local-first</code>
          <strong>GET {baseUrl}/health</strong>
        </aside>
      </header>

      <section className="agentStrip" aria-label="Agent guarantees">
        <span>DISCOVER, DON&apos;T GUESS</span><i>●</i>
        <span>LOOPBACK BY DEFAULT</span><i>●</i>
        <span>TOOLS STAY PERMISSIONED</span><i>●</i>
        <span>OUTPUTS STAY LOCAL</span>
      </section>

      <section className="llmSection llmQuickstart" id="quickstart">
        <div className="llmSectionHead">
          <p className="kicker">01 · Deterministic quickstart</p>
          <h2>Six moves from detection to cleanup.</h2>
          <p>
            The catalog—not memory—is the source of truth. Select the narrowest
            execution surface and leave the machine as you found it.
          </p>
        </div>
        <ol className="agentSteps">
          {operatingSteps.map(([title, command, copy], index) => (
            <li key={title}>
              <span>{String(index + 1).padStart(2, "0")}</span>
              <div><h3>{title}</h3><code>{command}</code><p>{copy}</p></div>
            </li>
          ))}
        </ol>
      </section>

      <section className="llmSection apiSection" id="api">
        <div className="llmSectionHead">
          <p className="kicker light">02 · Compatibility APIs</p>
          <h2>One local server.<br />Three familiar protocols.</h2>
          <p>
            Start on <code>{baseUrl}</code>, wait for health, then use the
            request shape your client already understands.
          </p>
        </div>
        <div className="apiGrid">
          <div className="endpointList">
            {endpoints.map(([method, path, copy]) => (
              <article key={path}>
                <span>{method}</span><code>{path}</code><p>{copy}</p>
              </article>
            ))}
          </div>
          <pre className="agentCode"><code>{chatExample}</code></pre>
        </div>
      </section>

      <section className="llmSection toolSection">
        <div className="toolCopy">
          <p className="kicker">03 · Tool calling</p>
          <h2>Tapioca transports.<br />Your agent decides.</h2>
          <p>
            Models propose tool calls; Tapioca never executes them. Validate
            function names and arguments, apply the host&apos;s approval policy,
            return a matching tool result, and cap the loop.
          </p>
          <ol>
            <li><span>1</span>Send JSON function definitions</li>
            <li><span>2</span>Validate the returned call</li>
            <li><span>3</span>Execute only an approved host tool</li>
            <li><span>4</span>Return the result with its call ID</li>
          </ol>
        </div>
        <pre className="toolCard"><code>{toolExample}</code></pre>
      </section>

      <section className="llmSection apiSection" id="media">
        <div className="llmSectionHead">
          <p className="kicker light">04 · Bundle-aware media</p>
          <h2>Ask for a model.<br />Not a pile of weights.</h2>
          <p>
            Agents treat catalog IDs as bundle contracts. MiniMax-H3 resolves
            the correct MPS or CUDA bundle, while Tapioca privately owns the
            engine graph, adapter ordering, and output path.
          </p>
          <p>
            LoRA automation uses the provider-neutral library: list installed
            references first, inspect or pull from Hugging Face, Civitai, or
            ModelScope, and import existing files into a managed local reference.
          </p>
        </div>
        <div className="apiGrid">
          <div className="endpointList">
            <article><span>1</span><code>tapioca catalog</code><p>Confirm platform, memory, and the exact model ID.</p></article>
            <article><span>2</span><code>adapter list</code><p>Reuse its canonical reference when the required LoRA is already installed.</p></article>
            <article><span>3</span><code>inspect / pull / import</code><p>Acquire from Hugging Face, Civitai, ModelScope, or a verified local file.</p></article>
            <article><span>4</span><code>ordered --adapter</code><p>Apply transformer LoRAs in command order; never infer compatibility by extension.</p></article>
            <article><span>5</span><code>verify output</code><p>Wait for exit, preserve the returned path, and inspect video plus audio streams.</p></article>
          </div>
          <pre className="agentCode"><code>{videoExample}</code></pre>
        </div>
      </section>

      <section className="llmSection clientSection">
        <div className="llmSectionHead">
          <p className="kicker">05 · Coding clients</p>
          <h2>Launch the agent.<br />Keep its real profile clean.</h2>
          <p>
            Tapioca creates isolated configuration under
            <code> TAPIOCA_HOME/launch</code>. The client must already be
            installed on the machine.
          </p>
        </div>
        <div className="clientGrid">
          {agentClients.map(([name, command]) => (
            <article key={name}><h3>{name}</h3><code>{command}</code></article>
          ))}
        </div>
      </section>

      <section className="llmSection skillsSection" id="skills">
        <div>
          <p className="kicker light">06 · Installable knowledge</p>
          <h2>Give Codex and Claude the playbook.</h2>
          <p>
            The repository ships one Agent Skills-compatible package with both
            Codex and Claude Code manifests. Its <b>use-tapioca</b> skill
            includes model selection, APIs, media, safety rules, and a
            structured cross-platform helper.
          </p>
          <a className="button cream" href={`${repo}/tree/main/plugins/tapioca-local-ai`}>
            Browse the plugin →
          </a>
        </div>
        <div className="installSkills">
          <article>
            <span>CODEX</span>
            <h3>Tapioca Local AI plugin</h3>
            <p>Add the repository marketplace, then install the local-AI plugin.</p>
            <code>codex plugin add tapioca-local-ai@personal</code>
          </article>
          <article>
            <span>CLAUDE CODE</span>
            <h3>/tapioca-local-ai:use-tapioca</h3>
            <p>Test directly from a checkout, then distribute through a Claude marketplace.</p>
            <code>claude --plugin-dir ./plugins/tapioca-local-ai</code>
          </article>
        </div>
      </section>

      <section className="llmSection safetySection">
        <div className="llmSectionHead">
          <p className="kicker">07 · Non-negotiables</p>
          <h2>Local power needs clear boundaries.</h2>
        </div>
        <div className="safetyGrid">
          <article><span>⌁</span><h3>Loopback first</h3><p>Never expose an unauthenticated model server by accident.</p></article>
          <article><span>↓</span><h3>Disclose downloads</h3><p>Report size and memory before pulling a very large model.</p></article>
          <article><span>✓</span><h3>Permission tools</h3><p>Treat every model-produced call as an untrusted proposal.</p></article>
          <article><span>≈</span><h3>Consent for voices</h3><p>Possessing an audio file does not imply permission to clone it.</p></article>
        </div>
      </section>

      <footer className="llmFooter">
        {/* eslint-disable-next-line @next/next/no-html-link-for-pages */}
        <a className="brand" href="/"><img src="/tapioca.png" alt="" /><span>tapioca</span></a>
        <p>Built for humans. Documented for agents.</p>
        <div><a href="/llms.txt">llms.txt</a><a href="/llms-full.txt">Full text</a><a href={repo}>GitHub</a></div>
      </footer>
    </main>
  );
}
