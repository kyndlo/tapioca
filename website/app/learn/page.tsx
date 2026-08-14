import Link from "next/link";

const repo = "https://github.com/kyndlo/tapioca";
const latest = `${repo}/releases/latest`;
const macDesktop = `${latest}/download/tapioca-desktop-macos-arm64.dmg`;
const windowsDesktop = `${latest}/download/tapioca-desktop-windows-amd64.exe`;

const contents = [
  ["install", "Install Tapioca"],
  ["updates", "Keep Tapioca current"],
  ["choose", "Choose by memory"],
  ["chat", "Run an LLM"],
  ["voice", "Clone a voice"],
  ["images", "Generate images"],
  ["video", "Generate video"],
  ["loras", "Choose LoRAs"],
  ["reuse-loras", "Reuse LoRAs"],
  ["help", "Fix common problems"],
];

function Command({ children }: { children: React.ReactNode }) {
  return <pre className="learnCommand"><code>{children}</code></pre>;
}

function Expected({ children }: { children: React.ReactNode }) {
  return <div className="expected"><b>What should happen</b><p>{children}</p></div>;
}

export default function Learn() {
  return (
    <main className="learnPage">
      <nav className="learnNav">
        <Link className="brand" href="/" aria-label="Tapioca home">
          <img src="/tapioca.png" alt="" /><span>tapioca</span>
        </Link>
        <div><a href="#install">Install</a><a href="#updates">Updates</a><a href="/import">Import & transfer</a><a href="#loras">LoRAs</a><a href="/llm">For agents</a><a className="button small ghost" href={repo}>GitHub ↗</a></div>
      </nav>

      <section className="learnHero">
        <div>
          <p className="kicker">Tapioca for complete beginners</p>
          <h1>Local AI,<br /><em>one step at a time.</em></h1>
          <p>
            You do not need to know what CUDA, GGUF, MLX, diffusion, or a LoRA
            means. Start with your computer and the thing you want to make.
            This guide explains every click, command, download, and file.
          </p>
          <div className="actions"><a className="button primary" href="#install">Start with installation →</a><a className="button ghost" href="/import">Reuse existing downloads</a></div>
        </div>
        <div className="learnMap" aria-label="Beginner learning path">
          <span>1</span><div><b>Install</b><small>One app, no account</small></div>
          <span>2</span><div><b>Choose</b><small>Match your memory</small></div>
          <span>3</span><div><b>Create</b><small>Chat, voice, image, video</small></div>
          <span>4</span><div><b>Customize</b><small>Add the right LoRA</small></div>
        </div>
      </section>

      <div className="learnBody">
        <aside className="learnContents">
          <p>On this page</p>
          {contents.map(([id, label], index) => <a key={id} href={`#${id}`}><span>{String(index + 1).padStart(2, "0")}</span>{label}</a>)}
          <small>Everything runs locally. Large model files are downloaded only when you use them.</small>
        </aside>

        <div className="learnGuides">
          <section className="learnSection" id="install">
            <p className="lesson">Lesson 01</p><h2>Install Tapioca</h2>
            <p className="lessonIntro">For most people, the desktop app is the easiest choice. It includes the same engine as the command line but gives you buttons, forms, progress, and a local media gallery.</p>
            <div className="installChoices">
              <article>
                <span className="osBadge">⌘ Apple Silicon</span><h3>Install on a Mac</h3>
                <ol><li>Click the download button.</li><li>Open the downloaded <code>.dmg</code>.</li><li>Drag Tapioca into Applications.</li><li>Open Tapioca and choose <b>Open</b> if macOS asks for confirmation.</li></ol>
                <a className="button primary" href={macDesktop}>Download for Mac</a>
                <details><summary>I prefer Terminal</summary><Command>curl -fsSL https://tapioca.rootfruit.cc/install.sh | sh</Command><p>The installer verifies the download, installs under your user account, and adds Tapioca to new terminal windows.</p></details>
              </article>
              <article>
                <span className="osBadge windows">▦ Windows x64</span><h3>Install on Windows</h3>
                <ol><li>Click the download button.</li><li>Open the downloaded <code>.exe</code>.</li><li>Choose <b>Install for me</b>.</li><li>Allow Windows to finish, then open Tapioca.</li></ol>
                <a className="button primary" href={windowsDesktop}>Download for Windows</a>
                <details><summary>I prefer PowerShell</summary><Command>irm https://tapioca.rootfruit.cc/install.ps1 | iex</Command><p>The installer verifies the archive and adds Tapioca to your user PATH automatically.</p></details>
              </article>
            </div>
            <Expected>Open the desktop app and look for a green <b>Tapioca runtime</b> indicator. In a terminal, <code>tapioca version</code> should print a version number.</Expected>
            <div className="beforeDownload"><b>Before downloading models</b><span>Keep 10–20 GiB of free disk space for beginner text or image models.</span><span>Video models can require 20–50 GiB.</span><span>Model files live in <code>~/.tapioca</code> or <code>%USERPROFILE%\.tapioca</code>.</span></div>
          </section>

          <section className="learnSection" id="updates">
            <p className="lesson">Lesson 02</p><h2>Keep Tapioca current without losing anything</h2>
            <p className="lessonIntro">Tapioca separates small model-catalog updates from larger software updates. Neither one deletes downloaded models, imported LoRAs, cloned voices, or generated files.</p>
            <div className="stepsDetailed">
              <article><span>1</span><div><h3>Refresh model recipes</h3><Command>tapioca catalog update</Command><p>Use this when a newly documented model does not appear. It downloads and validates a small checksummed catalog. Desktop does this automatically at startup and also offers <b>Settings → Refresh catalog</b>.</p></div></article>
              <article><span>2</span><div><h3>Check for a new app version</h3><Command>tapioca update --check</Command><p>This only checks GitHub Releases; it does not change the computer. Desktop checks automatically and displays an update banner.</p></div></article>
              <article><span>3</span><div><h3>Install the verified update</h3><Command>tapioca update</Command><p>The CLI downloads the matching Mac, Windows, or Linux bundle, verifies its SHA-256 checksum, and replaces the executable and runtime. Desktop users click <b>Update now</b>.</p></div></article>
            </div>
            <div className="rule"><span>Which update do I need?</span><p>If the model uses a runtime Tapioca already supports, <code>catalog update</code> is enough. If Tapioca says the model requires a newer runtime or command, install the software update.</p></div>
          </section>

          <section className="learnSection" id="choose">
            <p className="lesson">Lesson 03</p><h2>Choose by memory, not hype</h2>
            <p className="lessonIntro">A model must fit in memory while it runs. Download size is not the same as memory use, so use Tapioca’s memory recommendation as your first filter.</p>
            <div className="memoryTable">
              <div className="memoryHead"><b>Your computer</b><b>Safe first choices</b><b>Avoid at first</b></div>
              <div><b>8–12 GiB memory</b><span><code>qwen3:4b-q4_k_m</code><br /><code>chatterbox:nano</code></span><span>Large image and video models</span></div>
              <div><b>16 GiB memory</b><span><code>qwen3:8b-q4_k_m</code><br />FLUX Klein on Mac<br />SD Turbo on Windows</span><span>30B+ LLMs and MiniMax-H3</span></div>
              <div><b>24–32 GiB memory</b><span>12B–35B quantized LLMs<br />SDXL or LTX Video</span><span>Models recommending 48 GiB+</span></div>
              <div><b>48–96 GiB memory</b><span>Large MLX models<br />Qwen Image Flash<br />MiniMax-H3 on supported hardware</span><span>Anything above the catalog recommendation</span></div>
            </div>
            <Command>{`tapioca catalog update
tapioca catalog`}</Command>
            <p>Read each row from left to right: model name, task, download, memory, GPU, platform, and features. If your platform is not listed, choose another variant.</p>
            <div className="rule"><span>Golden rule</span><p>Start small, confirm it works, and move up one size. A smaller responsive model is more useful than a larger model that makes the computer swap or crash.</p></div>
          </section>

          <section className="learnSection" id="chat">
            <p className="lesson">Lesson 04</p><h2>Run your first LLM</h2>
            <div className="stepsDetailed">
              <article><span>1</span><div><h3>Choose Chat in the app</h3><p>Select <code>qwen3:4b-q4_k_m</code> for the safest first run on Mac or Windows. It needs about 8 GiB memory.</p></div></article>
              <article><span>2</span><div><h3>Send a message</h3><p>Tapioca downloads the model automatically the first time. Keep the app open while the progress bar completes.</p><Command>tapioca run qwen3:4b-q4_k_m</Command></div></article>
              <article><span>3</span><div><h3>Have a conversation</h3><p>Ask follow-up questions normally. The model sees the messages in the current conversation.</p></div></article>
              <article><span>4</span><div><h3>Stop when finished</h3><p>In Terminal, type <code>/bye</code> or press Ctrl-D. This stops the private model server and releases memory.</p></div></article>
            </div>
            <Expected>The first answer may take longer while the model starts. Later messages should begin faster. No prompt or response is uploaded by Tapioca.</Expected>
            <details className="trouble"><summary>The answer is extremely slow</summary><p>Close memory-heavy apps, choose a smaller model, and avoid context settings larger than you need. Confirm the catalog recommends the model for your memory.</p></details>
          </section>

          <section className="learnSection" id="voice">
            <p className="lesson">Lesson 05</p><h2>Clone a voice responsibly</h2>
            <div className="safetyCallout"><span>Permission first</span><p>Clone only your own voice or a voice whose speaker has clearly agreed. Never use a cloned voice to impersonate someone, bypass verification, or mislead listeners.</p></div>
            <h3>Prepare the recording</h3>
            <ul className="checklist"><li>Record one person for 3–10 seconds.</li><li>Use a quiet room with no music or echo.</li><li>Speak naturally and save WAV when possible.</li><li>Write the exact words spoken in the sample.</li></ul>
            <h3>Save the voice</h3>
            <Command>{`tapioca voice create narrator \\\n  --model chatterbox:nano \\\n  --audio ./narrator.wav \\\n  --transcript-file ./narrator.txt`}</Command>
            <p><code>narrator</code> is your private local nickname. The audio is copied into Tapioca’s voice folder so moving the original later will not break it.</p>
            <h3>Generate speech</h3>
            <Command>{`tapioca tts chatterbox:nano \\\n  --voice narrator \\\n  --text "Hello from my local voice." \\\n  --output hello.wav`}</Command>
            <Expected>A playable <code>hello.wav</code> appears in the current folder and in the desktop gallery. The first run takes longer because Tapioca prepares the speech runtime.</Expected>
          </section>

          <section className="learnSection" id="images">
            <p className="lesson">Lesson 06</p><h2>Generate your first image</h2>
            <p className="lessonIntro">Open <b>Images</b>, select the model recommended for your hardware, write what should be visible, and choose Generate. Keep the first prompt simple so failures are easy to diagnose.</p>
            <div className="platformRecipes">
              <article><span>Mac · 16 GiB+</span><h3>FLUX.2 Klein</h3><Command>{`tapioca image flux2-klein:4b-q4-mlx \\\n  --prompt "A red fox in snow" \\\n  --output fox.png`}</Command></article>
              <article><span>Windows · NVIDIA 6 GiB+</span><h3>SD Turbo CUDA</h3><Command>{`tapioca image sd-turbo:fp16 \\\n  --prompt "A red fox in snow" \\\n  --output fox.png`}</Command></article>
              <article><span>Windows · AMD or Intel</span><h3>SD Turbo DirectML</h3><Command>{`tapioca image sd-turbo:onnx-directml \\\n  --prompt "A red fox in snow" \\\n  --output fox.png`}</Command></article>
			  <article><span>96 GiB Mac or NVIDIA 16 GiB+</span><h3>Krea 2 Turbo</h3><Command>{`export HF_TOKEN=hf_your_read_token
tapioca pull krea-2-turbo --accept-license
tapioca image krea-2-turbo --prompt "A glass sculpture" --steps 8 --output art.png`}</Command></article>
			</div>
			<div className="rule"><span>Gated model: two approvals</span><p>First, sign in at Hugging Face and accept Krea&apos;s provider terms. Second, set an approved read token and run <code>--accept-license</code> to record your local Tapioca acknowledgement. That flag does not bypass Hugging Face access. Tapioca never accepts terms for you and does not save the token. Review outputs before sharing them.</p></div>
			<h3>Write a useful prompt</h3><p>Describe <b>subject + setting + lighting + visual style + composition</b>. Example: “A red fox in a snowy pine forest, golden-hour light, detailed wildlife photograph, eye-level portrait.”</p>
            <div className="rule"><span>Explore, then reproduce</span><p>Image, edit, and video use seed <code>0</code> by default. Add <code>--random-seed</code> for a new variation and save the number Tapioca prints. Use that number later as <code>--seed NUMBER</code> to repeat the same generation settings. Do not combine the two seed flags.</p></div>
            <Expected>The first run downloads several gigabytes and prepares a private runtime. A progress bar may pause while files load into memory. Later images reuse both downloads.</Expected>
          </section>

          <section className="learnSection" id="video">
            <p className="lesson">Lesson 07</p><h2>Generate motion and video</h2>
            <p className="lessonIntro">Video needs much more memory and time than images. Start with a short low-memory clip. Add a starting image when identity or composition matters.</p>
            <div className="platformRecipes">
              <article><span>Mac · 32 GiB+</span><h3>Wan 2.2</h3><Command>{`tapioca video wan2.2-video:5b-q8-mlx \\\n  --prompt "A fox running through snow" \\\n  --preset low-memory --output fox.mp4`}</Command></article>
              <article><span>Windows · NVIDIA 8 GiB+</span><h3>LTX Video</h3><Command>{`tapioca video ltx-video:2b-fp16 \\\n  --prompt "A fox running through snow" \\\n  --preset low-memory --output fox.mp4`}</Command></article>
              <article><span>48 GiB Mac or 16 GiB NVIDIA</span><h3>MiniMax-H3 + audio</h3><Command>{`tapioca video minimax-h3 \\\n  --image start.png \\\n  --prompt 'A presenter says exactly: "Hello."' \\\n  --preset low-memory --output hello.mp4`}</Command></article>
            </div>
            <div className="rule"><span>Choose an approximate duration</span><p>Use <code>--seconds 5</code> instead of calculating a model-compatible frame count. Tapioca prints the selected frame count before generation; the final duration can differ slightly because each video family has its own frame rule. Do not combine <code>--seconds</code> with <code>--frames</code>.</p></div>
            <Command>tapioca video minimax-h3 --prompt &quot;A presenter waves&quot; --seconds 5 --output hello.mp4</Command>
            <div className="rule"><span>Long videos</span><p>Local video models are best at short shots. Build a 30–60 second video from several 3–5 second clips, then join them. One enormous generation is slower, less stable, and more likely to drift away from the subject.</p></div>
            <ul className="checklist"><li>Use <code>--image</code> to anchor the first frame.</li><li>Use <code>--preset low-memory</code> for the first test.</li><li>Reduce resolution, frames, or steps when memory runs out.</li><li>MiniMax-H3 makes native stereo audio; most other video models do not.</li></ul>
          </section>

          <section className="learnSection" id="loras">
            <p className="lesson">Lesson 08</p><h2>Choose the correct LoRA</h2>
            <p className="lessonIntro">A LoRA is a small add-on—not a complete model. It can add a style, subject, motion, or editing behavior only when it was trained for the same base-model architecture you are running.</p>
            <div className="loraFormula"><b>base model</b><i>+</i><b>compatible LoRA</b><i>+</i><b>prompt and inputs</b><i>=</i><strong>output</strong></div>
            <h3>The six checks to make before downloading</h3>
            <ol className="loraChecks">
              <li><b>Base model:</b> The model card must name the same family—FLUX Klein, Wan 2.2, MiniMax-H3, SDXL, or another exact architecture.</li>
              <li><b>Task:</b> Image, image editing, or video must match what you are doing.</li>
              <li><b>Runtime:</b> Tapioca supports dynamic LoRAs with MFLUX, CUDA Diffusers, Wan MLX, and MiniMax-H3. ONNX DirectML cannot attach arbitrary LoRAs.</li>
              <li><b>Weight file:</b> Select the exact <code>.safetensors</code> file when a repository contains several.</li>
              <li><b>Inputs:</b> Check how many images are required and their order.</li>
              <li><b>License:</b> Confirm personal or commercial use is permitted for your project.</li>
            </ol>
            <h3>Inspect before pulling</h3>
            <Command>tapioca adapter inspect hf://OWNER/REPOSITORY</Command>
            <Command>tapioca adapter inspect civitai://MODEL_ID/VERSION_ID</Command>
            <Command>tapioca adapter inspect ms://OWNER/REPOSITORY</Command>
            <p>Use <code>hf://</code> for Hugging Face, numeric model/version IDs for Civitai, and <code>ms://</code> for ModelScope. You can also paste a complete Civitai URL containing <code>modelVersionId</code>.</p>
            <h3>Select a specific file</h3>
            <Command>{`tapioca adapter pull hf://OWNER/REPOSITORY \\\n  --file exact-lora-file.safetensors`}</Command>
            <h3>Already downloaded it?</h3>
            <Command>tapioca adapter import ~/Downloads/my-lora.safetensors --base minimax-h3 --name my-lora</Command>
            <p>Import verifies and copies the file into Tapioca. In the desktop app, use <b>Import from computer</b>; Tapioca creates the same managed <code>local://</code> reference for you.</p>
            <h3>Apply it gently</h3>
            <Command>{`tapioca video minimax-h3 \\\n  --adapter 'hf://OWNER/REPOSITORY#exact-lora-file.safetensors@0.8' \\\n  --prompt "A cinematic tracking shot" \\\n  --preset low-memory --output adapted.mp4`}</Command>
            <p>The <code>@0.8</code> is strength. Start around 0.7–0.9. If the output becomes distorted, lower it. Test one LoRA before stacking multiple adapters.</p>
            <h3>Supported sources and safeguards</h3>
            <ul className="loraChecks">
              <li><b>Hugging Face:</b> Use <code>hf://OWNER/REPOSITORY</code>. Private repositories can use <code>HF_TOKEN</code>.</li>
              <li><b>Civitai:</b> Use <code>civitai://MODEL_ID/VERSION_ID</code> or paste the complete version URL. Tapioca rejects checkpoint models when a LoRA is required.</li>
              <li><b>ModelScope:</b> Use <code>ms://OWNER/REPOSITORY</code>. <code>modelscope://</code> is also accepted.</li>
              <li><b>Local files:</b> Import regular <code>.safetensors</code> files. Tapioca verifies the file, records its hash and base family, and rejects unsafe paths.</li>
            </ul>
            <Command>{`tapioca adapter pull civitai://MODEL_ID/VERSION_ID#adapter.safetensors
tapioca adapter pull ms://OWNER/REPOSITORY#adapter.safetensors`}</Command>
            <p>Private sources use environment tokens: <code>HF_TOKEN</code>, <code>CIVITAI_TOKEN</code>, or <code>MODELSCOPE_API_TOKEN</code>. Tapioca never stores these tokens in adapter references or snapshot files.</p>
            <div className="rule"><span>Verified downloads</span><p>Tapioca downloads into a temporary file, checks the provider checksum when available, validates the safetensors header, and only then moves the file into the managed library.</p></div>
            <div className="loraWrong"><b>File extension does not prove compatibility</b><p>Two files can both end in <code>.safetensors</code> while containing completely different tensor shapes. “It downloads” does not mean “it works with this base model.”</p></div>
          </section>

          <section className="learnSection" id="reuse-loras">
            <p className="lesson">Lesson 09</p><h2>Reuse downloaded LoRAs</h2>
            <p className="lessonIntro">A LoRA only needs to be downloaded or imported once. Tapioca keeps it in its managed adapter library and reuses the local copy whenever you select the same reference.</p>
            <h3>1. See what is already installed</h3>
            <Command>tapioca adapter list</Command>
            <Expected>The list shows a reusable reference, its provider, and the exact managed path. Copy the reference from the first column; do not reconstruct it from the filesystem path.</Expected>
            <h3>2. Use the reference again</h3>
            <Command>{`tapioca video minimax-h3 \
  --adapter 'local://my-lora#my-lora.safetensors@0.8' \
  --prompt "A cinematic tracking shot" \
  --preset low-memory --output reused.mp4`}</Command>
            <p>The same rule applies to an installed <code>hf://</code>, <code>civitai://</code>, or <code>ms://</code> reference. Tapioca detects the cached file and does not download it again. Changing only <code>@0.8</code> changes strength; it does not create another copy.</p>
            <h3>Reuse it in the desktop app</h3>
            <ol className="loraChecks">
              <li><b>Open Images or Video:</b> Choose a base model that supports LoRAs.</li>
              <li><b>Find LoRA styles:</b> The Installed LoRA menu includes files pulled from providers and files imported from your computer.</li>
              <li><b>Assign LoRA:</b> Choose it, click <b>Assign LoRA</b>, and adjust Strength. You can reorder up to eight adapters.</li>
              <li><b>Generate:</b> Tapioca reuses the managed file. Provider references that are not installed yet are verified and installed automatically.</li>
            </ol>
            <h3>If the file was downloaded outside Tapioca</h3>
            <Command>tapioca adapter import ~/Downloads/style.safetensors --base minimax-h3 --name style</Command>
            <p>Use <code>adapter import</code>, not <code>--file</code>, for an existing computer file. The <code>--file</code> option selects a file inside a provider repository. Import copies the weights into the managed library without changing the original.</p>
            <h3>Move an adapter library to another computer</h3>
            <p>Copy the complete <code>adapters</code> directory—including each <code>snapshot.json</code>—into the other computer&apos;s Tapioca home. The default is <code>~/.tapioca/adapters</code> on macOS/Linux and <code>%USERPROFILE%\.tapioca\adapters</code> on Windows. Alternatively, import each raw safetensors file again and declare its base model.</p>
            <div className="rule"><span>Keep metadata together</span><p>Do not move only individual managed weight files or rename folders inside the adapter library. The snapshot records provider, checksum, revision, and compatibility information used for safe reuse.</p></div>
            <p><a className="button primary" href="/import">Open the complete import and computer-transfer guide →</a></p>
          </section>

          <section className="learnSection" id="help">
            <p className="lesson">Lesson 10</p><h2>Fix common beginner problems</h2>
            <div className="faq">
              <details><summary>Nothing seems to happen</summary><p>Look for download or runtime preparation progress. First runs can take minutes. Keep the app open and confirm free disk space.</p></details>
              <details><summary>The model is not listed</summary><p>Run <code>tapioca catalog update</code>, then <code>tapioca catalog</code>. Desktop users can choose <b>Refresh catalog</b> in Settings. The verified remote catalog can add recipes for existing runtimes without reinstalling Tapioca; a brand-new runtime still needs <code>tapioca update</code>.</p></details>
			  <details><summary>A gated Hugging Face model says access denied</summary><p>Open its Hugging Face page while signed in, accept the provider terms, create a read token, set <code>HF_TOKEN</code>, and pull once with <code>--accept-license</code>. The token must be present in the same terminal that launches Tapioca.</p></details>
			  <details><summary>Windows is not using NVIDIA</summary><p>Install a current NVIDIA driver, run <code>nvidia-smi</code>, close GPU-heavy apps, and assign Tapioca to the high-performance GPU in Windows Graphics settings.</p></details>
              <details><summary>The computer runs out of memory</summary><p>Choose a smaller model, use a lower quantization, select low-memory, and reduce video resolution or frames.</p></details>
              <details><summary>The LoRA fails or distorts everything</summary><p>Recheck the exact base architecture and weight file. Then test the base model alone and retry one LoRA at a lower strength.</p></details>
            </div>
            <div className="finishCard"><img src="/tapioca.png" alt="" /><div><b>You are ready to roll.</b><p>Start with one small model and one simple output. The advanced controls will make more sense after the basic loop works once.</p><Link href="/">Return to Tapioca home →</Link></div></div>
          </section>
        </div>
      </div>

      <footer className="learnFooter"><Link className="brand" href="/"><img src="/tapioca.png" alt="" /><span>tapioca</span></Link><p>Beginner guide · Local by default</p><div><a href="/import">Import & transfer</a><a href="/llm">For agents</a><a href={`${repo}/tree/main/docs`}>Full reference</a><a href={`${repo}/issues`}>Get help</a></div></footer>
    </main>
  );
}
