const repo = "https://github.com/kyndlo/tapioca";

const commands = [
  ["pull", "Download a model once", "tapioca pull qwen3:8b-q4_k_m"],
  ["run", "Chat in your terminal", "tapioca run qwen3:8b-q4_k_m"],
  ["serve", "Open an API for your apps", "tapioca serve qwen3:8b-q4_k_m"],
  ["image", "Create images locally", 'tapioca image sd-turbo --prompt "A pearl astronaut"'],
  ["video", "Turn prompts into motion", 'tapioca video ltx-video:2b-fp16 --prompt "Clouds rolling in"'],
  ["launch", "Power your coding agent", "tapioca launch opencode qwen3-coder:30b-mlx"],
];

const platforms = [
  {
    icon: "⌘",
    title: "Apple Silicon",
    copy: "Metal, MLX, MFLUX, and native llama.cpp. A particularly cozy home for local models.",
    tags: ["Text", "Image", "Video", "Agents"],
  },
  {
    icon: "▦",
    title: "Windows x64",
    copy: "Vulkan text models, NVIDIA CUDA, plus DirectML image generation for AMD and Intel.",
    tags: ["CUDA", "DirectML", "Vulkan"],
  },
  {
    icon: "◈",
    title: "Windows ARM64",
    copy: "Native ARM64 Tapioca, CPU llama.cpp, and ONNX image generation without x64 emulation.",
    tags: ["Native ARM64", "ONNX", "CPU"],
  },
];

const expertPoints = [
  ["OpenAI-compatible API", "Use /v1/chat/completions with tools, streaming, and familiar clients."],
  ["Backend-aware catalog", "A model variant declares its runtime, memory guidance, GPU needs, and platform."],
  ["Composable models", "Save reusable recipes that combine a base model, LoRAs, adapters, and presets."],
  ["Local by design", "Models and generated media stay on your machine under ~/.tapioca."],
];

export default function Home() {
  return (
    <main>
      <nav className="nav">
        <a className="brand" href="#top" aria-label="Tapioca home">
          <img src="/tapioca.png" alt="" />
          <span>tapioca</span>
        </a>
        <div className="navlinks">
          <a href="#start">Get started</a>
          <a href="#commands">Commands</a>
          <a href="#experts">For experts</a>
          <a className="button small ghost" href={repo}>GitHub ↗</a>
        </div>
      </nav>

      <section className="hero" id="top">
        <div className="heroCopy">
          <p className="eyebrow"><span /> Local AI, minus the drama</p>
          <h1>Your models.<br /><em>Ready to roll.</em></h1>
          <p className="lede">
            Run language, image, video, and coding-agent models on your own Mac or
            Windows PC—with one small, friendly command-line tool.
          </p>
          <div className="actions">
            <a className="button primary" href="#start">Make your first pearl →</a>
            <a className="button ghost" href={`${repo}/releases/latest`}>Download Tapioca</a>
          </div>
          <div className="trust">
            <span>✓ Open source</span><span>✓ Local-first</span><span>✓ No account required</span>
          </div>
        </div>
        <div className="heroArt" aria-label="Tapioca terminal example">
          <div className="orbit one">GLM</div>
          <div className="orbit two">MLX</div>
          <div className="orbit three">GGUF</div>
          <img src="/tapioca.png" alt="Tapioca pearl mascot" />
          <div className="terminal">
            <div className="terminalTop"><i /><i /><i /><span>tapioca</span></div>
            <code><b>›</b> tapioca run qwen3:8b-q4_k_m</code>
            <code className="muted">model ready · 6.2 GB · metal</code>
            <code><b>you ›</b> Build me something delightful.</code>
            <code><b>tapioca ›</b> Let&apos;s cook. <span className="cursor">▋</span></code>
          </div>
        </div>
      </section>

      <section className="ticker" aria-label="Supported tools">
        <span>CODEX</span><i>●</i><span>CLAUDE CODE</span><i>●</i><span>OPENCODE</span>
        <i>●</i><span>HERMES</span><i>●</i><span>OPENCLAW</span>
      </section>

      <section className="section start" id="start">
        <div className="sectionHead">
          <p className="kicker">01 · Beginner lane</p>
          <h2>From zero to chatting<br />in three tiny steps.</h2>
          <p>No model archaeology degree required. Tapioca downloads what you need and remembers where it lives.</p>
        </div>
        <div className="steps">
          <article>
            <span className="stepNo">1</span>
            <p className="label">Install</p>
            <h3>Get the app</h3>
            <pre><code>curl -fsSL https://raw.githubusercontent.com/kyndlo/tapioca/main/scripts/install.sh | sh</code></pre>
            <p>macOS Apple Silicon. Windows users can grab the x64 or ARM64 bundle from Releases.</p>
          </article>
          <article>
            <span className="stepNo">2</span>
            <p className="label">Choose</p>
            <h3>Pick a model</h3>
            <pre><code>tapioca catalog</code></pre>
            <p>See download size, memory guidance, GPU needs, and the right variant for your computer.</p>
          </article>
          <article>
            <span className="stepNo">3</span>
            <p className="label">Run</p>
            <h3>Say hello</h3>
            <pre><code>tapioca run qwen3:8b-q4_k_m</code></pre>
            <p>If the model is missing, Tapioca pulls it automatically. Type <code>/bye</code> when you&apos;re done.</p>
          </article>
        </div>
        <p className="helper">Not sure which model? <a href={`${repo}/blob/main/docs/guides/choosing-models.md`}>Use the friendly model chooser →</a></p>
      </section>

      <section className="section platformSection">
        <div className="sectionHead compact">
          <p className="kicker">02 · Bring your computer</p>
          <h2>One CLI. Three hardware worlds.</h2>
        </div>
        <div className="platforms">
          {platforms.map((item) => (
            <article key={item.title}>
              <span className="platformIcon">{item.icon}</span>
              <h3>{item.title}</h3>
              <p>{item.copy}</p>
              <div className="tags">{item.tags.map((tag) => <span key={tag}>{tag}</span>)}</div>
            </article>
          ))}
        </div>
      </section>

      <section className="section commandSection" id="commands">
        <div className="sectionHead compact">
          <p className="kicker">03 · Six verbs, lots of power</p>
          <h2>A pleasantly small command surface.</h2>
          <p>Learn the shape once, then move from a chat to an API, an image, or a full coding agent.</p>
        </div>
        <div className="commandGrid">
          {commands.map(([name, description, command]) => (
            <article key={name}>
              <div><span className="prompt">›</span><h3>{name}</h3></div>
              <p>{description}</p>
              <code>{command}</code>
            </article>
          ))}
        </div>
      </section>

      <section className="section studio">
        <div>
          <p className="kicker light">04 · The local studio</p>
          <h2>Words in.<br />Pixels and motion out.</h2>
          <p>
            Tapioca understands platform-native diffusion: MLX and MFLUX on Mac,
            CUDA and DirectML on Windows, and native ONNX on Windows ARM64.
          </p>
          <div className="actions">
            <a className="button cream" href={`${repo}/blob/main/docs/guides/image-generation.md`}>Image guide</a>
            <a className="button darkGhost" href={`${repo}/blob/main/docs/guides/video-generation.md`}>Video guide</a>
          </div>
        </div>
        <div className="recipe">
          <p><span>IMAGE RECIPE</span><b>fox-studio</b></p>
          <code><em>base</em> sd-turbo:onnx-directml</code>
          <code><em>prompt</em> “a red fox in snow”</code>
          <code><em>size</em> 512 × 512</code>
          <code><em>steps</em> 4</code>
          <strong>tapioca image fox-studio</strong>
        </div>
      </section>

      <section className="section experts" id="experts">
        <div className="sectionHead">
          <p className="kicker">05 · Expert lane</p>
          <h2>Simple outside.<br />Serious underneath.</h2>
          <p>Use Tapioca as an orchestration layer, a local OpenAI-compatible endpoint, or a portable runtime for your team.</p>
        </div>
        <div className="expertLayout">
          <div className="architecture">
            <div className="archTop">Your tools <span>Codex · Claude Code · OpenCode</span></div>
            <div className="connector">OpenAI-compatible API</div>
            <div className="archCore"><img src="/tapioca.png" alt="" /><b>Tapioca</b><span>catalog · routing · lifecycle</span></div>
            <div className="branches">
              <span>llama.cpp<small>GGUF</small></span>
              <span>MLX / MFLUX<small>Apple Silicon</small></span>
              <span>Diffusers<small>CUDA</small></span>
              <span>ONNX<small>DirectML / ARM</small></span>
            </div>
          </div>
          <div className="expertList">
            {expertPoints.map(([title, copy], index) => (
              <article key={title}><span>0{index + 1}</span><div><h3>{title}</h3><p>{copy}</p></div></article>
            ))}
          </div>
        </div>
      </section>

      <section className="cta">
        <img src="/tapioca.png" alt="" />
        <p className="kicker light">Your machine has been waiting</p>
        <h2>Ready to roll?</h2>
        <p>Start small. Pull a model. Make something weird and wonderful.</p>
        <div className="actions">
          <a className="button cream" href={`${repo}/releases/latest`}>Download Tapioca</a>
          <a className="button darkGhost" href={`${repo}/tree/main/docs`}>Read all the docs</a>
        </div>
      </section>

      <footer>
        <a className="brand" href="#top"><img src="/tapioca.png" alt="" /><span>tapioca</span></a>
        <p>Local AI should feel like yours. Built in the open.</p>
        <div><a href={repo}>GitHub</a><a href={`${repo}/releases`}>Releases</a><a href={`${repo}/issues`}>Issues</a></div>
      </footer>
    </main>
  );
}
