import { existsSync, readFileSync } from "node:fs";
import path from "node:path";
import { describe, expect, it } from "vitest";
import {
  BoundedNdjsonParser,
  controlEventSchema,
  controlRequestSchema,
  controlResponseSchema,
  NdjsonProtocolError,
  SIDECAR_PROTOCOL_VERSION,
} from "./sidecar";

describe("canonical v1 control schemas", () => {
  it("accepts the canonical request, response, and event shapes", () => {
    expect(
      controlRequestSchema.parse({
        version: 1,
        type: "request",
        id: "1",
        method: "catalog.list",
        params: { kind: "image" },
      }),
    ).toMatchObject({ version: SIDECAR_PROTOCOL_VERSION });
    expect(
      controlResponseSchema.parse({
        version: 1,
        type: "response",
        id: "1",
        result: { models: [] },
      }),
    ).toHaveProperty("result");
    const goldenEvent = JSON.parse(
      readFileSync(
        path.resolve(process.cwd(), "../contracts/control/v1/events.ndjson"),
        "utf8",
      ).split("\n")[0],
    );
    expect(controlEventSchema.parse(goldenEvent)).toHaveProperty(
      "event",
      "job.started",
    );
  });

  it("requires exactly one response outcome", () => {
    expect(() =>
      controlResponseSchema.parse({
        version: 1,
        type: "response",
        id: "1",
      }),
    ).toThrow();
    expect(() =>
      controlResponseSchema.parse({
        version: 1,
        type: "response",
        id: "1",
        result: {},
        error: { code: "bad", message: "bad", retryable: false },
      }),
    ).toThrow();
  });

  it("validates shared fixtures automatically when they become available", () => {
    const fixtureRoot = path.resolve(
      process.cwd(),
      "../contracts/control/v1",
    );
    if (!existsSync(fixtureRoot)) return;
    for (const name of [
      "requests.ndjson",
      "responses.ndjson",
      "events.ndjson",
    ]) {
      const file = path.join(fixtureRoot, name);
      if (!existsSync(file)) continue;
      const values = readFileSync(file, "utf8")
        .trim()
        .split("\n")
        .map((line) => JSON.parse(line));
      for (const value of values) {
        if (name === "requests.ndjson") controlRequestSchema.parse(value);
        else if (name === "responses.ndjson") controlResponseSchema.parse(value);
        else controlEventSchema.parse(value);
      }
    }
  });
});

describe("bounded NDJSON parsing", () => {
  const response = JSON.stringify({
    version: 1,
    type: "response",
    id: "1",
    result: { ok: true },
  });
  const event = readFileSync(
    path.resolve(process.cwd(), "../contracts/control/v1/events.ndjson"),
    "utf8",
  ).split("\n")[0];

  it("handles fragmented and multiple lines", () => {
    const parser = new BoundedNdjsonParser();
    expect(parser.push(response.slice(0, 12))).toEqual([]);
    expect(
      parser.push(`${response.slice(12)}\n${event}\n`),
    ).toMatchObject([
      { type: "response", id: "1" },
      { type: "event", event: "job.started" },
    ]);
  });

  it("rejects malformed and invalid output", () => {
    expect(() => new BoundedNdjsonParser().push("{oops}\n")).toThrow(
      NdjsonProtocolError,
    );
    expect(() =>
      new BoundedNdjsonParser().push('{"type":"mystery"}\n'),
    ).toThrow(NdjsonProtocolError);
  });

  it("enforces the 4 MiB line bound before a newline", () => {
    const parser = new BoundedNdjsonParser();
    expect(() => parser.push(Buffer.alloc(4 * 1024 * 1024 + 1, 0x61))).toThrow(
      expect.objectContaining({ code: "line_too_large" }),
    );
  });
});
