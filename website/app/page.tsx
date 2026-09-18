const repo = "https://github.com/kyndlo/tapioca";
const releases = `${repo}/releases/latest`;
const discord = "https://discord.gg/vkFpNvY2ZY";

const routes = [
  { number: "01", title: "Chat with a local model", description: "Pull a model once, then talk to it in the desktop app or terminal.", href: "/learn#chat", command: "tapioca run qwen3:4b-q4_k_m" },
  { number: "02", title: "Make images and video", description: "Choose a model that fits your hardware, set the size or duration, and create locally.", href: "/learn#images", command: "tapioca catalog" },
  { number: "03", title: "Give words a voice", description: "Generate speech or make a reusable voice from a recording you have permission to use.", href: "/learn#voice", command: "tapioca tts chatterbox:nano --text \"Hello!\"" },
  { number: "04", title: "Power your coding agent", description: "Connect Codex, Claude Code, OpenCode, OpenClaw, or Hermes to a compatible local model.", href: "/llm", command: "tapioca launch opencode qwen3-coder:30b-mlx" },
];

const platforms = [
  { title: "Mac", detail: "Apple Silicon · Metal and MLX", href: "/learn#install" },
  { title: "Windows", detail: "x64 and ARM64 · CUDA, DirectML, Vulkan", href: "/learn#install" },
  { title: "Linux", detail: "x64 and ARM64 · CUDA and Vulkan", href: `${repo}/blob/main/docs/guides/linux.md` },
];

export default function Home() {
  return (
    <main className="home">
      <header className="homeHeader" id="top">
        <nav className="homeNav" aria-label="Main navigation">
          <a className="homeBrand" href="#top" aria-label="Tapioca home"><img src="/tapioca.png" alt="" /><span>Tapioca</span></a>
          <div className="homeNavLinks">
            <a href="#create">What you can do</a>
            <a href="/learn">Guides</a>
            <a href="/import">Import models</a>
            <a href="/llm">For agents</a>
            <a href={repo}>GitHub ↗</a>
          </div>
          <a className="homeButton homeButtonPrimary homeNavDownload" href={releases}>Download <span aria-hidden="true">↗</span></a>
        </nav>
      </header>

      <section className="homeHero" aria-labelledby="home-title">
        <div className="homeHeroCopy">
          <p className="homeEyebrow">Local AI, made approachable</p>
          <h1 id="home-title">Make more.<br /><span>On your machine.</span></h1>
          <p className="homeLead">Chat with local models, create images and video, generate voices, and launch coding agents. One friendly desktop app and one small CLI for Mac, Windows, and Linux.</p>
          <div className="homeActions">
            <a className="homeButton homeButtonPrimary" href={releases}>Download Tapioca <span aria-hidden="true">↗</span></a>
            <a className="homeButton homeButtonOutline" href="/learn">Start with the guide <span aria-hidden="true">→</span></a>
          </div>
          <div className="homeHeroNotes"><span>Open source</span><span>Local-first</span><span>No account required</span></div>
        </div>
        <figure className="homeProduct">
          <div className="homeWindowBar"><span className="homeDots"><i /><i /><i /></span><b>Tapioca Desktop</b><span>Local workspace</span></div>
          <img src="/tapioca-desktop-ui.png" alt="Tapioca Desktop showing a local image-generation workspace with model selection and output gallery" />
          <figcaption>Use the desktop app for a visual workflow—or the same engine from your terminal.</figcaption>
        </figure>
      </section>

      <section className="homePaths" id="create" aria-labelledby="create-title">
        <div className="homeSectionTitle"><p className="homeEyebrow">Choose what you want to make</p><h2 id="create-title">A good place to start.</h2><p>No need to learn every backend first. Pick a task, then follow the steps for your computer.</p></div>
        <div className="homePathGrid">
          {routes.map((route) => <a className="homePath" href={route.href} key={route.number}>
            <span className="homePathNumber">{route.number}</span><span className="homePathArrow" aria-hidden="true">↗</span>
            <h3>{route.title}</h3><p>{route.description}</p><code>{route.command}</code>
          </a>)}
        </div>
      </section>

      <section className="homeInstall" id="install" aria-labelledby="install-title">
        <div className="homeSectionTitle"><p className="homeEyebrow">Get started</p><h2 id="install-title">Your machine, your choice.</h2><p>Download the desktop app, or install the CLI. Models and generated files stay in your local Tapioca directory.</p></div>
        <div className="homePlatformGrid">
          {platforms.map((platform) => <a href={platform.href} key={platform.title}><h3>{platform.title}</h3><p>{platform.detail}</p><span>Setup guide <span aria-hidden="true">↗</span></span></a>)}
        </div>
        <div className="homeInstallCommands">
          <div><span>macOS & Linux CLI</span><code>curl -fsSL https://tapioca.rootfruit.cc/install.sh | sh</code></div>
          <div><span>Windows PowerShell CLI</span><code>irm https://tapioca.rootfruit.cc/install.ps1 | iex</code></div>
        </div>
        <p className="homeInstallNote">Installer scripts and release binaries are open for inspection on <a href={repo}>GitHub</a>. The <a href="/learn#install">beginner guide</a> explains each step.</p>
      </section>

      <section className="homeDeepDive" aria-labelledby="deep-title">
        <div><p className="homeEyebrow">Build on what you already have</p><h2 id="deep-title">Go deeper when you&apos;re ready.</h2><p>Refresh model recipes without reinstalling Tapioca. Import downloaded LoRAs, move a model library between computers, or connect an agent through the local API.</p><div className="homeActions"><a className="homeButton homeButtonOutline" href="/import">Import & transfer →</a><a className="homeButton homeButtonOutline" href="/llm">Agent & API guide →</a></div></div>
        <div className="homeCommandPanel"><div><span>Keep the catalog current</span><code>tapioca catalog update</code></div><div><span>Check for a new release</span><code>tapioca update --check</code></div><div><span>Install the verified update</span><code>tapioca update</code></div><p>Updating Tapioca preserves your downloaded models, LoRAs, voices, and generated media.</p></div>
      </section>

      <section className="homeCommunity" aria-labelledby="community-title"><div><p className="homeEyebrow">Made in the open</p><h2 id="community-title">Questions, ideas, and creations welcome.</h2><p>Get help, share a local setup, or help shape what Tapioca supports next.</p></div><div className="homeActions"><a className="homeButton homeButtonPrimary" href={discord}>Join Discord ↗</a><a className="homeButton homeButtonOutline" href={`${repo}/issues`}>View issues ↗</a></div></section>

      <footer className="homeFooter"><a className="homeBrand" href="#top"><img src="/tapioca.png" alt="" /><span>Tapioca</span></a><p>Local AI should feel like yours.</p><div><a href="/learn">Guides</a><a href="/import">Import</a><a href="/llm">Agents</a><a href={repo}>Source</a><a href={`${repo}/blob/main/LICENSE`}>Apache-2.0</a></div></footer>
    </main>
  );
}
