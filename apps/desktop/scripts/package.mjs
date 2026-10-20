#!/usr/bin/env node
// Wrapper around `electron-builder` that packages the macOS app with its
// version in lockstep with the bundled CLI. Both are derived from
// `git describe --tags --match 'v[0-9]*' --always --dirty`; bundle-cli.mjs
// stamps the same value into the CLI binary via the `main.version` ldflag.
//
// Builds the Electron bundles once, then for each requested architecture
// compiles the matching Go CLI into resources/bin/ and invokes
// electron-builder with `-c.extraMetadata.version=<derived>` so the
// override applies at build time without mutating the tracked package.json.
//
// The electron-vite step is important: electron-builder only packages
// whatever is already in out/, so skipping it (or relying on stale
// artifacts from a prior partial build) ships an app with missing
// renderer code and white-screens on launch.
//
// Usage: `pnpm package -- [--mac [targets…]] [--x64] [--arm64] [args…]`.
// The architecture defaults to the host's. Targets after `--mac` and every
// other argument are forwarded to electron-builder unchanged (e.g.
// `--publish never`). Set `CSC_IDENTITY_AUTO_DISCOVERY=false` so
// electron-builder signs ad hoc instead of looking for a Developer ID cert.
//
// The `normalizeGitVersion`, `deriveVersion`, and `DESCRIBE_ARGS` exports let
// tests cover version derivation both as a pure string transform and as the
// real `git describe` invocation against a throwaway repo.

import { execFileSync, spawnSync } from "node:child_process";
import { rmSync } from "node:fs";
import { delimiter, dirname, resolve } from "node:path";
import { fileURLToPath, pathToFileURL } from "node:url";

const here = dirname(fileURLToPath(import.meta.url));
const desktopRoot = resolve(here, "..");
const bundleCliScript = resolve(here, "bundle-cli.mjs");

const MAC_FLAGS = new Set(["--mac", "--macos", "-m"]);

const ARCH_FLAGS = new Map([
  ["--x64", "x64"],
  ["--arm64", "arm64"],
  ["--universal", "universal"],
]);

const SUPPORTED_CLI_ARCHS = new Set(["x64", "arm64"]);

// Run a git subcommand with its arguments handed straight to the binary,
// never through a shell, so a match pattern like `v[0-9]*` reaches git as a
// single literal argument instead of a shell glob or quoted string.
function git(args, cwd) {
  try {
    return execFileSync("git", args, { encoding: "utf-8", cwd }).trim();
  } catch {
    return "";
  }
}

/**
 * Strip the leading `--` that npm/pnpm insert to separate their own
 * flags from the ones meant for the underlying script.  Without this,
 * `pnpm package -- --mac --arm64 --publish never` forwards the bare
 * `--` into electron-builder's argv, which terminates option parsing
 * and turns `--publish never` into ignored positional arguments.
 */
export function stripLeadingSeparator(argv) {
  if (argv.length > 0 && argv[0] === "--") return argv.slice(1);
  return argv;
}

/**
 * Pure transformation from the `git describe --tags --always --dirty`
 * output to the value we feed into electron-builder's extraMetadata.version.
 *
 *   - empty input              → null   (caller should fall back)
 *   - "v0.1.36"                → "0.1.36"
 *   - "v0.1.35-14-gf1415e96"   → "0.1.35-14-gf1415e96"  (semver prerelease)
 *   - "v0.1.35-…-dirty"        → same, dirty suffix preserved
 *   - "f1415e96" (no tag)      → "0.0.0-gf1415e96"       (fallback)
 *   - "2f24057b" (no tag, hash begins with a digit) → "0.0.0-g2f24057b"
 *   - "0123456"  (no tag, all-digit hash w/ leading zero) → "0.0.0-g0123456"
 *
 * Leading `v` is stripped so the result is valid semver for package.json.
 * The fallback matters because a bare commit hash is never valid semver —
 * even one that happens to start with a digit (e.g. "2f24057b") — and
 * electron-builder rejects a package.json version that is not semver.
 * The hash is prefixed with `g` so the pre-release identifier is always
 * alphanumeric; a bare all-digit hash with a leading zero (e.g. "0123456")
 * would otherwise form `0.0.0-0123456`, which is invalid semver.
 */
export function normalizeGitVersion(raw) {
  if (!raw) return null;
  const stripped = raw.replace(/^v/, "");
  // A real version begins with major.minor.patch. The bare commit hash
  // that `git describe --always` falls back to (no reachable tag) does not,
  // so coerce it to a 0.0.0 prerelease rather than passing it through.
  // Prefix the hash with `g` (mirroring `git describe`'s own `g<hash>`
  // shorthand) so a hash like "0123456" yields "0.0.0-g0123456" — a single
  // alphanumeric identifier — instead of the invalid "0.0.0-0123456".
  if (!/^\d+\.\d+\.\d+/.test(stripped)) {
    return `0.0.0-g${stripped}`;
  }
  return stripped;
}

// The exact argv handed to `git describe` for version derivation. Kept as a
// standalone array — never a shell command string — so the `v[0-9]*` match
// pattern reaches git as one literal argument.
export const DESCRIBE_ARGS = [
  "describe",
  "--tags",
  "--match",
  "v[0-9]*",
  "--always",
  "--dirty",
];

// Exported (with an optional cwd) so tests can exercise the real describe
// invocation against a throwaway repo, not just normalizeGitVersion in
// isolation.
export function deriveVersion(cwd) {
  return normalizeGitVersion(git(DESCRIBE_ARGS, cwd));
}

function uniqueOrdered(values) {
  return [...new Set(values)];
}

export function envWithLocalBins(env = process.env, root = desktopRoot) {
  const pathKey =
    Object.keys(env).find((key) => key.toUpperCase() === "PATH") ?? "PATH";
  const existingPath = env[pathKey] ?? "";
  const localBins = uniqueOrdered([
    resolve(root, "node_modules", ".bin"),
    resolve(root, "..", "..", "node_modules", ".bin"),
  ]);
  const mergedPath = uniqueOrdered([
    ...localBins,
    ...String(existingPath)
      .split(delimiter)
      .filter(Boolean),
  ]).join(delimiter);
  return { ...env, [pathKey]: mergedPath };
}

export function parsePackageArgs(argv) {
  const sharedArgs = [];
  const macTargets = [];
  const requestedArchs = [];

  for (let i = 0; i < argv.length; i += 1) {
    const token = argv[i];
    if (MAC_FLAGS.has(token)) {
      while (i + 1 < argv.length && !argv[i + 1].startsWith("-")) {
        macTargets.push(argv[i + 1]);
        i += 1;
      }
      continue;
    }

    const arch = ARCH_FLAGS.get(token);
    if (arch) {
      requestedArchs.push(arch);
      continue;
    }

    sharedArgs.push(token);
  }

  return {
    sharedArgs,
    macTargets,
    requestedArchs: uniqueOrdered(requestedArchs),
  };
}

export function resolveBuildArchs(parsed, hostPlatform = process.platform, hostArch = process.arch) {
  if (hostPlatform !== "darwin") {
    throw new Error(
      `[package] the desktop app packages on macOS only (current: ${hostPlatform})`,
    );
  }

  const archs =
    parsed.requestedArchs.length > 0 ? parsed.requestedArchs : [hostArch];

  const unsupported = archs.filter((value) => !SUPPORTED_CLI_ARCHS.has(value));
  if (unsupported.length > 0) {
    throw new Error(
      `[package] unsupported Desktop CLI architecture(s): ${unsupported.join(", ")}. ` +
        "Use --x64 or --arm64.",
    );
  }

  return archs;
}

export function builderArgsForArch(
  arch,
  parsed,
  version,
  { useScopedOutputDir = false } = {},
) {
  const builderArgs = [];
  if (version) builderArgs.push(`-c.extraMetadata.version=${version}`);
  builderArgs.push("--mac", ...parsed.macTargets);
  builderArgs.push(`--${arch}`);
  builderArgs.push(...parsed.sharedArgs);
  if (useScopedOutputDir) {
    builderArgs.push(`-c.directories.output=dist/mac-${arch}`);
  }
  return builderArgs;
}

function main() {
  const passthrough = stripLeadingSeparator(process.argv.slice(2));
  const parsed = parsePackageArgs(passthrough);
  const archs = resolveBuildArchs(parsed);
  console.log(`[package] architectures → ${archs.join(", ")}`);

  // Step 0: start every build from an empty output directory, so stale
  // artifacts from a prior run never mix with this run's output.
  const distDir = resolve(desktopRoot, "dist");
  rmSync(distDir, { recursive: true, force: true });
  console.log(`[package] cleaned output dir → ${distDir}`);

  // Step 1: build the Electron main/preload/renderer bundles. Without
  // this step electron-builder silently packages whatever is already in
  // out/, which on a fresh checkout (or after a partial build) ships an
  // app that white-screens because the renderer bundle is missing.
  //
  // A direct `node scripts/package.mjs` run gets no package-local binaries
  // on PATH from pnpm, so envWithLocalBins adds them.
  const viteResult = spawnSync("electron-vite", ["build"], {
    stdio: "inherit",
    cwd: desktopRoot,
    env: envWithLocalBins(),
  });
  if (viteResult.error) {
    console.error(
      "[package] failed to spawn electron-vite:",
      viteResult.error.message,
    );
    process.exit(1);
  }
  if (viteResult.status !== 0) {
    process.exit(viteResult.status ?? 1);
  }

  // Step 2: derive the version that should be written into the app.
  const version = deriveVersion();
  if (version) {
    console.log(`[package] Desktop version → ${version} (from git describe)`);
  } else {
    console.warn(
      "[package] could not derive version from git; falling back to package.json",
    );
  }

  const useScopedOutputDir = archs.length > 1;

  // Step 3: for each requested architecture, build the matching CLI into
  // resources/bin/ and package that architecture in isolation.
  for (const arch of archs) {
    console.log(`[package] bundling CLI → macOS ${arch}`);
    execFileSync("node", [bundleCliScript, "--target-arch", arch], {
      stdio: "inherit",
      cwd: desktopRoot,
    });

    const builderArgs = builderArgsForArch(arch, parsed, version, {
      useScopedOutputDir,
    });

    // Step 4: invoke electron-builder for the current architecture only.
    const result = spawnSync("electron-builder", builderArgs, {
      stdio: "inherit",
      cwd: desktopRoot,
      env: envWithLocalBins(),
    });

    if (result.error) {
      console.error(
        "[package] failed to spawn electron-builder:",
        result.error.message,
      );
      process.exit(1);
    }
    if (result.status !== 0) {
      process.exit(result.status ?? 1);
    }
  }
}

// Only run when invoked as a CLI, not when imported by a test file.
if (
  process.argv[1] &&
  import.meta.url === pathToFileURL(process.argv[1]).href
) {
  main();
}
