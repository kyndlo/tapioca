import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import {
  defaultCreatorSettings,
  modeLabel,
  moveLora,
  parseHfLoraReference,
  validateCreatorRequest,
  videoDurationSeconds,
  videoFramesForDuration,
} from "./state";
import type {
  CreatorAdapter,
  CreatorAdvancedSettings,
  CreatorLora,
  CreatorLoraOption,
  CreatorMode,
  CreatorModel,
  CreatorOutput,
  CreatorRequest,
  LocalFileKind,
  LocalFileSelection,
} from "./types";
import { creatorModes } from "./types";
import { VoiceRecorder } from "./VoiceRecorder";
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
  const [loraOptions, setLoraOptions] = useState<CreatorLoraOption[]>([]);
  const [selectedLoraReference, setSelectedLoraReference] = useState("");
  const [loadingLoras, setLoadingLoras] = useState(false);
  const [hfReference, setHfReference] = useState("");
  const [loraError, setLoraError] = useState<string>();
  const [settings, setSettings] = useState(defaultCreatorSettings);
  const [outputs, setOutputs] = useState<CreatorOutput[]>([]);
  const [progress, setProgress] = useState<number>();
  const [progressEstimated, setProgressEstimated] = useState(false);
  const [elapsedSeconds, setElapsedSeconds] = useState(0);
  const [progressMessage, setProgressMessage] = useState("");
  const [previewUrl, setPreviewUrl] = useState<string>();
  const [jobId, setJobId] = useState<string>();
  const [lastRequest, setLastRequest] = useState<CreatorRequest>();
  const [loading, setLoading] = useState(true);
  const [generating, setGenerating] = useState(false);
  const [error, setError] = useState<string>();
  const generationStartedAt = useRef(0);
  const durationEstimateMs = useRef(0);

  useEffect(() => {
    setMode(initialMode);
    setError(undefined);
    setProgress(undefined);
    setProgressEstimated(false);
    setElapsedSeconds(0);
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

  useEffect(() => {
    if (!selectedModel?.supportsLoRA) {
      setLoraOptions([]);
      setSelectedLoraReference("");
      return;
    }
    let active = true;
    setLoadingLoras(true);
    void adapter.availableLoras(selectedModel.id)
      .then((options) => {
        if (!active) return;
        setLoraOptions(options);
        setSelectedLoraReference((current) =>
          options.some((option) => option.reference === current)
            ? current
            : (options.find((option) => option.compatible)?.reference ?? ""),
        );
      })
      .catch(() => active && setLoraOptions([]))
      .finally(() => active && setLoadingLoras(false));
    return () => { active = false; };
  }, [adapter, selectedModel?.id, selectedModel?.supportsLoRA]);

  useEffect(() => {
    if (!generating || !generationStartedAt.current) return;
    const update = () => {
      const elapsed = Date.now() - generationStartedAt.current;
      setElapsedSeconds(elapsed / 1_000);
      if (progressEstimated) {
        setProgress(estimatedGenerationProgress(elapsed, durationEstimateMs.current));
      }
    };
    update();
    const timer = setInterval(update, 500);
    return () => clearInterval(timer);
  }, [generating, progressEstimated]);

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

  const addInstalledLora = () => {
    const option = loraOptions.find(({ reference }) => reference === selectedLoraReference);
    if (!option) return;
    if (!option.compatible) {
      setLoraError(option.reason ?? "This LoRA does not match the selected model family.");
      return;
    }
    if (loras.some((lora) => lora.source.type === "huggingface" && lora.source.reference === option.reference)) {
      setLoraError("That LoRA is already assigned.");
      return;
    }
    setLoras((current) => [...current, {
      id: crypto.randomUUID(),
      source: { type: "huggingface", reference: option.reference },
      weight: 0.8,
    }]);
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
    generationStartedAt.current = Date.now();
    durationEstimateMs.current = readDurationEstimate(generationRequest);
    setElapsedSeconds(0);
    setProgress(0.02);
    setProgressEstimated(true);
    setProgressMessage("Preparing local runtime…");
    setPreviewUrl(undefined);
    setError(undefined);
    setLastRequest(generationRequest);
    try {
      const result = await adapter.generate(generationRequest, (event) => {
        if (event.type === "queued") setProgressMessage(event.message ?? "Queued locally…");
        if (event.type === "progress") {
          setProgress(Math.max(0, Math.min(1, event.progress)));
          setProgressEstimated(false);
          setProgressMessage(event.message ?? "Generating…");
        }
        if (event.type === "preview") setPreviewUrl(event.previewUrl);
        if (event.type === "completed") {
          setProgress(1);
          setProgressEstimated(false);
          rememberDuration(generationRequest, Date.now() - generationStartedAt.current);
          setOutputs((current) => [event.output, ...current]);
          setGenerating(false);
          setJobId(undefined);
        }
        if (event.type === "error") {
          setError(event.message);
          setGenerating(false);
          setJobId(undefined);
          setProgressEstimated(false);
        }
      });
      setJobId(result.jobId);
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : String(cause));
      setGenerating(false);
      setJobId(undefined);
      setProgressEstimated(false);
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
      setProgressEstimated(false);
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
            {(mode === "voice-clone" || mode === "speech") && <>
              <VoiceRecorder
                adapter={adapter}
                disabled={generating}
                selection={voiceReference}
                onRecorded={setVoiceReference}
                onError={setError}
              />
              <div className="creator-reference-divider"><span>or use an existing recording</span></div>
              <FilePicker title={selectedModel?.requiresVoiceReference ? "Voice reference (required)" : "Voice reference (optional)"} description="Choose a clear WAV, MP3, FLAC, M4A, or OGG recording." file={voiceReference} accept="audio" onPick={() => void pickFile("audio", setVoiceReference)} onClear={() => setVoiceReference(undefined)} />
            </>}
          </section>

          {mode === "image" && (
            <ImageSetup
              model={selectedModel}
              settings={settings}
              disabled={generating}
              onChange={setSettings}
            />
          )}

          {mode === "video" && (
            <VideoSetup
              model={selectedModel}
              settings={settings}
              disabled={generating}
              onChange={setSettings}
            />
          )}

          {(mode === "image" || mode === "video") && selectedModel?.supportsLoRA !== false && (
            <LoraEditor
              loras={loras}
              reference={hfReference}
              error={loraError}
              options={loraOptions}
              selectedReference={selectedLoraReference}
              loadingOptions={loadingLoras}
              disabled={generating}
              onReference={setHfReference}
              onAddReference={addHfLora}
              onSelectedReference={setSelectedLoraReference}
              onAddInstalled={addInstalledLora}
              onRemove={(id) => setLoras((current) => current.filter((lora) => lora.id !== id))}
              onMove={(id, direction) => setLoras((current) => moveLora(current, id, direction))}
              onWeight={(id, weight) => setLoras((current) => current.map((lora) => lora.id === id ? { ...lora, weight } : lora))}
            />
          )}

          {(mode === "image" || mode === "video") && <AdvancedSettings mode={mode} settings={settings} disabled={generating} onChange={setSettings} />}

          <div className="creator-generate">
            <div>
              <strong>Ready to create locally</strong>
              {mode === "video" ? (
                <span className="creator-generate__summary">
                  {settings.width}×{settings.height} · {formatVideoDuration(videoDurationSeconds(settings.frames, settings.fps))} · {settings.frames} frames · {settings.steps} steps
                </span>
              ) : mode === "image" ? (
                <span className="creator-generate__summary">
                  {settings.width}×{settings.height} · {formatMegapixels(settings.width, settings.height)} · {settings.steps} steps
                </span>
              ) : null}
              <span>No prompt, reference, or output is uploaded by Tapioca.</span>
            </div>
            {generating ? <button type="button" className="creator-cancel" onClick={() => void cancel()} disabled={!jobId}>Cancel</button> : <button type="button" onClick={() => void generate()} disabled={loading || !selectedModel?.ready}>Generate</button>}
          </div>
        </div>

        <aside className="creator-results" aria-label="Generated outputs">
          <header><div><h2>Outputs</h2><span>{outputs.length} local</span></div>{lastRequest && !generating && <button type="button" onClick={() => void generate(lastRequest)}>Retry last</button>}</header>
          {generating && <div className="creator-progress" role="status">
            <div className="creator-progress__preview">{previewUrl ? <img src={previewUrl} alt="Generation preview" /> : <span aria-hidden="true">✦</span>}</div>
            <div className="creator-progress__headline"><div><strong>{progressMessage}</strong><small>{progressEstimated ? "Estimated from this model and your previous runs" : "Reported by the local runtime"}</small></div><b>{Math.round((progress ?? 0) * 100)}%</b></div>
            <progress max={1} value={progress ?? 0} />
            <div className="creator-progress__facts"><span>{formatElapsed(elapsedSeconds)} elapsed</span><span>{progressEstimated ? "Estimate adjusts after every completed run" : "Live progress"}</span></div>
          </div>}
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

function LoraEditor(props: {
  loras: CreatorLora[];
  options: CreatorLoraOption[];
  selectedReference: string;
  loadingOptions: boolean;
  reference: string;
  error?: string;
  disabled: boolean;
  onSelectedReference(value: string): void;
  onAddInstalled(): void;
  onReference(value: string): void;
  onAddReference(): void;
  onRemove(id: string): void;
  onMove(id: string, direction: -1 | 1): void;
  onWeight(id: string, weight: number): void;
}) {
  return (
    <section className="creator-card">
      <div className="creator-card__heading"><h2>LoRA styles</h2><span>Optional · up to 8</span></div>
      <p className="creator-help">Assign adapters already installed in Tapioca. Compatibility hints prevent the most common base-model mismatch.</p>
      <div className="creator-lora-library">
        <label>
          <span>Installed LoRA</span>
          <select
            value={props.selectedReference}
            onChange={(event) => props.onSelectedReference(event.target.value)}
            disabled={props.disabled || props.loadingOptions || !props.options.length}
          >
            {props.loadingOptions ? <option value="">Loading installed LoRAs…</option> : null}
            {!props.loadingOptions && !props.options.length ? <option value="">No installed LoRAs found</option> : null}
            {props.options.map((option) => (
              <option key={option.reference} value={option.reference} disabled={!option.compatible}>
                {option.name} · {formatBytes(option.bytes)}{option.compatible ? "" : " · incompatible"}
              </option>
            ))}
          </select>
        </label>
        <button type="button" onClick={props.onAddInstalled} disabled={props.disabled || !props.selectedReference}>Assign LoRA</button>
      </div>
      {props.selectedReference ? <p className="creator-lora-hint">{props.options.find((option) => option.reference === props.selectedReference)?.reason}</p> : null}
      {!props.options.length && !props.loadingOptions ? (
        <p className="creator-lora-empty">Install one first with <code>tapioca adapter pull hf://OWNER/REPOSITORY --file adapter.safetensors</code></p>
      ) : null}
      <details className="creator-lora-manual">
        <summary>Assign by Hugging Face reference</summary>
        <div className="creator-lora-add">
          <input value={props.reference} onChange={(event) => props.onReference(event.target.value)} placeholder="hf://creator/repository@0.8" disabled={props.disabled} aria-label="Hugging Face LoRA reference" />
          <button type="button" onClick={props.onAddReference} disabled={props.disabled || !props.reference.trim()}>Add reference</button>
        </div>
      </details>
      {props.error && <p className="creator-inline-error" role="alert">{props.error}</p>}
      {props.loras.length ? (
        <ol className="creator-loras">
          {props.loras.map((lora, index) => (
            <li key={lora.id}>
              <span>{index + 1}</span>
              <div><strong>{lora.source.type === "huggingface" ? lora.source.reference : lora.source.file.name}</strong><small>Applied in this order</small></div>
              <label>
                Strength
                <input type="range" min={0} max={2} step={0.05} value={lora.weight} onChange={(event) => props.onWeight(lora.id, Number(event.target.value))} disabled={props.disabled} />
                <output>{lora.weight.toFixed(2)}</output>
              </label>
              <div className="creator-lora-actions">
                <button type="button" onClick={() => props.onMove(lora.id, -1)} disabled={props.disabled || index === 0} aria-label="Move LoRA up">↑</button>
                <button type="button" onClick={() => props.onMove(lora.id, 1)} disabled={props.disabled || index === props.loras.length - 1} aria-label="Move LoRA down">↓</button>
                <button type="button" onClick={() => props.onRemove(lora.id)} disabled={props.disabled} aria-label="Remove LoRA">×</button>
              </div>
            </li>
          ))}
        </ol>
      ) : <p className="creator-empty-line">No LoRAs assigned. The base model will be used as-is.</p>}
    </section>
  );
}

function ImageSetup({
  model,
  settings,
  disabled,
  onChange,
}: {
  model?: CreatorModel;
  settings: CreatorAdvancedSettings;
  disabled: boolean;
  onChange(value: CreatorAdvancedSettings): void;
}) {
  const orientation = settings.width === settings.height
    ? "square"
    : settings.width > settings.height
      ? "landscape"
      : "portrait";
  const presets = imageResolutionPresets(orientation, model?.limits);
  const defaultSteps = model?.defaults?.steps ?? 4;
  const qualityPresets = uniqueByValue([
    { label: "Faster", detail: "Quick drafts", value: Math.max(2, Math.round(defaultSteps * 0.55)) },
    { label: "Balanced", detail: "Model default", value: defaultSteps },
    { label: "More detail", detail: "Takes longer", value: Math.min(100, Math.max(defaultSteps + 2, Math.round(defaultSteps * 1.5))) },
  ]);
  const workload = imageWorkload(settings, defaultSteps);
  const selectedPreset = presets.find(({ width, height }) => width === settings.width && height === settings.height);

  const setOrientation = (next: "landscape" | "portrait" | "square") => {
    const nextPresets = imageResolutionPresets(next, model?.limits);
    const preferredId = selectedPreset?.id ?? "balanced";
    const preset = nextPresets.find(({ id }) => id === preferredId) ?? nextPresets[0];
    if (preset) onChange({ ...settings, width: preset.width, height: preset.height });
  };

  return (
    <section className="creator-card video-setup image-setup" aria-labelledby="image-setup-title">
      <div className="creator-card__heading">
        <div>
          <h2 id="image-setup-title">Image setup</h2>
          <p>Choose the final shape, pixel dimensions, and generation quality before creating.</p>
        </div>
        <span className={`video-workload video-workload--${workload.level}`}>{workload.label} workload</span>
      </div>

      <div className="video-summary image-summary" aria-label="Current image settings">
        <div><span>Resolution</span><strong>{settings.width} × {settings.height}</strong><small>exact output pixels</small></div>
        <div><span>Shape</span><strong>{orientation}</strong><small>{imageAspectRatio(settings.width, settings.height)}</small></div>
        <div><span>Image size</span><strong>{formatMegapixels(settings.width, settings.height)}</strong><small>pixels per image</small></div>
        <div><span>Quality</span><strong>{settings.steps} steps</strong><small>more steps take longer</small></div>
      </div>

      <fieldset className="video-choice">
        <legend>Shape</legend>
        <p>Pick the layout first; Tapioca will offer practical resolutions for it.</p>
        <div className="video-segmented" aria-label="Image orientation">
          {(["landscape", "portrait", "square"] as const).map((item) => (
            <button type="button" key={item} className={orientation === item ? "is-selected" : ""} onClick={() => setOrientation(item)} disabled={disabled}>
              {item[0].toUpperCase() + item.slice(1)}
            </button>
          ))}
        </div>
      </fieldset>

      <fieldset className="video-choice">
        <legend>Resolution</legend>
        <p>These are the exact width and height of the saved image. Larger images need more memory.</p>
        <div className="video-choice__buttons video-choice__buttons--resolution">
          {presets.map((preset) => (
            <button
              type="button"
              key={`${preset.id}-${preset.width}x${preset.height}`}
              className={settings.width === preset.width && settings.height === preset.height ? "is-selected" : ""}
              onClick={() => onChange({ ...settings, width: preset.width, height: preset.height })}
              disabled={disabled}
            >
              <strong>{preset.label}</strong>
              <span>{preset.width} × {preset.height} · {preset.detail}</span>
            </button>
          ))}
        </div>
      </fieldset>

      <fieldset className="video-choice">
        <legend>Generation quality</legend>
        <p>Quality steps refine the image. They do not increase its pixel dimensions.</p>
        <div className="video-choice__buttons">
          {qualityPresets.map((preset) => (
            <button type="button" key={preset.value} className={settings.steps === preset.value ? "is-selected" : ""} onClick={() => onChange({ ...settings, steps: preset.value })} disabled={disabled}>
              <strong>{preset.label}</strong>
              <span>{preset.value} steps · {preset.detail}</span>
            </button>
          ))}
        </div>
      </fieldset>

      <p className="video-setup__note">
        Need an exact custom size? Open Advanced settings below. Width and height must remain divisible by 8.
      </p>
    </section>
  );
}

function imageResolutionPresets(
  orientation: "landscape" | "portrait" | "square",
  limits?: CreatorModel["limits"],
) {
  const maxWidth = Math.min(2048, limits?.maxWidth ?? 2048);
  const maxHeight = Math.min(2048, limits?.maxHeight ?? 2048);
  const dimensions = orientation === "square"
    ? [[512, 512], [1024, 1024], [1536, 1536]]
    : orientation === "landscape"
      ? [[768, 512], [1056, 704], [1536, 1024]]
      : [[512, 768], [704, 1056], [1024, 1536]];
  return ["Lower memory", "Balanced", "Higher detail"].flatMap((label, index) => {
    const [width, height] = dimensions[index];
    if (width > maxWidth || height > maxHeight) return [];
    return [{
      id: ["draft", "balanced", "detailed"][index],
      label,
      detail: ["fastest", "recommended", "slower"][index],
      width,
      height,
    }];
  });
}

function imageAspectRatio(width: number, height: number): string {
  const divisor = greatestCommonDivisor(width, height);
  return `${width / divisor}:${height / divisor} aspect ratio`;
}

function greatestCommonDivisor(left: number, right: number): number {
  let a = Math.abs(Math.round(left));
  let b = Math.abs(Math.round(right));
  while (b) [a, b] = [b, a % b];
  return a || 1;
}

function imageWorkload(settings: CreatorAdvancedSettings, defaultSteps: number) {
  const score =
    (settings.width * settings.height) / (1024 * 1024) *
    (settings.steps / Math.max(1, defaultSteps));
  if (score < 0.7) return { level: "light", label: "Light" };
  if (score < 1.6) return { level: "medium", label: "Moderate" };
  if (score < 3.5) return { level: "heavy", label: "Heavy" };
  return { level: "extreme", label: "Very heavy" };
}

function VideoSetup({
  model,
  settings,
  disabled,
  onChange,
}: {
  model?: CreatorModel;
  settings: CreatorAdvancedSettings;
  disabled: boolean;
  onChange(value: CreatorAdvancedSettings): void;
}) {
  const duration = videoDurationSeconds(settings.frames, settings.fps);
  const maxFrames = model?.limits?.maxFrames ?? 513;
  const maxDuration = Math.max(3, Math.floor(maxFrames / settings.fps));
  const orientation = settings.width === settings.height
    ? "square"
    : settings.width > settings.height
      ? "landscape"
      : "portrait";
  const baseWidth = model?.defaults?.width ?? 768;
  const baseHeight = model?.defaults?.height ?? 432;
  const resolutionPresets = videoResolutionPresets(
    orientation,
    baseWidth,
    baseHeight,
    model?.limits,
  );
  const defaultSteps = model?.defaults?.steps ?? 8;
  const qualityPresets = uniqueByValue([
    { label: "Faster", detail: "Fewer passes", value: Math.max(4, Math.round(defaultSteps * 0.55)) },
    { label: "Balanced", detail: "Model default", value: defaultSteps },
    { label: "More detail", detail: "Takes longer", value: Math.min(100, Math.max(defaultSteps + 2, Math.round(defaultSteps * 1.5))) },
  ]);
  const workload = videoWorkload(settings, defaultSteps);

  const setDuration = (seconds: number) => onChange({
    ...settings,
    frames: videoFramesForDuration(model?.id ?? "", seconds, settings.fps, maxFrames),
  });
  const setOrientation = (next: "landscape" | "portrait" | "square") => {
    const balanced = videoResolutionPresets(
      next,
      baseWidth,
      baseHeight,
      model?.limits,
    ).find(({ id }) => id === "balanced");
    if (balanced) onChange({ ...settings, width: balanced.width, height: balanced.height });
  };

  return (
    <section className="creator-card video-setup" aria-labelledby="video-setup-title">
      <div className="creator-card__heading">
        <div>
          <h2 id="video-setup-title">Video setup</h2>
          <p>Choose what viewers will receive. Tapioca handles the model-specific frame math.</p>
        </div>
        <span className={`video-workload video-workload--${workload.level}`}>{workload.label} workload</span>
      </div>

      <div className="video-summary" aria-label="Current video settings">
        <div><span>Duration</span><strong>{formatVideoDuration(duration)}</strong><small>{settings.frames} generated frames</small></div>
        <div><span>Resolution</span><strong>{settings.width} × {settings.height}</strong><small>{orientation} · {formatMegapixels(settings.width, settings.height)} / frame</small></div>
        <div><span>Playback</span><strong>{settings.fps} FPS</strong><small>MP4 video</small></div>
        <div><span>Quality</span><strong>{settings.steps} steps</strong><small>More steps take longer</small></div>
      </div>

      <fieldset className="video-choice">
        <legend>Duration</legend>
        <p>Longer clips generate more frames and take proportionally longer.</p>
        <div className="video-choice__buttons">
          {[3, 5, 10].filter((seconds) => seconds <= maxDuration).map((seconds) => {
            const frames = videoFramesForDuration(model?.id ?? "", seconds, settings.fps, maxFrames);
            const actual = videoDurationSeconds(frames, settings.fps);
            return (
              <button
                type="button"
                key={seconds}
                className={Math.abs(duration - actual) < 0.02 ? "is-selected" : ""}
                onClick={() => setDuration(seconds)}
                disabled={disabled}
              >
                <strong>About {seconds}s</strong>
                <span>{frames} frames · actual {formatVideoDuration(actual)}</span>
              </button>
            );
          })}
        </div>
        <label className="video-duration-slider">
          <span>Custom duration <output>{formatVideoDuration(duration)}</output></span>
          <input
            type="range"
            min={1}
            max={maxDuration}
            step={1}
            value={Math.min(maxDuration, Math.max(1, Math.round(duration)))}
            onChange={(event) => setDuration(Number(event.target.value))}
            disabled={disabled}
          />
        </label>
      </fieldset>

      <fieldset className="video-choice">
        <legend>Shape and resolution</legend>
        <p>Resolution is the exact pixel size of the finished video.</p>
        <div className="video-segmented" aria-label="Video orientation">
          {(["landscape", "portrait", "square"] as const).map((item) => (
            <button type="button" key={item} className={orientation === item ? "is-selected" : ""} onClick={() => setOrientation(item)} disabled={disabled}>
              {item[0].toUpperCase() + item.slice(1)}
            </button>
          ))}
        </div>
        <div className="video-choice__buttons video-choice__buttons--resolution">
          {resolutionPresets.map((preset) => (
            <button
              type="button"
              key={`${preset.id}-${preset.width}x${preset.height}`}
              className={settings.width === preset.width && settings.height === preset.height ? "is-selected" : ""}
              onClick={() => onChange({ ...settings, width: preset.width, height: preset.height })}
              disabled={disabled}
            >
              <strong>{preset.label}</strong>
              <span>{preset.width} × {preset.height} · {preset.detail}</span>
            </button>
          ))}
        </div>
      </fieldset>

      <fieldset className="video-choice">
        <legend>Generation quality</legend>
        <p>This changes denoising passes. It does not change the output resolution.</p>
        <div className="video-choice__buttons">
          {qualityPresets.map((preset) => (
            <button type="button" key={preset.value} className={settings.steps === preset.value ? "is-selected" : ""} onClick={() => onChange({ ...settings, steps: preset.value })} disabled={disabled}>
              <strong>{preset.label}</strong>
              <span>{preset.value} steps · {preset.detail}</span>
            </button>
          ))}
        </div>
      </fieldset>

      <p className="video-setup__note">
        Estimated workload is relative, not a completion-time promise. GPU, model, LoRAs, and first-run setup all affect speed.
      </p>
    </section>
  );
}

function videoResolutionPresets(
  orientation: "landscape" | "portrait" | "square",
  baseWidth: number,
  baseHeight: number,
  limits?: CreatorModel["limits"],
) {
  const maxWidth = Math.min(2048, limits?.maxWidth ?? 2048);
  const maxHeight = Math.min(2048, limits?.maxHeight ?? 2048);
  const orient = (width: number, height: number) => {
    const long = Math.max(width, height);
    const short = Math.min(width, height);
    if (orientation === "square") {
      const size = Math.min(768, long);
      return { width: size, height: size };
    }
    return orientation === "landscape"
      ? { width: long, height: short }
      : { width: short, height: long };
  };
  const clamp = ({ width, height }: { width: number; height: number }) => ({
    width: Math.max(256, Math.min(maxWidth, Math.round(width / 32) * 32)),
    height: Math.max(256, Math.min(maxHeight, Math.round(height / 32) * 32)),
  });
  const draft = clamp(orient(512, 288));
  const balanced = clamp(orient(baseWidth, baseHeight));
  const detailed = clamp(orient(baseWidth * 1.25, baseHeight * 1.25));
  return uniqueByDimensions([
    { id: "draft", label: "Lower memory", detail: "fastest", ...draft },
    { id: "balanced", label: "Model default", detail: "recommended", ...balanced },
    { id: "detailed", label: "Higher detail", detail: "slower", ...detailed },
  ]);
}

function uniqueByDimensions<T extends { width: number; height: number }>(items: T[]): T[] {
  const seen = new Set<string>();
  return items.filter(({ width, height }) => {
    const key = `${width}x${height}`;
    if (seen.has(key)) return false;
    seen.add(key);
    return true;
  });
}

function uniqueByValue<T extends { value: number }>(items: T[]): T[] {
  const seen = new Set<number>();
  return items.filter(({ value }) => {
    if (seen.has(value)) return false;
    seen.add(value);
    return true;
  });
}

function videoWorkload(settings: CreatorAdvancedSettings, defaultSteps: number) {
  const score =
    (settings.width * settings.height) / (512 * 288) *
    (settings.frames / 73) *
    (settings.steps / Math.max(1, defaultSteps));
  if (score < 1.5) return { level: "light", label: "Light" };
  if (score < 4) return { level: "medium", label: "Moderate" };
  if (score < 9) return { level: "heavy", label: "Heavy" };
  return { level: "extreme", label: "Very heavy" };
}

function formatVideoDuration(seconds: number): string {
  if (!Number.isFinite(seconds) || seconds <= 0) return "0s";
  if (seconds < 60) return `${seconds.toFixed(seconds < 10 ? 1 : 0)}s`;
  const minutes = Math.floor(seconds / 60);
  return `${minutes}m ${Math.round(seconds % 60)}s`;
}

function formatMegapixels(width: number, height: number): string {
  return `${((width * height) / 1_000_000).toFixed(2)} MP`;
}

function AdvancedSettings({ mode, settings, disabled, onChange }: { mode: CreatorMode; settings: CreatorAdvancedSettings; disabled: boolean; onChange(value: CreatorAdvancedSettings): void }) {
  const number = (key: keyof CreatorAdvancedSettings, value: string) => onChange({ ...settings, [key]: value === "" && key === "seed" ? undefined : Number(value) });
  const dimensionStep = mode === "video" ? 32 : 8;
  return <details className="creator-card creator-advanced"><summary>{mode === "video" ? "Expert overrides" : "Advanced settings"} <span>{mode === "video" ? "Exact dimensions, frames, playback, and seed" : "Only runtime-supported controls are shown"}</span></summary><div className="creator-settings">{(mode === "image" || mode === "video") && <><label>Width<input type="number" min={256} max={2048} step={dimensionStep} value={settings.width} onChange={(event) => number("width", event.target.value)} disabled={disabled} /></label><label>Height<input type="number" min={256} max={2048} step={dimensionStep} value={settings.height} onChange={(event) => number("height", event.target.value)} disabled={disabled} /></label><label>Steps<input type="number" min={1} max={100} value={settings.steps} onChange={(event) => number("steps", event.target.value)} disabled={disabled} /></label></>}{mode === "video" && <><label>Frames<input type="number" min={1} max={513} value={settings.frames} onChange={(event) => number("frames", event.target.value)} disabled={disabled} /></label><label>FPS<input type="number" min={1} max={60} value={settings.fps} onChange={(event) => number("fps", event.target.value)} disabled={disabled} /></label></>}<label>Seed<input type="number" min={0} placeholder="Random" value={settings.seed ?? ""} onChange={(event) => number("seed", event.target.value)} disabled={disabled} /></label></div>{mode === "video" ? <p className="creator-advanced__hint">Changing frames or FPS updates the duration summary. MiniMax-H3 uses 17n+5 frames; other models may require 4n+1 or 8n+1.</p> : null}</details>;
}

function OutputCard({ output, onReveal, onMetadata }: { output: CreatorOutput; onReveal(): Promise<void>; onMetadata(): Promise<void> }) {
  return <article className="creator-output"><div className="creator-output__media">{output.mediaType === "image" && <img src={output.url} alt={output.prompt || "Generated image"} />}{output.mediaType === "video" && <video src={output.url} controls preload="metadata" />}{output.mediaType === "audio" && <div className="creator-audio"><span aria-hidden="true">≋</span><audio src={output.url} controls preload="metadata" /></div>}</div><div className="creator-output__meta"><div><strong>{output.modelName}</strong><time dateTime={output.createdAt}>{new Date(output.createdAt).toLocaleString()}</time></div>{output.prompt && <p>{output.prompt}</p>}<dl>{Object.entries(output.metadata).slice(0, 4).map(([key, value]) => <div key={key}><dt>{key}</dt><dd>{String(value)}</dd></div>)}</dl><div className="creator-output__actions"><button type="button" onClick={() => void onReveal()}>Reveal file</button><button type="button" onClick={() => void onMetadata()}>Save metadata</button></div></div></article>;
}

function ModeGlyph({ mode }: { mode: CreatorMode }) {
  return <span aria-hidden="true">{{ image: "◇", video: "▷", speech: "≋", "voice-clone": "◉" }[mode]}</span>;
}

function formatElapsed(seconds: number): string {
  const whole = Math.max(0, Math.floor(seconds));
  return `${Math.floor(whole / 60)}:${String(whole % 60).padStart(2, "0")}`;
}

function formatBytes(bytes: number): string {
  const mib = bytes / 1024 ** 2;
  return mib >= 1024 ? `${(mib / 1024).toFixed(1)} GiB` : `${Math.max(1, Math.round(mib))} MiB`;
}

function estimateStorageKey(request: CreatorRequest): string {
  const { width, height, steps, frames } = request.settings;
  return `tapioca.creator.duration.v1:${request.mode}:${request.modelId}:${width}x${height}:${steps}:${frames}`;
}

function readDurationEstimate(request: CreatorRequest): number {
  if (typeof localStorage !== "undefined") {
    const saved = Number(localStorage.getItem(estimateStorageKey(request)));
    if (Number.isFinite(saved) && saved >= 1_000) return saved;
  }
  if (request.mode === "speech" || request.mode === "voice-clone") return 45_000;
  const pixels = request.settings.width * request.settings.height;
  if (request.mode === "image") {
    return Math.max(20_000, 90_000 * (pixels / (512 * 512)) * (request.settings.steps / 4));
  }
  const normalized = request.modelId.toLowerCase();
  const baseline = normalized.includes("minimax-h3")
    ? normalized.includes("mac") ? 24 * 60_000 : 4 * 60_000
    : 6 * 60_000;
  return Math.max(
    30_000,
    baseline * (pixels / (640 * 352)) * (request.settings.steps / 10) * (request.settings.frames / 124),
  );
}

function rememberDuration(request: CreatorRequest, durationMs: number): void {
  if (typeof localStorage === "undefined" || durationMs < 1_000) return;
  const key = estimateStorageKey(request);
  const previous = Number(localStorage.getItem(key));
  const calibrated = Number.isFinite(previous) && previous > 0
    ? Math.round(previous * 0.35 + durationMs * 0.65)
    : Math.round(durationMs);
  localStorage.setItem(key, String(calibrated));
}

function estimatedGenerationProgress(elapsedMs: number, estimateMs: number): number {
  if (!Number.isFinite(estimateMs) || estimateMs <= 0) return 0.02;
  return Math.max(0.02, Math.min(0.95, 0.02 + (elapsedMs / estimateMs) * 0.88));
}
