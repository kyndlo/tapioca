import { copyFile, mkdir } from "node:fs/promises";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const websiteRoot = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const repositoryRoot = resolve(websiteRoot, "..");
const publicRoot = resolve(websiteRoot, "public");

await mkdir(publicRoot, { recursive: true });
await Promise.all([
  copyFile(resolve(repositoryRoot, "scripts/install.sh"), resolve(publicRoot, "install.sh")),
  copyFile(resolve(repositoryRoot, "scripts/install.ps1"), resolve(publicRoot, "install.ps1")),
]);
