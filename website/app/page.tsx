const repo = "https://github.com/kyndlo/tapioca";
const discord = "https://discord.gg/vkFpNvY2ZY";

const commands = [
  ["catalog update", "Refresh verified model recipes", "tapioca catalog update"],
  ["update", "Install the latest verified release", "tapioca update --check && tapioca update"],
  ["pull", "Download a model once", "tapioca pull qwen3:8b-q4_k_m"],
  ["run", "Chat in your terminal", "tapioca run qwen3:8b-q4_k_m"],
  ["serve", "Open an API for your apps", "tapioca serve qwen3:8b-q4_k_m"],
  ["image", "Create images locally", 'tapioca image sd-turbo --prompt "A pearl astronaut"'],
  ["video", "Turn prompts into motion", 'tapioca video ltx-video:2b-fp16 --prompt "Clouds rolling in"'],
  ["tts", "Generate or clone a voice", 'tapioca tts chatterbox:multilingual --text "Hello"'],
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
  {
    icon: "🐧",
    title: "Linux",
    copy: "Vulkan text models, NVIDIA CUDA media generation, and local speech for GPU workstations and cloud servers.",
    tags: ["x64", "ARM64", "CUDA", "Vulkan"],
  },
];

const expertPoints = [
  ["OpenAI-compatible API", "Use /v1/chat/completions with tools, streaming, and familiar clients."],
  ["Backend-aware catalog", "A model variant declares its runtime, memory guidance, GPU needs, and platform."],
  ["Reusable LoRA library", "Pull from Hugging Face, Civitai, or ModelScope—or import local safetensors once and reuse them everywhere."],
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
          <a href="#desktop">Desktop app</a>
          <a href="/learn">Beginner guide</a>
          <a href="#updates">Updates</a>
          <a href="/import">Import & transfer</a>
          <a href="#voices">Voices</a>
          <a href="#commands">Commands</a>
          <a href="#experts">For experts</a>
          <a href="/llm">For agents</a>
          <a href={discord}>Discord ↗</a>
          <a className="button small ghost" href={repo}>GitHub ↗</a>
        </div>
      </nav>

      <section className="hero" id="top">
        <div className="heroCopy">
          <p className="eyebrow"><span /> Local AI, minus the drama</p>
          <h1>Your models.<br /><em>Ready to roll.</em></h1>
          <p className="lede">
            Run language, speech, image, video, and coding-agent models on your own
            Mac, Windows PC, or Linux server—with a friendly desktop studio and
            one compact command-line tool.
          </p>
          <div className="actions">
            <a className="button primary" href="/learn">Start the beginner guide →</a>
            <a className="button ghost" href={`${repo}/releases/latest`}>Download Tapioca</a>
          </div>
          <p className="heroHelper">Already have large weights? <a href="/import">Import LoRAs or transfer models without starting over →</a></p>
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

      <section className="section desktopShowcase" id="desktop">
        <div className="desktopIntro">
          <p className="kicker light">New · Tapioca Desktop 0.10.0</p>
          <h2>Local AI now has<br />a friendly face.</h2>
          <p>
            Chat, create images and videos, clone voices, manage models, and
            launch coding agents without living in a terminal. The same local
            Tapioca runtime powers everything underneath.
          </p>
          <div className="desktopFeatures">
            <span><b>Private chat</b>Run GGUF and MLX models locally</span>
            <span><b>Creative studio</b>Images, video, speech, and voice cloning</span>
            <span><b>Agent cockpit</b>Codex, Claude Code, OpenCode, OpenClaw, and Hermes</span>
            <span><b>Model manager</b>Discover, pull, remove, and inspect models</span>
            <span><b>Stay current</b>Automatic release checks and refreshable model recipes</span>
          </div>
          <div className="actions">
            <a className="button cream" href={`${repo}/releases/latest`}>Download the desktop app →</a>
            <a className="button darkGhost" href={`${repo}/tree/main/desktop`}>Explore the source</a>
          </div>
          <small>Checks GitHub Releases at startup, verifies downloads, and preserves your models and outputs while updating.</small>
        </div>
        <figure className="desktopFrame">
          <div className="desktopFrameTop">
            <i /><i /><i /><span>Tapioca · Images</span>
          </div>
          <img
            src="/tapioca-desktop-ui.png"
            alt="Tapioca Desktop image studio running Qwen Image Flash locally"
          />
          <figcaption>
            One workspace for chat, media generation, voices, agents, models, and APIs.
          </figcaption>
        </figure>
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
            <h3>Get Tapioca</h3>
            <pre><code>curl -fsSL https://tapioca.rootfruit.cc/install.sh | sh</code></pre>
            <p>Download the new desktop app, or use this command for the CLI on macOS and Linux. Windows gets an equally tiny PowerShell installer.</p>
          </article>
          <article>
            <span className="stepNo">2</span>
            <p className="label">Choose</p>
            <h3>Pick a model</h3>
            <pre><code>tapioca catalog update{`\n`}tapioca catalog</code></pre>
            <p>Refresh verified model recipes without reinstalling, then see download size, memory guidance, GPU needs, and the right variant for your computer.</p>
          </article>
          <article>
            <span className="stepNo">3</span>
            <p className="label">Run</p>
            <h3>Say hello</h3>
            <pre><code>tapioca run qwen3:8b-q4_k_m</code></pre>
            <p>If the model is missing, Tapioca pulls it automatically. Type <code>/bye</code> when you&apos;re done.</p>
          </article>
        </div>
        <p className="helper">Want every step explained? <a href="/learn">Open the complete beginner learning center →</a> Already downloaded weights? <a href="/import">Import and transfer them →</a></p>
      </section>

      <section className="section platformSection">
        <div className="sectionHead compact">
          <p className="kicker">02 · Bring your computer</p>
          <h2>One CLI. Four hardware worlds.</h2>
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
          <p className="kicker">03 · Small command surface, lots of power</p>
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

      <section className="section updateSection" id="updates">
        <div className="sectionHead compact">
          <p className="kicker">04 · Stay current without starting over</p>
          <h2>New models quickly.<br />New runtimes safely.</h2>
          <p>The model catalog and Tapioca itself update separately. Your downloaded models, LoRAs, voices, and generated media stay in place.</p>
        </div>
        <div className="updateGrid">
          <article><span>Fast path</span><h3>Refresh model recipes</h3><code>tapioca catalog update</code><p>Downloads a small JSON manifest and checksum, validates repositories, variants, backends, and paths, then caches it locally. If anything is invalid, Tapioca keeps using its built-in offline catalog.</p></article>
          <article><span>Software path</span><h3>Update Tapioca</h3><code>tapioca update --check<br />tapioca update</code><p>Selects the correct GitHub Release for your operating system and architecture, verifies SHA-256, then replaces the CLI and runtime without touching <code>~/.tapioca</code>.</p></article>
          <article><span>Desktop path</span><h3>Click Update now</h3><code>Settings → Software updates</code><p>The app checks at startup, shows a banner when a newer version exists, and also provides <b>Refresh catalog</b> in Settings. macOS, Windows, and Linux receive platform-specific verified packages.</p></article>
        </div>
      </section>

      <section className="section studio">
        <div>
          <p className="kicker light">05 · The local studio</p>
          <h2>Words in.<br />Pixels and motion out.</h2>
          <p>
            Tapioca understands platform-native diffusion: MLX and MFLUX on Mac,
            CUDA and DirectML on Windows, native ONNX on Windows ARM64, and
            CUDA media generation on Linux.
          </p>
		  <p>
			Krea 2 Turbo adds fast 8-step 1K image generation on NVIDIA and,
			experimentally, high-memory Apple Silicon. Tapioca shows its gated license
			before download and never stores the user&apos;s Hugging Face token. Provider
			approval and local <code>--accept-license</code> acknowledgement are separate steps.
		  </p>
          <p>
            Ask for video length with <code>--seconds</code> and let Tapioca choose a
            valid frame count. Explore image or video variations with
            <code>--random-seed</code>, then reuse the printed value with
            <code>--seed</code> when you want the same generation settings again.
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

      <section className="section voices" id="voices">
        <div className="voiceIntro">
          <p className="kicker">06 · Local voices</p>
          <h2>A voice you can keep.</h2>
          <p>
            Turn text into speech or save a short, consented recording as a
            reusable local voice. Samples, transcripts, model weights, and
            generated audio stay on your computer.
          </p>
          <div className="voiceModels">
            <span><b>Chatterbox Nano</b>Portable, fast English</span>
            <span><b>Chatterbox Multilingual</b>23+ languages</span>
            <span><b>Qwen3-TTS MLX</b>Apple Silicon quality</span>
          </div>
          <a className="textLink" href={`${repo}/blob/main/docs/guides/speech-and-voices.md`}>Follow the voice guide →</a>
        </div>
        <div className="voiceFlow">
          <div className="wave" aria-hidden="true">
            {[18, 34, 52, 27, 66, 42, 74, 38, 58, 24, 48, 30, 64, 36, 20].map((height, index) => (
              <i key={index} style={{ height }} />
            ))}
          </div>
          <p><span>1</span> Save a voice you have permission to use</p>
          <code>tapioca voice create narrator \<br />  --model qwen3-tts \<br />  --audio voice.wav \<br />  --transcript-file voice.txt</code>
          <p><span>2</span> Use it whenever you need it</p>
          <code>tapioca tts qwen3-tts \<br />  --voice narrator \<br />  --text &quot;Welcome home.&quot; \<br />  --output welcome.wav</code>
          <small>Chatterbox audio includes its built-in imperceptible watermark.</small>
        </div>
      </section>

      <section className="section linux">
        <div className="sectionHead compact">
          <p className="kicker">07 · Laptop to cloud</p>
          <h2>The same commands on your Linux GPU box.</h2>
          <p>
            Ship a local workflow to an x64 or ARM64 workstation, headless
            server, or cloud machine. Vulkan runs GGUF text models; NVIDIA CUDA
            powers image, video, and speech backends.
          </p>
        </div>
        <div className="linuxLayout">
          <div className="installCard">
            <div><i /><i /><i /><span>bash</span></div>
            <code>curl -fsSL https://tapioca.rootfruit.cc/install.sh | sh</code>
            <code className="success">✓ installed tapioca for linux/amd64</code>
            <code>tapioca catalog update</code>
            <code>tapioca catalog</code>
          </div>
          <div className="linuxPoints">
            <article><span>01</span><div><b>Native releases</b><p>Linux x64 and ARM64 archives bundle the matching runtime.</p></div></article>
            <article><span>02</span><div><b>GPU aware</b><p>Use Vulkan for text and CUDA for diffusion, video, and TTS.</p></div></article>
            <article><span>03</span><div><b>Agent ready</b><p>Point Codex, Claude Code, OpenCode, Hermes, or OpenClaw at the same local API.</p></div></article>
            <a className="textLink" href={`${repo}/blob/main/docs/guides/linux.md`}>Set up Linux step by step →</a>
          </div>
        </div>
      </section>

      <section className="section experts" id="experts">
        <div className="sectionHead">
          <p className="kicker">08 · Expert lane</p>
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
              <span>Speech runtimes<small>MLX / MPS / CUDA</small></span>
            </div>
          </div>
          <div className="expertList">
            {expertPoints.map(([title, copy], index) => (
              <article key={title}><span>0{index + 1}</span><div><h3>{title}</h3><p>{copy}</p></div></article>
            ))}
          </div>
        </div>
      </section>

      <section className="section community" aria-labelledby="community-title">
        <div className="communityCopy">
          <p className="kicker light">The Tapioca community</p>
          <h2 id="community-title">Questions are welcome.<br />So are weird creations.</h2>
          <p>
            Join us on Discord to get installation help, compare local model
            setups, share what you make, and report issues with the people
            building Tapioca.
          </p>
          <a className="button cream" href={discord}>Join us on Discord ↗</a>
        </div>
        <div className="communityRooms" aria-label="Discord community channels">
          <article><span>#</span><div><b>help-and-support</b><p>Bring your OS, hardware, Tapioca version, and exact error.</p></div></article>
          <article><span>#</span><div><b>showcase</b><p>Share images, videos, voices, agents, and useful model setups.</p></div></article>
          <article><span>#</span><div><b>ideas-and-feedback</b><p>Shape the catalog, documentation, desktop app, and next features.</p></div></article>
        </div>
      </section>

      <section className="cta">
        <img src="/tapioca.png" alt="" />
        <p className="kicker light">Your machine has been waiting</p>
        <h2>Ready to roll?</h2>
        <p>Start small. Pull a model. Make something weird and wonderful.</p>
        <div className="actions">
          <a className="button cream" href="/learn">Start with the beginner guide</a>
          <a className="button darkGhost" href={`${repo}/tree/main/docs`}>Read all the docs</a>
        </div>
      </section>

      <footer>
        <a className="brand" href="#top"><img src="/tapioca.png" alt="" /><span>tapioca</span></a>
        <p>Local AI should feel like yours. Built in the open.</p>
        <div><a href="/learn">For beginners</a><a href="/import">Import & transfer</a><a href="/llm">For agents</a><a href={discord}>Discord</a><a href={repo}>GitHub</a><a href={`${repo}/releases`}>Releases</a><a href={`${repo}/issues`}>Issues</a><a href={`${repo}/blob/main/LICENSE`}>Apache-2.0</a></div>
      </footer>
    </main>
  );
}
