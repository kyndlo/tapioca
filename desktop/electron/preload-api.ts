import { IPC_CHANNELS, ipcSchemas, type TapiocaDesktopApi } from "../src/shared/ipc";

export interface PreloadTransport {
  invoke(channel: string, payload?: unknown): Promise<unknown>;
  on(channel: string, listener: (_event: unknown, payload: unknown) => void): void;
  removeListener(channel: string, listener: (_event: unknown, payload: unknown) => void): void;
}

export function createTapiocaApi(transport: PreloadTransport): TapiocaDesktopApi {
  return Object.freeze({
    async health() {
      return ipcSchemas.healthResult.parse(
        await transport.invoke(IPC_CHANNELS.health),
      );
    },
    async systemSnapshot() {
      return ipcSchemas.systemSnapshotResult.parse(
        await transport.invoke(IPC_CHANNELS.systemSnapshot),
      );
    },
    async models() {
      return ipcSchemas.modelsResult.parse(
        await transport.invoke(IPC_CHANNELS.models),
      );
    },
    async catalogRefresh() {
      return ipcSchemas.catalogRefreshResult.parse(
        await transport.invoke(IPC_CHANNELS.catalogRefresh),
      );
    },
    async softwareUpdateCheck() {
      return ipcSchemas.softwareUpdateInfoResult.parse(
        await transport.invoke(IPC_CHANNELS.softwareUpdateCheck),
      );
    },
    async softwareUpdateInstall() {
      return ipcSchemas.softwareUpdateInstallResult.parse(
        await transport.invoke(IPC_CHANNELS.softwareUpdateInstall),
      );
    },
    async modelPull(raw: Parameters<TapiocaDesktopApi["modelPull"]>[0]) {
      const input = ipcSchemas.modelPullInput.parse(raw);
      return ipcSchemas.modelPullResult.parse(
        await transport.invoke(IPC_CHANNELS.modelPull, input),
      );
    },
    async modelRemove(raw: Parameters<TapiocaDesktopApi["modelRemove"]>[0]) {
      const input = ipcSchemas.modelRemoveInput.parse(raw);
      await transport.invoke(IPC_CHANNELS.modelRemove, input);
    },
    async cancelJob(raw: Parameters<TapiocaDesktopApi["cancelJob"]>[0]) {
      const input = ipcSchemas.cancelJobInput.parse(raw);
      return Boolean(await transport.invoke(IPC_CHANNELS.cancelJob, input));
    },
    async chatStatus() {
      return ipcSchemas.chatStatusResult.parse(
        await transport.invoke(IPC_CHANNELS.chatStatus),
      );
    },
    async serverStart(raw: Parameters<TapiocaDesktopApi["serverStart"]>[0]) {
      const input = ipcSchemas.serverStartInput.parse(raw);
      return ipcSchemas.serverMutationResult.parse(await transport.invoke(IPC_CHANNELS.serverStart, input));
    },
    async serverStop(raw: Parameters<TapiocaDesktopApi["serverStop"]>[0]) {
      const input = ipcSchemas.serverStopInput.parse(raw);
      return ipcSchemas.serverMutationResult.parse(await transport.invoke(IPC_CHANNELS.serverStop, input));
    },
    async serverStatus(raw: Parameters<TapiocaDesktopApi["serverStatus"]>[0] = {}) {
      const input = ipcSchemas.serverStatusInput.parse(raw);
      return ipcSchemas.serverStatusResult.parse(await transport.invoke(IPC_CHANNELS.serverStatus, input));
    },
    async chatRequest(raw: Parameters<TapiocaDesktopApi["chatRequest"]>[0]) {
      const input = ipcSchemas.chatRequestInput.parse(raw);
      return ipcSchemas.chatResponseResult.parse(
        await transport.invoke(IPC_CHANNELS.chatRequest, input),
      );
    },
    async selectWorkspace() {
      return ipcSchemas.workspaceResult.parse(
        await transport.invoke(IPC_CHANNELS.selectWorkspace),
      );
    },
    async agentDescribe(raw: Parameters<TapiocaDesktopApi["agentDescribe"]>[0]) {
      const input = ipcSchemas.agentDescribeInput.parse(raw);
      return ipcSchemas.agentDescribeResult.parse(
        await transport.invoke(IPC_CHANNELS.agentDescribe, input),
      );
    },
    async agentLaunch(raw: Parameters<TapiocaDesktopApi["agentLaunch"]>[0]) {
      const input = ipcSchemas.agentLaunchInput.parse(raw);
      return ipcSchemas.agentLaunchResult.parse(
        await transport.invoke(IPC_CHANNELS.agentLaunch, input),
      );
    },
    async creatorCapabilities() {
      return ipcSchemas.creatorCapabilitiesResult.parse(await transport.invoke(IPC_CHANNELS.creatorCapabilities));
    },
    async creatorCatalog() {
      return ipcSchemas.creatorCatalogResult.parse(await transport.invoke(IPC_CHANNELS.creatorCatalog));
    },
    async creatorPickFile(raw: Parameters<TapiocaDesktopApi["creatorPickFile"]>[0]) {
      const input = ipcSchemas.creatorPickFileInput.parse(raw);
      return ipcSchemas.creatorPickFileResult.parse(await transport.invoke(IPC_CHANNELS.creatorPickFile, input));
    },
    async creatorSaveRecording(raw: Parameters<TapiocaDesktopApi["creatorSaveRecording"]>[0]) {
      const input = ipcSchemas.creatorSaveRecordingInput.parse(raw);
      return ipcSchemas.creatorSaveRecordingResult.parse(
        await transport.invoke(IPC_CHANNELS.creatorSaveRecording, input),
      );
    },
    async creatorGenerate(raw: Parameters<TapiocaDesktopApi["creatorGenerate"]>[0]) {
      const input = ipcSchemas.creatorGenerateInput.parse(raw);
      return ipcSchemas.creatorGenerateResult.parse(await transport.invoke(IPC_CHANNELS.creatorGenerate, input));
    },
    async creatorOutputs() {
      return ipcSchemas.creatorOutputsResult.parse(
        await transport.invoke(IPC_CHANNELS.creatorOutputs),
      );
    },
    async creatorLoraList() {
      return ipcSchemas.creatorLoraListResult.parse(await transport.invoke(IPC_CHANNELS.creatorLoraList));
    },
    async creatorLoraInspect(raw: Parameters<TapiocaDesktopApi["creatorLoraInspect"]>[0]) {
      const input = ipcSchemas.creatorLoraInspectInput.parse(raw);
      return ipcSchemas.creatorLoraInspectResult.parse(await transport.invoke(IPC_CHANNELS.creatorLoraInspect, input));
    },
    async creatorReveal(raw: Parameters<TapiocaDesktopApi["creatorReveal"]>[0]) {
      await transport.invoke(IPC_CHANNELS.creatorReveal, ipcSchemas.creatorOutputInput.parse(raw));
    },
    async creatorSaveMetadata(raw: Parameters<TapiocaDesktopApi["creatorSaveMetadata"]>[0]) {
      return Boolean(await transport.invoke(IPC_CHANNELS.creatorSaveMetadata, ipcSchemas.creatorOutputInput.parse(raw)));
    },
    onJobEvent(listener: Parameters<TapiocaDesktopApi["onJobEvent"]>[0]) {
      const wrapped = (_event: unknown, payload: unknown) => {
        listener(ipcSchemas.jobEvent.parse(payload));
      };
      transport.on(IPC_CHANNELS.jobEvent, wrapped);
      return () => transport.removeListener(IPC_CHANNELS.jobEvent, wrapped);
    },
  });
}
