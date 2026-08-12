import {
  useEffect,
  useMemo,
  useRef,
  useState,
  type KeyboardEvent,
} from "react";
import {
  estimatedDiskAfterInstall,
  formatModelBytes,
  modelCompatibility,
} from "./model-utils";
import type {
  MachineProfile,
  ModelHubAdapter,
  ModelKind,
  ModelPlatform,
  ModelRecord,
  PullProgress,
} from "./types";
import "./models.css";

export interface ModelsScreenProps {
  adapter: ModelHubAdapter;
  machine: MachineProfile;
}

interface TransferState extends PullProgress {
  state: "downloading" | "cancelling" | "error";
  message?: string;
}

const kinds: Array<{ value: "all" | ModelKind; label: string }> = [
  { value: "all", label: "All types" },
  { value: "chat", label: "Chat" },
  { value: "image", label: "Images" },
  { value: "video", label: "Video" },
  { value: "speech", label: "Voice" },
];

export function ModelsScreen({ adapter, machine }: ModelsScreenProps) {
  const [models, setModels] = useState<ModelRecord[]>();
  const [query, setQuery] = useState("");
  const [kind, setKind] = useState<"all" | ModelKind>("all");
  const [platform, setPlatform] = useState<"all" | ModelPlatform>("all");
  const [compatibleOnly, setCompatibleOnly] = useState(false);
  const [selected, setSelected] = useState<ModelRecord>();
  const [removeTarget, setRemoveTarget] = useState<ModelRecord>();
  const [transfers, setTransfers] = useState<Record<string, TransferState>>({});
  const [error, setError] = useState<string>();
  const [attempt, setAttempt] = useState(0);
  const pullControllers = useRef(new Map<string, AbortController>());

  useEffect(() => {
    const controller = new AbortController();
    setError(undefined);
    adapter
      .listModels(controller.signal)
      .then(setModels)
      .catch((reason: unknown) => {
        if (controller.signal.aborted) return;
        setError(
          reason instanceof Error
            ? reason.message
            : "The model catalog is unavailable.",
        );
      });
    return () => controller.abort();
  }, [adapter, attempt]);

  useEffect(
    () => () => {
      for (const controller of pullControllers.current.values()) {
        controller.abort();
      }
    },
    [],
  );

  const visibleModels = useMemo(() => {
    const normalizedQuery = query.trim().toLowerCase();
    const filtered = (models ?? []).filter((model) => {
      if (kind !== "all" && model.kind !== kind) return false;
      if (
        platform !== "all" &&
        !model.requirements.platforms.includes(platform)
      ) {
        return false;
      }
      if (
        compatibleOnly &&
        modelCompatibility(model, machine).level === "incompatible"
      ) {
        return false;
      }
      if (!normalizedQuery) return true;
      return [
        model.name,
        model.creator,
        model.description,
        model.backend,
        ...model.tags,
      ]
        .join(" ")
        .toLowerCase()
        .includes(normalizedQuery);
    });
    return [...filtered].sort((left, right) => {
      const score = (model: ModelRecord) =>
        (model.installed ? 100 : 0) +
        (modelCompatibility(model, machine).level === "compatible" ? 30 : 0) +
        (model.name.startsWith("minimax-h3") ? 10 : 0);
      return score(right) - score(left) || left.name.localeCompare(right.name);
    });
  }, [compatibleOnly, kind, machine, models, platform, query]);

  const kindCounts = useMemo(() => {
    const counts: Record<"all" | ModelKind, number> = { all: 0, chat: 0, image: 0, video: 0, speech: 0 };
    for (const model of models ?? []) {
      counts.all += 1;
      counts[model.kind] += 1;
    }
    return counts;
  }, [models]);

	const pull = async (model: ModelRecord, acceptLicense = false, accessToken?: string) => {
    const controller = new AbortController();
    pullControllers.current.set(model.id, controller);
    setTransfers((current) => ({
      ...current,
      [model.id]: {
        state: "downloading",
        receivedBytes: 0,
        totalBytes: model.requirements.diskBytes,
      },
    }));
    try {
		const installed = await adapter.pullModel(model.id, {
			signal: controller.signal,
			acceptLicense,
			accessToken,
        onProgress(progress) {
          setTransfers((current) => ({
            ...current,
            [model.id]: { state: "downloading", ...progress },
          }));
        },
      });
      setModels((current) =>
        current?.map((item) => (item.id === installed.id ? installed : item)),
      );
      setSelected((current) =>
        current?.id === installed.id ? installed : current,
      );
      setTransfers((current) => {
        const next = { ...current };
        delete next[model.id];
        return next;
      });
    } catch (reason) {
      if (controller.signal.aborted) return;
      setTransfers((current) => ({
        ...current,
        [model.id]: {
          ...(current[model.id] ?? {
            receivedBytes: 0,
            totalBytes: model.requirements.diskBytes,
          }),
          state: "error",
          message:
            reason instanceof Error ? reason.message : "Download failed.",
        },
      }));
    } finally {
      pullControllers.current.delete(model.id);
    }
  };

  const cancelPull = async (model: ModelRecord) => {
    setTransfers((current) => ({
      ...current,
      [model.id]: { ...current[model.id], state: "cancelling" },
    }));
    pullControllers.current.get(model.id)?.abort();
    try {
      await adapter.cancelPull(model.id);
      setTransfers((current) => {
        const next = { ...current };
        delete next[model.id];
        return next;
      });
    } catch (reason) {
      setTransfers((current) => ({
        ...current,
        [model.id]: {
          ...current[model.id],
          state: "error",
          message:
            reason instanceof Error ? reason.message : "Could not cancel.",
        },
      }));
    }
  };

  const remove = async () => {
    if (!removeTarget) return;
    const target = removeTarget;
    setError(undefined);
    try {
      await adapter.removeModel(target.id);
      setModels((current) =>
        current?.map((model) =>
          model.id === target.id
            ? { ...model, installed: false, installedBytes: undefined }
            : model,
        ),
      );
      setSelected((current) =>
        current?.id === target.id
          ? { ...current, installed: false, installedBytes: undefined }
          : current,
      );
      setRemoveTarget(undefined);
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : "Could not remove the model.");
    }
  };

  return (
    <section className="models-feature" aria-labelledby="models-title">
      <header className="models-header">
        <div>
          <p className="models-kicker">Model Hub</p>
          <h1 id="models-title">
            Find your next <span>local model.</span>
          </h1>
          <p>
            Compatibility is calculated from this machine’s platform, memory,
            storage, and accelerator—not from marketing labels.
          </p>
        </div>
        <div className="models-machine" aria-label="Current machine profile">
          <span>{machine.platform}</span>
          <strong>{formatModelBytes(machine.memoryBytes)} memory</strong>
          <span>{machine.accelerators.join(" + ")}</span>
        </div>
      </header>

      <div className="models-toolbar">
        <label className="models-search">
          <span className="sr-only">Search models</span>
          <span aria-hidden="true">⌕</span>
          <input
            type="search"
            value={query}
            onChange={(event) => setQuery(event.target.value)}
            placeholder="Search name, creator, backend, or tag"
          />
          {query ? (
            <button
              type="button"
              aria-label="Clear search"
              onClick={() => setQuery("")}
            >
              ×
            </button>
          ) : null}
        </label>
        <label>
          <span className="sr-only">Model type</span>
          <select
            value={kind}
            onChange={(event) =>
              setKind(event.target.value as "all" | ModelKind)
            }
          >
            {kinds.map((item) => (
              <option key={item.value} value={item.value}>
                {item.label}
              </option>
            ))}
          </select>
        </label>
        <label>
          <span className="sr-only">Platform</span>
          <select
            value={platform}
            onChange={(event) =>
              setPlatform(event.target.value as "all" | ModelPlatform)
            }
          >
            <option value="all">All platforms</option>
            <option value="macos">macOS</option>
            <option value="windows">Windows</option>
            <option value="linux">Linux</option>
          </select>
        </label>
        <label className="models-check">
          <input
            type="checkbox"
            checked={compatibleOnly}
            onChange={(event) => setCompatibleOnly(event.target.checked)}
          />
          <span>Fits this machine</span>
        </label>
      </div>
      <div className="models-kind-shortcuts" aria-label="Model categories">
        {kinds.map((item) => (
          <button
            type="button"
            key={item.value}
            className={kind === item.value ? "models-kind-shortcut models-kind-shortcut--active" : "models-kind-shortcut"}
            onClick={() => setKind(item.value)}
          >
            <strong>{kindCounts[item.value]}</strong>
            <span>{item.label}</span>
          </button>
        ))}
        <p>{kind === "video" ? "Every video variant is listed, including MiniMax-H3 when supported by the current platform." : "Choose a category to see every catalog entry."}</p>
      </div>

      {error ? (
        <div className="models-state" role="alert">
          <span aria-hidden="true">!</span>
          <h2>Could not load the catalog</h2>
          <p>{error}</p>
          <button type="button" onClick={() => setAttempt((value) => value + 1)}>
            Try again
          </button>
        </div>
      ) : !models ? (
        <ModelSkeletons />
      ) : visibleModels.length ? (
        <div className="model-grid">
          {visibleModels.map((model) => (
            <ModelCard
              key={model.id}
              machine={machine}
              model={model}
              transfer={transfers[model.id]}
              onCancel={() => void cancelPull(model)}
              onOpen={() => setSelected(model)}
				onPull={() => model.gated ? setSelected(model) : void pull(model)}
            />
          ))}
        </div>
      ) : (
        <div className="models-state">
          <span aria-hidden="true">⌕</span>
          <h2>No models match those filters.</h2>
          <p>Try a broader search or show models for every platform.</p>
          <button
            type="button"
            onClick={() => {
              setQuery("");
              setKind("all");
              setPlatform("all");
              setCompatibleOnly(false);
            }}
          >
            Clear filters
          </button>
        </div>
      )}

      {selected ? (
        <ModelDetail
          machine={machine}
          model={selected}
          transfer={transfers[selected.id]}
          onCancel={() => void cancelPull(selected)}
          onClose={() => setSelected(undefined)}
			onPull={(acceptLicense, accessToken) => void pull(selected, acceptLicense, accessToken)}
          onRemove={() => setRemoveTarget(selected)}
        />
      ) : null}
      {removeTarget ? (
        <ConfirmRemove
          model={removeTarget}
          onCancel={() => setRemoveTarget(undefined)}
          onConfirm={() => void remove()}
        />
      ) : null}
    </section>
  );
}

function ModelCard({
  model,
  machine,
  transfer,
  onOpen,
  onPull,
  onCancel,
}: {
  model: ModelRecord;
  machine: MachineProfile;
  transfer?: TransferState;
  onOpen(): void;
  onPull(): void;
  onCancel(): void;
}) {
  const compatibility = modelCompatibility(model, machine);
  const percent = transfer
    ? Math.min(100, Math.round((transfer.receivedBytes / transfer.totalBytes) * 100))
    : 0;
  return (
    <article className="model-card">
      <button
        className="model-card__body"
        type="button"
        onClick={onOpen}
        aria-label={`View ${model.name} details`}
      >
        <div className="model-card__topline">
          <span className={`model-kind model-kind--${model.kind}`}>
            {model.kind}
          </span>
          {model.installed ? (
            <span className="model-installed">✓ Installed</span>
          ) : null}
        </div>
        <h2>{model.name}{model.name.startsWith("minimax-h3") ? <span className="model-featured">H3</span> : null}</h2>
        <p className="model-creator">by {model.creator}</p>
        <p className="model-description">{model.description}</p>
        <div className="model-badges">
          <span
            className={`compatibility compatibility--${compatibility.level}`}
            title={compatibility.reasons.join(". ")}
          >
            {compatibility.level === "compatible"
              ? "Good fit"
              : compatibility.level === "tight"
                ? "Limited headroom"
                : "Not compatible"}
          </span>
          <span>{formatModelBytes(model.requirements.memoryBytes)} RAM</span>
          <span>{formatModelBytes(model.requirements.diskBytes)}</span>
        </div>
        <div className="model-platforms">
          {model.requirements.platforms.map((item) => (
            <span key={item}>{item}</span>
          ))}
          <span>{model.requirements.accelerators.join(" / ")}</span>
        </div>
      </button>
      <div className="model-card__action">
        {transfer ? (
          <div className="model-transfer">
            <div>
              <span>
                {transfer.state === "error"
                  ? transfer.message
                  : transfer.state === "cancelling"
                    ? "Cancelling…"
                    : `Downloading ${percent}%`}
              </span>
              {transfer.state !== "error" ? (
                <button type="button" onClick={onCancel}>
                  Cancel
                </button>
              ) : (
                <button type="button" onClick={onPull}>
                  Retry
                </button>
              )}
            </div>
            <div
              className="model-transfer__track"
              role="progressbar"
              aria-label={`${model.name} download`}
              aria-valuemin={0}
              aria-valuemax={100}
              aria-valuenow={percent}
            >
              <span style={{ width: `${percent}%` }} />
            </div>
          </div>
        ) : model.installed ? (
          <button className="model-secondary" type="button" onClick={onOpen}>
            Manage
          </button>
        ) : (
          <button
            className="model-primary"
            type="button"
            onClick={onPull}
            disabled={compatibility.level === "incompatible"}
            title={
              compatibility.level === "incompatible"
                ? compatibility.reasons.join(". ")
                : undefined
            }
          >
            Pull model
          </button>
        )}
      </div>
    </article>
  );
}

function ModelDetail({
  model,
  machine,
  transfer,
  onClose,
  onPull,
  onCancel,
  onRemove,
}: {
  model: ModelRecord;
  machine: MachineProfile;
  transfer?: TransferState;
  onClose(): void;
  onPull(acceptLicense: boolean, accessToken?: string): void;
  onCancel(): void;
  onRemove(): void;
}) {
  const compatibility = modelCompatibility(model, machine);
  const estimate = estimatedDiskAfterInstall(model, machine);
	const [licenseAccepted, setLicenseAccepted] = useState(false);
	const [accessToken, setAccessToken] = useState("");
  useDialogEscape(onClose);
  return (
    <div className="model-modal-backdrop" role="presentation" onMouseDown={onClose}>
      <section
        className="model-detail"
        role="dialog"
        aria-modal="true"
        aria-labelledby="model-detail-title"
        onMouseDown={(event) => event.stopPropagation()}
      >
        <button
          className="model-modal-close"
          type="button"
          onClick={onClose}
          aria-label="Close model details"
          autoFocus
        >
          ×
        </button>
        <p className="models-kicker">{model.kind} model</p>
        <h2 id="model-detail-title">{model.name}</h2>
        <p className="model-detail__creator">by {model.creator}</p>
        <p className="model-detail__description">{model.description}</p>
        <div className={`model-fit model-fit--${compatibility.level}`}>
          <strong>
            {compatibility.level === "compatible"
              ? "A good fit for this machine"
              : compatibility.level === "tight"
                ? "Expected to run with limited headroom"
                : "This model does not fit this machine"}
          </strong>
          <span>{compatibility.reasons.join(" · ")}</span>
        </div>
        <dl className="model-specs">
          <div>
            <dt>Backend</dt>
            <dd>{model.backend}</dd>
          </div>
          <div>
            <dt>Memory</dt>
            <dd>{formatModelBytes(model.requirements.memoryBytes)} minimum</dd>
          </div>
          <div>
            <dt>Download</dt>
            <dd>{estimate.required}</dd>
          </div>
          <div>
            <dt>Disk after pull</dt>
            <dd>{estimate.remaining} free</dd>
          </div>
          <div>
            <dt>Platforms</dt>
            <dd>{model.requirements.platforms.join(", ")}</dd>
          </div>
          <div>
            <dt>Acceleration</dt>
            <dd>{model.requirements.accelerators.join(", ")}</dd>
          </div>
        </dl>
		<div className="model-tags">
          {model.tags.map((tag) => (
            <span key={tag}>{tag}</span>
          ))}
		</div>
		{model.gated ? (
			<div className="model-license">
				<strong>{model.license ?? "Gated model license"}</strong>
				<p>
					First accept the provider terms at {model.licenseUrl}. Paste a Hugging Face read
					 token for this download. Tapioca passes it only to the local downloader and does not save it.
				</p>
				<label className="model-license__token">
					<span>Hugging Face read token</span>
					<input
						type="password"
						value={accessToken}
						onChange={(event) => setAccessToken(event.target.value)}
						placeholder="hf_…"
						autoComplete="off"
					/>
				</label>
				<label>
					<input
						type="checkbox"
						checked={licenseAccepted}
						onChange={(event) => setLicenseAccepted(event.target.checked)}
					/>
					<span>I reviewed and accept these model terms.</span>
				</label>
			</div>
		) : null}
        <div className="model-detail__actions">
          {transfer ? (
            <button className="model-secondary" type="button" onClick={onCancel}>
              Cancel download
            </button>
          ) : model.installed ? (
            <>
              <span className="model-installed">✓ Installed locally</span>
              <button className="model-danger" type="button" onClick={onRemove}>
                Remove model
              </button>
            </>
          ) : (
            <button
              className="model-primary"
              type="button"
				onClick={() => onPull(licenseAccepted, accessToken)}
				disabled={compatibility.level === "incompatible" || (model.gated && (!licenseAccepted || !accessToken.trim()))}
            >
              Pull {estimate.required}
            </button>
          )}
        </div>
      </section>
    </div>
  );
}

function ConfirmRemove({
  model,
  onCancel,
  onConfirm,
}: {
  model: ModelRecord;
  onCancel(): void;
  onConfirm(): void;
}) {
  useDialogEscape(onCancel);
  return (
    <div className="model-modal-backdrop model-modal-backdrop--confirm">
      <section
        className="model-confirm"
        role="alertdialog"
        aria-modal="true"
        aria-labelledby="remove-title"
        aria-describedby="remove-description"
      >
        <h2 id="remove-title">Remove {model.name}?</h2>
        <p id="remove-description">
          This deletes the local model files. You can pull the model again
          later.
        </p>
        <div>
          <button type="button" onClick={onCancel} autoFocus>
            Keep model
          </button>
          <button className="model-danger" type="button" onClick={onConfirm}>
            Remove local files
          </button>
        </div>
      </section>
    </div>
  );
}

function ModelSkeletons() {
  return (
    <div className="model-grid" aria-busy="true" aria-label="Loading models">
      {[0, 1, 2, 3, 4, 5].map((item) => (
        <div className="model-skeleton" key={item} />
      ))}
    </div>
  );
}

function useDialogEscape(onClose: () => void) {
  useEffect(() => {
    const handleKey = (event: globalThis.KeyboardEvent) => {
      if (event.key === "Escape") onClose();
    };
    window.addEventListener("keydown", handleKey);
    return () => window.removeEventListener("keydown", handleKey);
  }, [onClose]);
}

export function activateCardFromKeyboard(
  event: KeyboardEvent<HTMLElement>,
  activate: () => void,
) {
  if (event.key === "Enter" || event.key === " ") {
    event.preventDefault();
    activate();
  }
}
