import { access, copyFile, mkdir } from "node:fs/promises";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const websiteRoot = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const repositoryRoot = resolve(websiteRoot, "..");
const publicRoot = resolve(websiteRoot, "public");

await mkdir(publicRoot, { recursive: true });
await Promise.all(["install.sh", "install.ps1"].map(async (name) => {
  const source = resolve(repositoryRoot, "scripts", name);
  const target = resolve(publicRoot, name);
  try {
    await copyFile(source, target);
  } catch (error) {
    if (error.code !== "ENOENT") throw error;
    // Sites builds the website from its own source repository, without the
    // monorepo parent. In that checkout the verified public copy is committed.
    await access(target);
  }
}));
