import { execFileSync } from "node:child_process";
import { existsSync, mkdtempSync, readFileSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { delimiter, join, resolve } from "node:path";
import { afterEach, describe, it, expect } from "vitest";
import {
  builderArgsForArch,
  deriveVersion,
  DESCRIBE_ARGS,
  envWithLocalBins,
  normalizeGitVersion,
  parsePackageArgs,
  resolveBuildArchs,
  stripLeadingSeparator,
} from "./package.mjs";

describe("normalizeGitVersion", () => {
  it("returns null for empty / nullish input", () => {
    expect(normalizeGitVersion("")).toBe(null);
    expect(normalizeGitVersion(null)).toBe(null);
    expect(normalizeGitVersion(undefined)).toBe(null);
  });

  it("strips the leading v on a clean tag", () => {
    expect(normalizeGitVersion("v0.1.36")).toBe("0.1.36");
    expect(normalizeGitVersion("v1.0.0")).toBe("1.0.0");
  });

  it("preserves the prerelease suffix between tags", () => {
    expect(normalizeGitVersion("v0.1.35-14-gf1415e96")).toBe(
      "0.1.35-14-gf1415e96",
    );
  });

  it("preserves the dirty suffix on a modified worktree", () => {
    expect(normalizeGitVersion("v0.1.35-14-gf1415e96-dirty")).toBe(
      "0.1.35-14-gf1415e96-dirty",
    );
  });

  it("handles v-prefixed prerelease tags", () => {
    expect(normalizeGitVersion("v1.0.0-alpha")).toBe("1.0.0-alpha");
    expect(normalizeGitVersion("v1.0.0-rc.2")).toBe("1.0.0-rc.2");
  });

  it("falls back to 0.0.0-g<hash> when no tags are reachable", () => {
    // `git describe --tags --always` returns just the short commit hash
    // when there are no tags in the history at all. A hash that begins with
    // a digit (e.g. "2f24057b") is still not valid semver and must fall
    // through — otherwise electron-builder rejects it. The `g` prefix
    // mirrors git describe's own `g<hash>` shorthand and keeps the
    // pre-release identifier a single alphanumeric token.
    expect(normalizeGitVersion("f1415e96")).toBe("0.0.0-gf1415e96");
    expect(normalizeGitVersion("abc1234")).toBe("0.0.0-gabc1234");
    expect(normalizeGitVersion("2f24057b")).toBe("0.0.0-g2f24057b");
  });

  it("degrades a non-semver tag prefix that slips past the --match filter", () => {
    // `git describe` is invoked with `--match 'v[0-9]*'` so a release-train
    // tag like `release_iteration/…` is never the nearest match; the version
    // resolves to the `vX.Y.Z-N-g<hash>` shape instead. If that filter ever
    // regresses, the describe output carries the non-semver tag verbatim and
    // must NOT be passed through as a version — it has no `major.minor.patch`
    // prefix, so it degrades to the `0.0.0-g<hash>` fallback rather than
    // producing something electron-builder would reject.
    expect(
      normalizeGitVersion("release_iteration/Sprint_0705-3-g9adfcd4d8"),
    ).toBe("0.0.0-grelease_iteration/Sprint_0705-3-g9adfcd4d8");
    // With the filter in place the real input is well-formed and passes through.
    expect(normalizeGitVersion("v0.3.35-38-g9adfcd4d8")).toBe(
      "0.3.35-38-g9adfcd4d8",
    );
  });

  it("prefixes an all-digit hash so the pre-release is valid semver", () => {
    // A short hash that is all decimal digits with a leading zero would
    // produce `0.0.0-0123456` — a numeric pre-release identifier must not
    // have a leading zero, so that value is invalid semver and
    // electron-builder would reject the no-tag builds this fallback exists
    // to protect. The `g` prefix makes it a single alphanumeric identifier,
    // which is always valid.
    expect(normalizeGitVersion("0123456")).toBe("0.0.0-g0123456");
    expect(normalizeGitVersion("04567")).toBe("0.0.0-g04567");
  });
});

describe("DESCRIBE_ARGS", () => {
  it("passes the match pattern as one bare argv token, never a shell-quoted string", () => {
    // A quoted pattern would reach git with its quotes and match no tag,
    // so tagged builds would fall back to a synthetic 0.0.0-g<hash> version.
    expect(DESCRIBE_ARGS).toContain("v[0-9]*");
    for (const arg of DESCRIBE_ARGS) {
      expect(arg).not.toContain("'");
      expect(arg).not.toContain('"');
    }
  });
});

describe("deriveVersion (real git describe)", () => {
  // These exercise the actual `git describe` invocation — not just the
  // normalizeGitVersion string transform — because how git is called
  // decides whether the `v[0-9]*` match pattern reaches it as a literal
  // argument.
  const repos = [];

  function initRepo() {
    const dir = mkdtempSync(join(tmpdir(), "multica-desktop-ver-"));
    repos.push(dir);
    const run = (...args) =>
      execFileSync("git", args, { cwd: dir, encoding: "utf-8" });
    run("init", "-q");
    run("config", "user.email", "test@multica.ai");
    run("config", "user.name", "test");
    run("config", "commit.gpgsign", "false");
    run("commit", "-q", "--allow-empty", "-m", "root");
    return { dir, run };
  }

  afterEach(() => {
    while (repos.length) rmSync(repos.pop(), { recursive: true, force: true });
  });

  it("resolves a clean semver tag to its bare version", () => {
    const { dir, run } = initRepo();
    run("tag", "v1.4.2");
    expect(deriveVersion(dir)).toBe("1.4.2");
  });

  it("selects the semver tag even when a nearer non-semver tag exists", () => {
    // A release-train tag like `release_iteration/…` sitting closer to HEAD
    // must not become the version. With the match pattern correctly reaching
    // git, describe skips it and reports the real vX.Y.Z tag. If the pattern
    // were mangled (e.g. quotes leaking through a shell) git would match
    // nothing and the version would collapse to `0.0.0-…`.
    const { dir, run } = initRepo();
    run("tag", "v1.4.2");
    run("commit", "-q", "--allow-empty", "-m", "sprint");
    run("tag", "release_iteration/Sprint_0705");
    const version = deriveVersion(dir);
    expect(version).toMatch(/^1\.4\.2-1-g[0-9a-f]+$/);
    expect(version).not.toMatch(/^0\.0\.0/);
  });

  it("falls back to 0.0.0-g<hash> when no semver tag is reachable", () => {
    const { dir } = initRepo();
    expect(deriveVersion(dir)).toMatch(/^0\.0\.0-g[0-9a-f]+$/);
  });
});

describe("stripLeadingSeparator", () => {
  it("removes the leading -- inserted by npm/pnpm", () => {
    expect(stripLeadingSeparator(["--", "--mac", "--arm64", "--publish", "always"])).toEqual([
      "--mac", "--arm64", "--publish", "always",
    ]);
  });

  it("leaves args untouched when there is no leading --", () => {
    expect(stripLeadingSeparator(["--mac", "--arm64"])).toEqual(["--mac", "--arm64"]);
  });

  it("does not strip a -- that appears mid-argv", () => {
    expect(stripLeadingSeparator(["--mac", "--", "--arm64"])).toEqual([
      "--mac", "--", "--arm64",
    ]);
  });

  it("handles an empty array", () => {
    expect(stripLeadingSeparator([])).toEqual([]);
  });
});

describe("parsePackageArgs", () => {
  it("collects mac targets, architectures and shared args", () => {
    expect(
      parsePackageArgs([
        "--mac", "dir",
        "--arm64",
        "--publish", "never",
      ]),
    ).toEqual({
      sharedArgs: ["--publish", "never"],
      macTargets: ["dir"],
      requestedArchs: ["arm64"],
    });
  });

  it("accepts the --macos and -m aliases without targets", () => {
    expect(parsePackageArgs(["-m", "--macos", "--x64"])).toEqual({
      sharedArgs: [],
      macTargets: [],
      requestedArchs: ["x64"],
    });
  });
});

describe("resolveBuildArchs", () => {
  const parsed = { sharedArgs: [], macTargets: [], requestedArchs: [] };

  it("defaults to the host architecture", () => {
    expect(resolveBuildArchs(parsed, "darwin", "arm64")).toEqual(["arm64"]);
  });

  it("builds every requested architecture", () => {
    expect(
      resolveBuildArchs(
        { ...parsed, requestedArchs: ["arm64", "x64"] },
        "darwin",
        "arm64",
      ),
    ).toEqual(["arm64", "x64"]);
  });

  it("rejects unsupported architectures", () => {
    expect(() =>
      resolveBuildArchs(
        { ...parsed, requestedArchs: ["universal"] },
        "darwin",
        "arm64",
      ),
    ).toThrow(/unsupported Desktop CLI architecture/);
  });

  it("rejects hosts other than macOS", () => {
    expect(() => resolveBuildArchs(parsed, "linux", "x64")).toThrow(
      /macOS only/,
    );
  });
});

describe("builderArgsForArch", () => {
  it("builds the host architecture into the default output directory", () => {
    expect(
      builderArgsForArch(
        "arm64",
        { sharedArgs: ["--publish", "never"], macTargets: [], requestedArchs: [] },
        "1.2.3",
      ),
    ).toEqual([
      "-c.extraMetadata.version=1.2.3",
      "--mac",
      "--arm64",
      "--publish",
      "never",
    ]);
  });

  it("adds scoped output directories and forwards mac targets", () => {
    expect(
      builderArgsForArch(
        "x64",
        {
          sharedArgs: ["--publish", "never"],
          macTargets: ["dir"],
          requestedArchs: ["arm64", "x64"],
        },
        "1.2.3",
        { useScopedOutputDir: true },
      ),
    ).toEqual([
      "-c.extraMetadata.version=1.2.3",
      "--mac",
      "dir",
      "--x64",
      "--publish",
      "never",
      "-c.directories.output=dist/mac-x64",
    ]);
  });

  it("leaves the package.json version when none was derived", () => {
    expect(
      builderArgsForArch(
        "arm64",
        { sharedArgs: [], macTargets: [], requestedArchs: [] },
        null,
      ),
    ).toEqual(["--mac", "--arm64"]);
  });
});

describe("envWithLocalBins", () => {
  it("prepends desktop-local binary directories to PATH", () => {
    const desktopRoot = "/repo/apps/desktop";
    const result = envWithLocalBins(
      { PATH: ["/usr/local/bin", "/usr/bin"].join(delimiter) },
      desktopRoot,
    );
    expect(result.PATH.split(delimiter)).toEqual([
      resolve(desktopRoot, "node_modules", ".bin"),
      resolve(desktopRoot, "..", "..", "node_modules", ".bin"),
      "/usr/local/bin",
      "/usr/bin",
    ]);
  });

  it("preserves an existing Path key and avoids duplicate entries", () => {
    const desktopRoot = "/repo/apps/desktop";
    const desktopBin = resolve(desktopRoot, "node_modules", ".bin");
    const workspaceBin = resolve(desktopRoot, "..", "..", "node_modules", ".bin");
    const result = envWithLocalBins(
      { Path: [desktopBin, "runner-bin", workspaceBin].join(delimiter) },
      desktopRoot,
    );
    expect(result).not.toHaveProperty("PATH");
    expect(result.Path.split(delimiter)).toEqual([
      desktopBin,
      workspaceBin,
      "runner-bin",
    ]);
  });
});

describe("electron-builder.yml packaging config", () => {
  // A multi-arch build writes each architecture's output to dist/mac-<arch>
  // in the same apps/desktop dir; electron-builder only auto-excludes the
  // *current* output dir, so without an explicit `!dist/**` the earlier
  // architecture's app is repacked into the next architecture's app.asar.
  // Keep the exclusion pinned so a future edit to the files list cannot drop
  // it unnoticed.
  // Resolve electron-builder.yml relative to cwd, tolerating vitest running
  // from either the desktop package dir or the repo root — import.meta.url is
  // not a file:// URL under the test transform, so avoid fileURLToPath here.
  const configPath = [
    resolve(process.cwd(), "electron-builder.yml"),
    resolve(process.cwd(), "apps/desktop/electron-builder.yml"),
  ].find((candidate) => existsSync(candidate));

  // Extract the entries of the top-level `files:` block sequence without a
  // YAML dependency: collect the `  - "…"` items that follow `files:` up to
  // the next top-level key. Commented (`#`) lines are ignored, so a
  // commented-out exclusion would (correctly) not count.
  function readFilesBlock(raw) {
    const lines = raw.split("\n");
    const start = lines.findIndex((l) => /^files:\s*$/.test(l));
    if (start === -1) return [];
    const entries = [];
    for (let i = start + 1; i < lines.length; i += 1) {
      const line = lines[i];
      if (/^\S/.test(line)) break; // next top-level key ends the block
      const trimmed = line.trim();
      if (trimmed === "" || trimmed.startsWith("#")) continue;
      const m = trimmed.match(/^-\s*"?(.*?)"?\s*$/);
      if (m) entries.push(m[1]);
    }
    return entries;
  }

  it("excludes prior architecture output from packaged files", () => {
    expect(configPath, "electron-builder.yml not found").toBeTruthy();
    const entries = readFilesBlock(readFileSync(configPath, "utf-8"));
    expect(entries.length).toBeGreaterThan(0);
    expect(entries).toContain("!dist/**");
  });
});
