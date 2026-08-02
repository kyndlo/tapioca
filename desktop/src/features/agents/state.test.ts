import { describe, expect, it } from "vitest";
import {
  applyAgentEvent,
  canLaunchAgent,
  initialAgentRunView,
  maskEnvironment,
} from "./state";

describe("agent cockpit state", () => {
  it("requires every fixed launch prerequisite", () => {
    const ready = {
      installed: true,
      modelReady: true,
      serverReady: true,
      hasWorkspace: true,
      busy: false,
    };
    expect(canLaunchAgent(ready)).toBe(true);
    for (const key of Object.keys(ready) as Array<keyof typeof ready>) {
      if (key === "busy") {
        expect(canLaunchAgent({ ...ready, busy: true })).toBe(false);
      } else {
        expect(canLaunchAgent({ ...ready, [key]: false })).toBe(false);
      }
    }
  });

  it("masks sensitive environment values without changing names", () => {
    expect(
      maskEnvironment([
        { name: "BASE_URL", value: "http://127.0.0.1:11435" },
        { name: "TOKEN", value: "secret", sensitive: true },
      ]),
    ).toEqual([
      { name: "BASE_URL", value: "http://127.0.0.1:11435" },
      { name: "TOKEN", value: "••••••••", sensitive: true },
    ]);
  });

  it("updates status, bounds logs, and maps exit state", () => {
    let state = applyAgentEvent(initialAgentRunView(), {
      type: "status",
      status: "running",
      message: "Ready",
    });
    for (let index = 0; index < 505; index += 1) {
      state = applyAgentEvent(state, {
        type: "log",
        level: "info",
        message: `line ${index}`,
      });
    }
    expect(state.status).toBe("running");
    expect(state.logs).toHaveLength(500);
    state = applyAgentEvent(state, { type: "exit", code: 0 });
    expect(state).toMatchObject({ status: "completed" });
  });
});
