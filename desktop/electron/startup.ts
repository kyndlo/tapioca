import type { ControlClient } from "./control-client";

type StartupClient = Pick<ControlClient, "start" | "state">;

export interface StartupResult {
  ready: boolean;
  error?: Error;
}

export async function settleControlBeforeWindow(
  client: StartupClient,
  executable: string,
  createWindow: () => void,
): Promise<StartupResult> {
  try {
    await client.start(executable);
    createWindow();
    return { ready: true };
  } catch (error) {
    const normalized =
      error instanceof Error ? error : new Error(String(error));
    createWindow();
    return { ready: false, error: normalized };
  }
}
