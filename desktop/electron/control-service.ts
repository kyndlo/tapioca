import type { CatalogFilter } from "../src/shared/ipc";
import {
  adaptCatalogResult,
  adaptHealthResult,
  adaptInstalledResult,
} from "./control-adapters";
import type { ControlClient } from "./control-client";

type RequestClient = Pick<ControlClient, "request">;

export class ControlService {
  constructor(private readonly client: RequestClient) {}

  async health() {
    return adaptHealthResult(await this.client.request("health.get"));
  }

  async catalog(filter: CatalogFilter) {
    const rawCatalog = await this.client.request("catalog.list");
    return adaptCatalogResult(rawCatalog, filter);
  }

  async installed() {
    return adaptInstalledResult(await this.client.request("installed.list"));
  }
}
