export function encodePcm16Wav(
  channels: readonly Float32Array[],
  sampleRate: number,
): Uint8Array {
  if (!channels.length || !channels[0]?.length || sampleRate <= 0) {
    throw new Error("The microphone recording did not contain audio.");
  }
  const frameCount = Math.min(...channels.map((channel) => channel.length));
  const bytes = new Uint8Array(44 + frameCount * 2);
  const view = new DataView(bytes.buffer);
  const ascii = (offset: number, value: string) => {
    for (let index = 0; index < value.length; index += 1) {
      view.setUint8(offset + index, value.charCodeAt(index));
    }
  };
  ascii(0, "RIFF");
  view.setUint32(4, 36 + frameCount * 2, true);
  ascii(8, "WAVE");
  ascii(12, "fmt ");
  view.setUint32(16, 16, true);
  view.setUint16(20, 1, true);
  view.setUint16(22, 1, true);
  view.setUint32(24, sampleRate, true);
  view.setUint32(28, sampleRate * 2, true);
  view.setUint16(32, 2, true);
  view.setUint16(34, 16, true);
  ascii(36, "data");
  view.setUint32(40, frameCount * 2, true);
  for (let frame = 0; frame < frameCount; frame += 1) {
    let sample = 0;
    for (const channel of channels) sample += channel[frame] ?? 0;
    sample = Math.max(-1, Math.min(1, sample / channels.length));
    view.setInt16(44 + frame * 2, sample < 0 ? sample * 0x8000 : sample * 0x7fff, true);
  }
  return bytes;
}

export async function decodeRecordingToWav(blob: Blob): Promise<{
  bytes: Uint8Array;
  durationSeconds: number;
}> {
  const context = new AudioContext();
  try {
    const decoded = await context.decodeAudioData(await blob.arrayBuffer());
    const channels = Array.from(
      { length: decoded.numberOfChannels },
      (_, index) => decoded.getChannelData(index),
    );
    return {
      bytes: encodePcm16Wav(channels, decoded.sampleRate),
      durationSeconds: decoded.duration,
    };
  } finally {
    await context.close();
  }
}
