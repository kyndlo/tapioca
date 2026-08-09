import { describe, expect, it } from "vitest";
import { encodePcm16Wav } from "./audio";

describe("encodePcm16Wav", () => {
  it("creates a mono PCM WAV with a valid header", () => {
    const bytes = encodePcm16Wav(
      [new Float32Array([-1, -0.5, 0, 0.5, 1])],
      16_000,
    );
    const view = new DataView(bytes.buffer);
    const text = (start: number, length: number) =>
      String.fromCharCode(...bytes.slice(start, start + length));

    expect(text(0, 4)).toBe("RIFF");
    expect(text(8, 4)).toBe("WAVE");
    expect(text(36, 4)).toBe("data");
    expect(view.getUint16(22, true)).toBe(1);
    expect(view.getUint32(24, true)).toBe(16_000);
    expect(view.getUint32(40, true)).toBe(10);
    expect(bytes).toHaveLength(54);
  });

  it("rejects empty recordings", () => {
    expect(() => encodePcm16Wav([], 16_000)).toThrow(/did not contain audio/i);
  });
});
