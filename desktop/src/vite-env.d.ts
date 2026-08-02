/// <reference types="vite/client" />

import type { TapiocaDesktopApi } from "./shared/ipc";

declare global {
  interface Window {
    tapioca: TapiocaDesktopApi;
  }
}

export {};
