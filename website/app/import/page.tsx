import type { Metadata } from "next";
import Link from "next/link";

const repo = "https://github.com/kyndlo/tapioca";

export const metadata: Metadata = {
  title: "Import and transfer LoRAs and models — Tapioca",
  description: "Reuse existing LoRA and model downloads with Tapioca, move libraries between computers, and avoid downloading large weights twice.",
};

function Command({ children }: { children: React.ReactNode }) {
  return <pre className="learnCommand"><code>{children}</code></pre>;
}

function Expected({ children }: { children: React.ReactNode }) {
  return <div className="expected"><b>What should happen</b><p>{children}</p></div>;
}

const contents = [
  ["one-lora", "Import one LoRA"],
  ["desktop", "Use the desktop app"],
  ["all-loras", "Move your LoRA library"],
  ["models", "Transfer base models"],
  ["external", "Use an external drive"],
  ["verify", "Verify the transfer"],
];

export default function ImportAndTransfer() {
  return (
    <main className="learnPage">
      <nav className="learnNav">
        <Link className="brand" href="/" aria-label="Tapioca home">
          <img src="/tapioca.png" alt="" /><span>tapioca</span>
        </Link>
        <div><a href="#one-lora">Import LoRA</a><a href="#models">Transfer models</a><a href="/learn">Beginner guide</a><a className="button small ghost" href={repo}>GitHub ↗</a></div>
      </nav>

      <section className="learnHero importHero">
        <div>
          <p className="kicker">Keep the weights you already have</p>
          <h1>Import once.<br /><em>Move without redownloading.</em></h1>
          <p>
            Bring an existing LoRA into Tapioca, copy a complete adapter
            library to another computer, or reuse a compatible catalog model
            you already downloaded.
          </p>
          <div className="actions"><a className="button primary" href="#one-lora">Import a LoRA →</a><a className="button ghost" href="#models">Transfer a base model</a></div>
        </div>
        <div className="learnMap" aria-label="Import and transfer choices">
          <span>1</span><div><b>Raw LoRA</b><small>Import one .safetensors file</small></div>
          <span>2</span><div><b>LoRA library</b><small>Copy weights with snapshot metadata</small></div>
          <span>3</span><div><b>Base model</b><small>Copy, verify, and register</small></div>
          <span>4</span><div><b>External disk</b><small>Move the complete Tapioca home</small></div>
        </div>
      </section>

      <div className="learnBody">
        <aside className="learnContents">
          <p>Choose your case</p>
          {contents.map(([id, label], index) => <a key={id} href={`#${id}`}><span>{String(index + 1).padStart(2, "0")}</span>{label}</a>)}
          <small>Quit Tapioca before copying managed folders. Keep metadata files with their weights.</small>
        </aside>

        <div className="learnGuides">
          <section className="learnSection" id="one-lora">
            <p className="lesson">Existing download</p><h2>Import one LoRA file</h2>
            <p className="lessonIntro">Use this when a browser, Civitai client, Hugging Face tool, or another application already downloaded a LoRA to your computer.</p>
            <h3>macOS or Linux</h3>
            <Command>{`tapioca adapter import ~/Downloads/cinematic-motion.safetensors \
  --base minimax-h3 \
  --name cinematic-motion`}</Command>
            <h3>Windows PowerShell</h3>
            <Command>{`tapioca adapter import "C:\\Users\\Carlos\\Downloads\\cinematic-motion.safetensors" \`
  --base minimax-h3 \`
  --name cinematic-motion`}</Command>
            <p><code>--base</code> is the model family named on the LoRA&apos;s model card. It is required because a <code>.safetensors</code> extension does not prove compatibility. <code>--name</code> gives the local import an easy reusable name.</p>
            <Expected>Tapioca verifies the safetensors header, calculates a SHA-256 hash, copies the file into its managed library, and prints a <code>local://</code> reference. Your original file is left unchanged.</Expected>
            <h3>Confirm and reuse it</h3>
            <Command>tapioca adapter list</Command>
            <Command>{`tapioca video minimax-h3 \
  --adapter 'local://cinematic-motion#cinematic-motion.safetensors@0.8' \
  --prompt "A cinematic tracking shot" \
  --output adapted.mp4`}</Command>
            <div className="rule"><span>Important difference</span><p><code>adapter import PATH</code> accepts a file already on your computer. <code>--file</code> selects one weight inside a provider repository; it is not a local path option.</p></div>
          </section>

          <section className="learnSection" id="desktop">
            <p className="lesson">No terminal needed</p><h2>Import with Tapioca Desktop</h2>
            <ol className="loraChecks">
              <li><b>Open Images or Video.</b> Choose the compatible base model first.</li>
              <li><b>Find LoRA styles.</b> Select <b>Import from computer</b>.</li>
              <li><b>Choose the file.</b> Select a regular <code>.safetensors</code> LoRA.</li>
              <li><b>Declare the base.</b> Enter the exact base-model family from its model card.</li>
              <li><b>Import and assign.</b> It appears under <b>Installed LoRA</b> and can be reused without another copy.</li>
            </ol>
            <p>Tapioca&apos;s desktop and CLI use the same managed adapter library. An import made in either interface appears in the other.</p>
          </section>

          <section className="learnSection" id="all-loras">
            <p className="lesson">Computer to computer</p><h2>Move your complete LoRA library</h2>
            <p className="lessonIntro">This is faster than importing files one at a time and preserves provider references, hashes, revisions, and compatibility hints.</p>
            <div className="platformRecipes">
              <article><span>macOS and Linux</span><h3>Adapter folder</h3><Command>~/.tapioca/adapters</Command></article>
              <article><span>Windows</span><h3>Adapter folder</h3><Command>%USERPROFILE%\.tapioca\adapters</Command></article>
            </div>
            <ol className="loraChecks">
              <li>Quit Tapioca on both computers.</li>
              <li>Copy the complete <code>adapters</code> directory from the source.</li>
              <li>Place it inside the destination computer&apos;s Tapioca home.</li>
              <li>Keep every <code>snapshot.json</code> next to its weight files.</li>
              <li>Run <code>tapioca adapter list</code> on the destination.</li>
            </ol>
            <div className="loraWrong"><b>Do not separate managed weights from metadata</b><p>If you only have a raw <code>.safetensors</code> file, use <code>adapter import</code>. If you copy Tapioca&apos;s managed library, copy each whole folder including <code>snapshot.json</code>.</p></div>
          </section>

          <section className="learnSection" id="models">
            <p className="lesson">Large base weights</p><h2>Transfer an installed base model</h2>
            <p className="lessonIntro">Base-model variants are hardware specific. Transfer only a model compatible with the destination. An Apple Silicon MLX model is not a Windows CUDA model.</p>
            <ol className="loraChecks">
              <li>Run <code>tapioca list</code> on the source and note the exact model ID.</li>
              <li>Quit Tapioca on both computers.</li>
              <li>Copy that model&apos;s complete directory from <code>TAPIOCA_HOME/models</code>.</li>
              <li>Place it under the destination&apos;s <code>TAPIOCA_HOME/models</code> with the same directory name.</li>
              <li>Run <code>tapioca pull MODEL</code> on the destination without <code>--force</code>.</li>
            </ol>
            <Command>{`tapioca pull minimax-h3
tapioca list`}</Command>
            <Expected>Tapioca reuses every matching file, downloads only required files that are missing, and registers the model with paths that are correct for the destination computer.</Expected>
            <div className="rule"><span>Do not copy registry.json alone</span><p>The registry contains absolute paths from the old machine. Let <code>tapioca pull MODEL</code> rebuild model registration after the files are copied.</p></div>
          </section>

          <section className="learnSection" id="external">
            <p className="lesson">Storage control</p><h2>Use an external drive</h2>
            <p className="lessonIntro">Set <code>TAPIOCA_HOME</code> before importing or pulling. Models, adapters, voices, recipes, and managed runtimes will use that location.</p>
            <h3>macOS or Linux</h3>
            <Command>{`export TAPIOCA_HOME="/Volumes/ExternalSSD/tapioca"
tapioca adapter list
tapioca list`}</Command>
            <h3>Windows PowerShell</h3>
            <Command>{`$env:TAPIOCA_HOME = "D:\\Tapioca"
tapioca adapter list
tapioca list`}</Command>
            <p>Add the environment variable permanently in your operating system if every session should use the external disk. Leave space for partial downloads and generated media as well as final weights.</p>
          </section>

          <section className="learnSection" id="verify">
            <p className="lesson">Before cleanup</p><h2>Verify the destination</h2>
            <Command>{`tapioca adapter list
tapioca list
tapioca run MODEL`}</Command>
            <p>For an image or video model, create one small smoke-test output. Delete the old copy only after the destination loads the model and produces a valid result.</p>
            <div className="finishCard"><img src="/tapioca.png" alt="" /><div><b>Your downloads can travel.</b><p>Import raw LoRAs, preserve managed metadata, and let Tapioca verify transferred base-model folders.</p><Link href="/learn">Continue to the beginner guide →</Link></div></div>
          </section>
        </div>
      </div>

      <footer className="learnFooter"><Link className="brand" href="/"><img src="/tapioca.png" alt="" /><span>tapioca</span></Link><p>Import and transfer guide</p><div><a href="/learn">For beginners</a><a href="/llm">For agents</a><a href={`${repo}/blob/main/docs/guides/import-and-transfer.md`}>Markdown guide</a></div></footer>
    </main>
  );
}
