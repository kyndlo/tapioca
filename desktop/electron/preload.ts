import { contextBridge, ipcRenderer } from "electron";
import { createTapiocaApi } from "./preload-api";

const api = createTapiocaApi({
  invoke: (channel, payload) => ipcRenderer.invoke(channel, payload),
  on: (channel, listener) => ipcRenderer.on(channel, listener),
  removeListener: (channel, listener) =>
    ipcRenderer.removeListener(channel, listener),
});

contextBridge.exposeInMainWorld("tapioca", api);
