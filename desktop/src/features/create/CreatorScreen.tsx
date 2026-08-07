import { useCallback, useEffect, useMemo, useState } from "react";
import {
  defaultCreatorSettings,
  modeLabel,
  moveLora,
  parseHfLoraReference,
  validateCreatorRequest,
} from "./state";
import type {
  CreatorAdapter,
  CreatorAdvancedSettings,
  CreatorLora,
  CreatorMode,
  CreatorModel,
  CreatorOutput,
  CreatorRequest,
  LocalFileKind,
  LocalFileSelection,
} from "./types";
import { creatorModes } from "./types";
import "./create.css";

export interface CreatorScreenProps {
  adapter: CreatorAdapter;
  initialMode?: CreatorMode;
  modes?: CreatorMode[];
}

export function CreatorScreen({
  adapter,
  initialMode = "image",
  modes = [...creatorModes],
}: CreatorScreenProps) {
  const [mode, setMode] = useState<CreatorMode>(initialMode);
  const [models, setModels] = useState<CreatorModel[]>([]);
  const [modelId, setModelId] = useState("");
  const [prompt, setPrompt] = useState("");
  const [text, setText] = useState("");
  const [inputImage, setInputImage] = useState<LocalFileSelection>();
  const [voiceReference, setVoiceReference] = useState<LocalFileSelection>();
  const [loras, setLoras] = useState<CreatorLora[]>([]);
  const [hfReference, setHfReference] = useState("");
  const [loraError, setLoraError] = useState<string>();
  const [settings, setSettings] = useState(defaultCreatorSettings);
  const [outputs, setOutputs] = useState<CreatorOutput[]>([]);
  const [progress, setProgress] = useState<number>();
  const [progressMessage, setProgressMessage] = useState("");
  const [previewUrl, setPreviewUrl] = useState<string>();
  const [jobId, setJobId] = useState<string>();
  const [lastRequest, setLastRequest] = useState<CreatorRequest>();
  const [loading, setLoading] = useState(true);
  const [generating, setGenerating] = useState(false);
  const [error, setError] = useState<string>();

  useEffect(() => {
    setMode(initialMode);
    setError(undefined);
    setProgress(undefined);
    setProgressMessage("");
    setPreviewUrl(undefined);
    setJobId(undefined);
    setGenerating(false);
  }, [initialMode]);

  const loadModels = useCallback(async (nextMode: CreatorMode) => {
    setLoading(true);
    setError(undefined);
    try {
      const next = (await adapter.models(nextMode)).filter((model) =>
        model.modes.includes(nextMode),
      );
      setModels(next);
      setModelId((current) =>
        next.some((model) => model.id === current)
          ? current
          : (next.find((model) => model.ready)?.id ?? next[0]?.id ?? ""),
      );
    } catch (cause) {
      setModels([]);
      setError(cause instanceof Error ? cause.message : String(cause));
    } finally {
      setLoading(false);
    }
  }, [adapter]);

  useEffect(() => void loadModels(mode), [loadModels, mode]);
  useEffect(() => {
    void adapter.outputs()
      .then(setOutputs)
      .catch(() => undefined);
  }, [adapter]);

  const selectedModel = useMemo(
    () => models.find((model) => model.id === modelId),
    [modelId, models],
  );

  useEffect(() => {
    if (!selectedModel?.defaults) return;
    setSettings((current) => ({ ...current, ...selectedModel.defaults }));
  }, [selectedModel?.id]);

  const pickFile = async (
    kind: LocalFileKind,
    setter: (selection: LocalFileSelection | undefined) => void,
  ) => {
    setError(undefined);
    try {
      const selection = await adapter.pickFile(kind);
      if (selection) setter(selection);
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : String(cause));
    }
  };

  const addHfLora = () => {
    const parsed = parseHfLoraReference(hfReference);
    if ("error" in parsed) {
      setLoraError(parsed.error);
      return;
    }
    setLoras((current) => [
      ...current,
      {
        id: crypto.randomUUID(),
        source: { type: "huggingface", reference: parsed.reference },
        weight: parsed.weight,
      },
    ]);
    setHfReference("");
    setLoraError(undefined);
  };

  const request = (): CreatorRequest => ({
    mode,
    modelId,
    prompt: prompt.trim(),
    text: text.trim() || undefined,
    inputImage,
    voiceReference,
    loras,
    settings,
  });

  const generate = async (generationRequest = request()) => {
    const validation = validateCreatorRequest(generationRequest);
    if (!selectedModel?.ready) validation.unshift("The selected model is not ready.");
    if (selectedModel?.requiresInputImage && !generationRequest.inputImage) {
      validation.push("This model requires an input image.");
    }
    if (selectedModel?.requiresVoiceReference && !generationRequest.voiceReference) {
      validation.push("This model requires a voice reference recording.");
    }
    if (validation.length) {
      setError(validation.join(" "));
      return;
    }
    setGenerating(true);
    setProgress(undefined);
    setProgressMessage("Preparing local runtime…");
    setPreviewUrl(undefined);
    setError(undefined);
    setLastRequest(generationRequest);
    try {
      const result = await adapter.generate(generationRequest, (event) => {
        if (event.type === "queued") setProgressMessage(event.message ?? "Queued locally…");
        if (event.type === "progress") {
          setProgress(Math.max(0, Math.min(1, event.progress)));
          setProgressMessage(event.message ?? "Generating…");
        }
        if (event.type === "preview") setPreviewUrl(event.previewUrl);
        if (event.type === "completed") {
          setProgress(1);
          setOutputs((current) => [event.output, ...current]);
          setGenerating(false);
          setJobId(undefined);
        }
        if (event.type === "error") {
          setError(event.message);
          setGenerating(false);
          setJobId(undefined);
        }
      });
      setJobId(result.jobId);
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : String(cause));
      setGenerating(false);
      setJobId(undefined);
    }
  };

  const cancel = async () => {
    if (!jobId) return;
    try {
      await adapter.cancel(jobId);
      setProgressMessage("Generation cancelled.");
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : String(cause));
    } finally {
      setGenerating(false);
      setJobId(undefined);
    }
  };

  return (
    <section className="creator" aria-label="Local media creator">
      <header className="creator-header">
        <div>
          <span className="creator-kicker">Private, local generation</span>
          <h1>Create</h1>
          <p>Your prompts and references stay on this machine.</p>
        </div>
        <div className="creator-private"><span aria-hidden="true">●</span> Local processing</div>
      </header>

      <div className="creator-tabs" role="tablist" aria-label="Creation mode">
        {modes.map((item) => (
          <button
            type="button"
            role="tab"
            aria-selected={mode === item}
            key={item}
            onClick={() => setMode(item)}
            disabled={generating}
          >
            <ModeGlyph mode={item} />
            {modeLabel(item)}
          </button>
        ))}
      </div>

      {error && <div className="creator-error" role="alert"><span>{error}</span><button type="button" onClick={() => setError(undefined)}>Dismiss</button></div>}

      <div className="creator-layout">
        <div className="creator-controls">
          <section className="creator-card">
            <div className="creator-card__heading"><h2>Model & input</h2><span>{modeLabel(mode)}</span></div>
            <label className="creator-field">
              <span>Compatible model</span>
              <select value={modelId} onChange={(event) => setModelId(event.target.value)} disabled={loading || generating}>
                {loading && <option value="">Loading catalog…</option>}
                {!loading && models.length === 0 && <option value="">No compatible models</option>}
                {models.map((model) => <option key={model.id} value={model.id} disabled={!model.ready}>{model.name}{model.ready ? "" : " — unavailable"}</option>)}
              </select>
              <small>{selectedModel?.detail ?? "Only models compatible with this workflow are shown."}</small>
            </label>

            {(mode === "image" || mode === "video") && (
              <label className="creator-field">
                <span>Prompt</span>
                <textarea rows={4} value={prompt} onChange={(event) => setPrompt(event.target.value)} placeholder={mode === "image" ? "Describe the image you want to create…" : "Describe the subject, motion, camera, and scene…"} disabled={generating} />
              </label>
            )}
            {(mode === "speech" || mode === "voice-clone") && (
              <label className="creator-field">
                <span>Text to speak</span>
                <textarea rows={4} value={text} onChange={(event) => setText(event.target.value)} placeholder="Type what the local voice should say…" disabled={generating} />
              </label>
            )}

            {(mode === "image" || mode === "video") && selectedModel?.supportsInputImage && <FilePicker title={mode === "video" ? "Start image" : "Input image (optional)"} description={selectedModel.requiresInputImage ? "This model requires an input image." : "Optionally guide generation with an existing image."} file={inputImage} accept="image" onPick={() => void pickFile("image", setInputImage)} onClear={() => setInputImage(undefined)} />}
            {(mode === "voice-clone" || mode === "speech") && <FilePicker title={selectedModel?.requiresVoiceReference ? "Voice reference (required)" : "Voice reference (optional)"} description="Use a clear, consented recording with little background noise." file={voiceReference} accept="audio" onPick={() => void pickFile("audio", setVoiceReference)} onClear={() => setVoiceReference(undefined)} />}
          </section>

          {(mode === "image" || mode === "video") && selectedModel?.supportsLoRA !== false && (
            <LoraEditor
              loras={loras}
              reference={hfReference}
              error={loraError}
              disabled={generating}
              onReference={setHfReference}
              onAddReference={addHfLora}
              onRemove={(id) => setLoras((current) => current.filter((lora) => lora.id !== id))}
              onMove={(id, direction) => setLoras((current) => moveLora(current, id, direction))}
              onWeight={(id, weight) => setLoras((current) => current.map((lora) => lora.id === id ? { ...lora, weight } : lora))}
            />
          )}

          {(mode === "image" || mode === "video") && <AdvancedSettings mode={mode} settings={settings} disabled={generating} onChange={setSettings} />}

          <div className="creator-generate">
            <div><strong>Ready to create locally</strong><span>No prompt, reference, or output is uploaded by Tapioca.</span></div>
            {generating ? <button type="button" className="creator-cancel" onClick={() => void cancel()} disabled={!jobId}>Cancel</button> : <button type="button" onClick={() => void generate()} disabled={loading || !selectedModel?.ready}>Generate</button>}
          </div>
        </div>

        <aside className="creator-results" aria-label="Generated outputs">
          <header><div><h2>Outputs</h2><span>{outputs.length} local</span></div>{lastRequest && !generating && <button type="button" onClick={() => void generate(lastRequest)}>Retry last</button>}</header>
          {generating && <div className="creator-progress" role="status"><div className="creator-progress__preview">{previewUrl ? <img src={previewUrl} alt="Generation preview" /> : <span aria-hidden="true">✦</span>}</div><strong>{progressMessage}</strong><progress max={1} value={progress} /><small>{progress === undefined ? "Runtime has not reported a percentage" : `${Math.round(progress * 100)}%`}</small></div>}
          {!generating && outputs.length === 0 && <div className="creator-empty"><span aria-hidden="true">◇</span><h3>Your local gallery is empty</h3><p>Completed images, videos, and audio will appear here.</p></div>}
          <div className="creator-gallery">
            {outputs.map((output) => <OutputCard key={output.id} output={output} onReveal={() => adapter.reveal(output.id)} onMetadata={() => adapter.saveMetadata(output.id)} />)}
          </div>
        </aside>
      </div>
    </section>
  );
}

function FilePicker(props: { title: string; description: string; file?: LocalFileSelection; accept: "image" | "audio"; onPick(): void; onClear(): void }) {
  return <div className="creator-file"><div>{props.file?.previewUrl && props.accept === "image" ? <img src={props.file.previewUrl} alt="Selected reference" /> : <span aria-hidden="true">{props.accept === "image" ? "▧" : "≋"}</span>}<div><strong>{props.file?.name ?? props.title}</strong><small>{props.file ? "Selected from this computer" : props.description}</small></div></div><div><button type="button" onClick={props.onPick}>{props.file ? "Change" : "Choose file"}</button>{props.file && <button type="button" onClick={props.onClear}>Remove</button>}</div></div>;
}

function LoraEditor(props: { loras: CreatorLora[]; reference: string; error?: string; disabled: boolean; onReference(value: string): void; onAddReference(): void; onRemove(id: string): void; onMove(id: string, direction: -1 | 1): void; onWeight(id: string, weight: number): void }) {
  return <section className="creator-card"><div className="creator-card__heading"><h2>LoRA stack</h2><span>Optional</span></div><p className="creator-help">Use an installed Hugging Face LoRA reference such as <code>hf://creator/cinematic-motion@0.8</code>. Tapioca inspects the reference and the runtime resolves it from its managed adapter store; arbitrary local files are not accepted by control v1.</p><div className="creator-lora-add"><input value={props.reference} onChange={(event) => props.onReference(event.target.value)} placeholder="hf://creator/repository@0.8" disabled={props.disabled} aria-label="Hugging Face LoRA reference" /><button type="button" onClick={props.onAddReference} disabled={props.disabled || !props.reference.trim()}>Add installed reference</button></div>{props.error && <p className="creator-inline-error" role="alert">{props.error}</p>}{props.loras.length ? <ol className="creator-loras">{props.loras.map((lora, index) => <li key={lora.id}><span>{index + 1}</span><div><strong>{lora.source.type === "huggingface" ? lora.source.reference : lora.source.file.name}</strong><small>Managed Hugging Face adapter</small></div><label>Weight<input type="number" min={0} max={2} step={0.05} value={lora.weight} onChange={(event) => props.onWeight(lora.id, Number(event.target.value))} disabled={props.disabled} /></label><div className="creator-lora-actions"><button type="button" onClick={() => props.onMove(lora.id, -1)} disabled={props.disabled || index === 0} aria-label="Move LoRA up">↑</button><button type="button" onClick={() => props.onMove(lora.id, 1)} disabled={props.disabled || index === props.loras.length - 1} aria-label="Move LoRA down">↓</button><button type="button" onClick={() => props.onRemove(lora.id)} disabled={props.disabled} aria-label="Remove LoRA">×</button></div></li>)}</ol> : <p className="creator-empty-line">No LoRAs added. The base model will be used as-is.</p>}</section>;
}

function AdvancedSettings({ mode, settings, disabled, onChange }: { mode: CreatorMode; settings: CreatorAdvancedSettings; disabled: boolean; onChange(value: CreatorAdvancedSettings): void }) {
  const number = (key: keyof CreatorAdvancedSettings, value: string) => onChange({ ...settings, [key]: value === "" && key === "seed" ? undefined : Number(value) });
  return <details className="creator-card creator-advanced"><summary>Advanced settings <span>Only runtime-supported controls are shown</span></summary><div className="creator-settings">{(mode === "image" || mode === "video") && <><label>Width<input type="number" min={256} max={2048} step={8} value={settings.width} onChange={(event) => number("width", event.target.value)} disabled={disabled} /></label><label>Height<input type="number" min={256} max={2048} step={8} value={settings.height} onChange={(event) => number("height", event.target.value)} disabled={disabled} /></label><label>Steps<input type="number" min={1} max={100} value={settings.steps} onChange={(event) => number("steps", event.target.value)} disabled={disabled} /></label></>}{mode === "video" && <><label>Frames<input type="number" min={1} max={241} value={settings.frames} onChange={(event) => number("frames", event.target.value)} disabled={disabled} /></label><label>FPS<input type="number" min={1} max={60} value={settings.fps} onChange={(event) => number("fps", event.target.value)} disabled={disabled} /></label></>}<label>Seed<input type="number" min={0} placeholder="Random" value={settings.seed ?? ""} onChange={(event) => number("seed", event.target.value)} disabled={disabled} /></label></div></details>;
}

function OutputCard({ output, onReveal, onMetadata }: { output: CreatorOutput; onReveal(): Promise<void>; onMetadata(): Promise<void> }) {
  return <article className="creator-output"><div className="creator-output__media">{output.mediaType === "image" && <img src={output.url} alt={output.prompt || "Generated image"} />}{output.mediaType === "video" && <video src={output.url} controls preload="metadata" />}{output.mediaType === "audio" && <div className="creator-audio"><span aria-hidden="true">≋</span><audio src={output.url} controls preload="metadata" /></div>}</div><div className="creator-output__meta"><div><strong>{output.modelName}</strong><time dateTime={output.createdAt}>{new Date(output.createdAt).toLocaleString()}</time></div>{output.prompt && <p>{output.prompt}</p>}<dl>{Object.entries(output.metadata).slice(0, 4).map(([key, value]) => <div key={key}><dt>{key}</dt><dd>{String(value)}</dd></div>)}</dl><div className="creator-output__actions"><button type="button" onClick={() => void onReveal()}>Reveal file</button><button type="button" onClick={() => void onMetadata()}>Save metadata</button></div></div></article>;
}

function ModeGlyph({ mode }: { mode: CreatorMode }) {
  return <span aria-hidden="true">{{ image: "◇", video: "▷", speech: "≋", "voice-clone": "◉" }[mode]}</span>;
}
