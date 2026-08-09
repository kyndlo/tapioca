import { useEffect, useRef, useState } from "react";
import { decodeRecordingToWav } from "./audio";
import type { CreatorAdapter, LocalFileSelection } from "./types";

type RecorderState = "idle" | "requesting" | "recording" | "processing";

export function VoiceRecorder({
  adapter,
  disabled,
  selection,
  onRecorded,
  onError,
}: {
  adapter: CreatorAdapter;
  disabled: boolean;
  selection?: LocalFileSelection;
  onRecorded(selection: LocalFileSelection): void;
  onError(message: string): void;
}) {
  const [consent, setConsent] = useState(false);
  const [state, setState] = useState<RecorderState>("idle");
  const [seconds, setSeconds] = useState(0);
  const [level, setLevel] = useState(0);
  const recorder = useRef<MediaRecorder | undefined>(undefined);
  const stream = useRef<MediaStream | undefined>(undefined);
  const audioContext = useRef<AudioContext | undefined>(undefined);
  const meterFrame = useRef<number | undefined>(undefined);
  const chunks = useRef<Blob[]>([]);
  const startedAt = useRef(0);
  const disposed = useRef(false);

  useEffect(() => {
    if (state !== "recording") return;
    const timer = setInterval(
      () => setSeconds((Date.now() - startedAt.current) / 1_000),
      200,
    );
    return () => clearInterval(timer);
  }, [state]);

  useEffect(
    () => () => {
      disposed.current = true;
      chunks.current = [];
      if (recorder.current?.state === "recording") recorder.current.stop();
      stream.current?.getTracks().forEach((track) => track.stop());
      stopAudioMeter();
    },
    [],
  );

  const start = async () => {
    if (!consent) return;
    if (!navigator.mediaDevices?.getUserMedia || typeof MediaRecorder === "undefined") {
      onError("Microphone recording is unavailable in this desktop environment.");
      return;
    }
    setState("requesting");
    setSeconds(0);
    try {
      const nextStream = await navigator.mediaDevices.getUserMedia({
        audio: {
          channelCount: 1,
          echoCancellation: true,
          noiseSuppression: true,
          autoGainControl: true,
        },
        video: false,
      });
      const mimeType = ["audio/webm;codecs=opus", "audio/webm", "audio/mp4"]
        .find((candidate) => MediaRecorder.isTypeSupported(candidate));
      const nextRecorder = new MediaRecorder(
        nextStream,
        mimeType ? { mimeType } : undefined,
      );
      chunks.current = [];
      stream.current = nextStream;
      recorder.current = nextRecorder;
      startAudioMeter(nextStream);
      nextRecorder.addEventListener("dataavailable", (event) => {
        if (event.data.size) chunks.current.push(event.data);
      });
      nextRecorder.addEventListener("error", () => {
        onError("The microphone stopped unexpectedly. Please record again.");
        setState("idle");
      });
      nextRecorder.addEventListener("stop", () => void finish(nextRecorder.mimeType));
      startedAt.current = Date.now();
      nextRecorder.start(250);
      setState("recording");
    } catch (cause) {
      setState("idle");
      onError(
        cause instanceof DOMException && cause.name === "NotAllowedError"
          ? "Microphone access was denied. Allow Tapioca in system privacy settings and try again."
          : cause instanceof Error
            ? cause.message
            : String(cause),
      );
    }
  };

  const finish = async (mimeType: string) => {
    if (disposed.current) return;
    setState("processing");
    stream.current?.getTracks().forEach((track) => track.stop());
    stopAudioMeter();
    try {
      const wav = await decodeRecordingToWav(new Blob(chunks.current, { type: mimeType }));
      if (wav.durationSeconds < 2) {
        throw new Error("Record at least two seconds so the model can learn the voice.");
      }
      onRecorded(await adapter.saveVoiceRecording(wav.bytes, wav.durationSeconds));
    } catch (cause) {
      onError(cause instanceof Error ? cause.message : String(cause));
    } finally {
      if (disposed.current) return;
      chunks.current = [];
      recorder.current = undefined;
      stream.current = undefined;
      setState("idle");
    }
  };

  const stop = () => {
    if (recorder.current?.state === "recording") recorder.current.stop();
  };

  function startAudioMeter(nextStream: MediaStream) {
    const context = new AudioContext();
    const analyser = context.createAnalyser();
    analyser.fftSize = 256;
    const samples = new Uint8Array(analyser.fftSize);
    context.createMediaStreamSource(nextStream).connect(analyser);
    audioContext.current = context;
    const update = () => {
      analyser.getByteTimeDomainData(samples);
      let energy = 0;
      for (const sample of samples) {
        const normalized = (sample - 128) / 128;
        energy += normalized * normalized;
      }
      if (!disposed.current) {
        setLevel(Math.min(1, Math.sqrt(energy / samples.length) * 4));
        meterFrame.current = requestAnimationFrame(update);
      }
    };
    void context.resume();
    update();
  }

  function stopAudioMeter() {
    if (meterFrame.current !== undefined) cancelAnimationFrame(meterFrame.current);
    meterFrame.current = undefined;
    if (audioContext.current) void audioContext.current.close();
    audioContext.current = undefined;
    if (!disposed.current) setLevel(0);
  }

  return (
    <section className="voice-recorder" aria-labelledby="voice-recorder-title">
      <div className="voice-recorder__heading">
        <div>
          <span className="voice-recorder__icon" aria-hidden="true">●</span>
          <div>
            <strong id="voice-recorder-title">Record a voice reference</strong>
            <small>10–30 seconds of clear speech gives the best clone.</small>
          </div>
        </div>
        {state === "recording" ? <time>{formatDuration(seconds)}</time> : null}
      </div>
      <div className={`voice-recorder__meter voice-recorder__meter--${state}`} aria-hidden="true">
        {Array.from({ length: 18 }, (_, index) => {
          const shape = 0.35 + Math.sin(((index + 1) / 19) * Math.PI) * 0.65;
          return <span key={index} style={{ height: `${8 + level * shape * 28}px` }} />;
        })}
      </div>
      <p className="voice-recorder__level" aria-live="polite">
        {state === "recording"
          ? level < 0.08
            ? "Speak a little louder or move closer to the microphone."
            : level > 0.82
              ? "The input is very loud—move slightly farther away."
              : "Good recording level. Keep speaking naturally."
          : "Choose a quiet room and speak naturally for 10–30 seconds."}
      </p>
      {selection?.previewUrl ? (
        <audio className="voice-recorder__preview" src={selection.previewUrl} controls preload="metadata" />
      ) : null}
      <label className="voice-recorder__consent">
        <input
          type="checkbox"
          checked={consent}
          onChange={(event) => setConsent(event.target.checked)}
          disabled={disabled || state !== "idle"}
        />
        <span>I have permission to clone this voice.</span>
      </label>
      <div className="voice-recorder__actions">
        {state === "recording" ? (
          <button type="button" className="voice-recorder__stop" onClick={stop}>Stop and use recording</button>
        ) : (
          <button
            type="button"
            onClick={() => void start()}
            disabled={disabled || !consent || state !== "idle"}
          >
            {state === "requesting"
              ? "Waiting for microphone…"
              : state === "processing"
                ? "Preparing WAV…"
                : selection
                  ? "Record again"
                  : "Start recording"}
          </button>
        )}
        <span>Recorded locally and saved as a private WAV reference.</span>
      </div>
    </section>
  );
}

function formatDuration(seconds: number): string {
  const whole = Math.max(0, Math.floor(seconds));
  return `${Math.floor(whole / 60)}:${String(whole % 60).padStart(2, "0")}`;
}
