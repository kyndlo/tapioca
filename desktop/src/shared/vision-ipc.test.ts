import {expect, it} from "vitest";
import {chatRequestInputSchema} from "./ipc";

it("accepts inline still images and rejects unsupported image sources and modalities", () => {
  const payload = (content: unknown) => ({jobId: "chat", model: "minicpm", messages: [{role: "user", content}]});
  expect(chatRequestInputSchema.safeParse(payload([{type: "text", text: "Describe"}, {type: "image_url", image_url: {url: "data:image/png;base64,AA=="}}])).success).toBe(true);
  for (const part of [
    {type: "image_url", image_url: {url: "https://example.com/a.png"}},
    {type: "image_url", image_url: {url: "file:///tmp/a.png"}},
    {type: "image_url", image_url: {url: "data:image/svg+xml;base64,AA=="}},
    {type: "input_audio", input_audio: {data: "AA=="}},
  ]) expect(chatRequestInputSchema.safeParse(payload([part])).success).toBe(false);
});
