#!/usr/bin/env node
// Builds the macOS `multica` CLI from server/cmd/multica and copies the
// binary into apps/desktop/resources/bin/ so electron-vite (dev) and
// electron-builder (prod) pick it up. Running this on every dev/build/package
// invocation guarantees the bundled CLI always matches the current Go
// source — no more stale binary surprises. Go's build cache makes the
// no-op case (nothing changed) effectively free.
//
// ldflags mirror `make build` so `multica --version` reports a meaningful
// version / commit / date. `--target-arch x64|arm64` picks the CPU
// architecture; it defaults to the host's.
//
// Graceful: if `go` is not installed, we skip the build and the app falls
// back to `multica` on PATH at runtime. A genuine Go compile error is
// fatal — you want that to block dev, not hide.

import { access, chmod, copyFile, mkdir, rm } from "node:fs/promises";
import { constants } from "node:fs";
import { execFileSync, execSync } from "node:child_process";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { deriveVersion } from "./package.mjs";

const here = dirname(fileURLToPath(import.meta.url));
const repoRoot = resolve(here, "..", "..", "..");
const serverDir = join(repoRoot, "server");

const GOOS = "darwin";

const SUPPORTED_ARCHS = new Set(["x64", "arm64"]);

function runtimeArchFromArgs(argv) {
  const flagIndex = argv.indexOf("--target-arch");
  if (flagIndex === -1) return process.arch;
  return argv[flagIndex + 1] ?? "";
}

function normalizeRuntimeArch(arch) {
  if (SUPPORTED_ARCHS.has(arch)) return arch;
  throw new Error(
    `[bundle-cli] unsupported target architecture: ${arch}. ` +
      "Use x64 or arm64.",
  );
}

const targetArch = normalizeRuntimeArch(runtimeArchFromArgs(process.argv.slice(2)));
const goarch = targetArch === "x64" ? "amd64" : targetArch;
const binName = "multica";
const srcBinary = join(serverDir, "bin", `${GOOS}-${goarch}`, binName);
const destDir = join(repoRoot, "apps", "desktop", "resources", "bin");
const destBinary = join(destDir, binName);

// Hand git arguments straight to the binary (no shell).
function git(...args) {
  try {
    return execFileSync("git", args, { encoding: "utf-8" }).trim();
  } catch {
    return "";
  }
}

function hasGo() {
  try {
    execSync("go version", { stdio: "pipe" });
    return true;
  } catch {
    return false;
  }
}

async function exists(p) {
  try {
    await access(p, constants.F_OK);
    return true;
  } catch {
    return false;
  }
}

if (hasGo()) {
  const version = deriveVersion() || "dev";
  const commit = git("rev-parse", "--short", "HEAD") || "unknown";
  const date = new Date().toISOString().replace(/\.\d+Z$/, "Z");
  const ldflags = `-X main.version=${version} -X main.commit=${commit} -X main.date=${date}`;

  console.log(
    `[bundle-cli] go build → ${srcBinary} (${GOOS}/${goarch}, version=${version} commit=${commit})`,
  );
  await mkdir(join(serverDir, "bin", `${GOOS}-${goarch}`), { recursive: true });
  execFileSync(
    "go",
    [
      "build",
      "-ldflags",
      ldflags,
      "-o",
      srcBinary,
      "./cmd/multica",
    ],
    {
      cwd: serverDir,
      stdio: "inherit",
      env: {
        ...process.env,
        CGO_ENABLED: "0",
        GOOS,
        GOARCH: goarch,
      },
    },
  );
} else {
  console.warn(
    "[bundle-cli] `go` not found in PATH — skipping CLI build. " +
      "Desktop will use whatever is already in resources/bin/, or fall back " +
      "to `multica` on PATH at runtime.",
  );
}

if (!(await exists(srcBinary))) {
  console.warn(
    `[bundle-cli] ${srcBinary} not present — Desktop will fall back to ` +
      "`multica` on PATH at runtime.",
  );
  await rm(destDir, { recursive: true, force: true });
  process.exit(0);
}

await rm(destDir, { recursive: true, force: true });
await mkdir(destDir, { recursive: true });
await copyFile(srcBinary, destBinary);
await chmod(destBinary, 0o755);

// Ad-hoc sign so Gatekeeper doesn't complain when the parent app (which is
// itself unsigned) spawns the child.
try {
  execSync(`codesign -s - --force ${JSON.stringify(destBinary)}`, {
    stdio: "pipe",
  });
} catch {
  // Non-fatal. Unsigned binaries still run when the parent app is trusted.
}

console.log(`[bundle-cli] bundled ${srcBinary} → ${destBinary}`);
