import { spawnSync } from "node:child_process";
import { copyFileSync, existsSync, mkdirSync, readdirSync } from "node:fs";
import { homedir } from "node:os";
import { dirname, resolve, sep } from "node:path";
import { fileURLToPath } from "node:url";

const root = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const outDir = resolve(root, "app/public/wasm");
const guestDir = resolve(root, "recipes/go/guest");
const zigCache = resolve(root, "recipes/zig/.zig-cache");

mkdirSync(outDir, { recursive: true });
mkdirSync(guestDir, { recursive: true });
mkdirSync(zigCache, { recursive: true });

function zigBin() {
  if (process.env.ZIG && existsSync(process.env.ZIG)) return process.env.ZIG;
  const local = resolve(root, ".tools/zig/zig");
  if (existsSync(local)) return local;
  return "zig";
}

function tinygoBin() {
  if (process.env.TINYGO && existsSync(process.env.TINYGO)) return process.env.TINYGO;
  const aqua = resolve(homedir(), ".local/share/aquaproj-aqua/pkgs/github_release/github.com/tinygo-org/tinygo");
  const unpacked = findBin(aqua, "tinygo", 0);
  if (unpacked) return unpacked;
  return "tinygo";
}

function findBin(dir, name, depth) {
  if (depth > 6) return null;
  let entries;
  try {
    entries = readdirSync(dir, { withFileTypes: true });
  } catch {
    return null;
  }
  for (const entry of entries) {
    const path = resolve(dir, entry.name);
    if (entry.isFile() && entry.name === name && path.includes(`${sep}bin${sep}`)) return path;
    if (entry.isDirectory()) {
      const hit = findBin(path, name, depth + 1);
      if (hit) return hit;
    }
  }
  return null;
}

function run(cmd, args, options = {}) {
  const result = spawnSync(cmd, args, {
    cwd: options.cwd ?? root,
    stdio: "inherit",
    env: options.env ?? process.env,
  });
  if (result.error) {
    console.error(`failed to start ${cmd}: ${result.error.message}`);
    process.exit(1);
  }
  if (result.status !== 0) process.exit(result.status ?? 1);
}

const zig = zigBin();
const zigProbe = spawnSync(zig, ["version"], { encoding: "utf8" });
if (zigProbe.status !== 0) {
  console.error(
    "zig が見つかりません。Zig 0.17 以降を PATH に入れるか、.tools/zig/zig に置いてください。\n" +
      "macOS arm64: https://ziglang.org/download/0.17.1/zig-aarch64-macos-0.17.1.tar.xz",
  );
  process.exit(1);
}

const cWasm = resolve(guestDir, "c.wasm");
run(zig, [
  "cc",
  "-target",
  "wasm32-freestanding",
  "-mcpu=mvp",
  "-O2",
  "-g0",
  "-nostdlib",
  "-Wl,--no-entry",
  "-Wl,--export-memory",
  "-Wl,--stack=4096",
  "-Wl,--initial-memory=65536",
  "-o",
  cWasm,
  resolve(root, "recipes/c/div5.c"),
]);

const zigWasm = resolve(guestDir, "zig.wasm");
run(zig, [
  "build-exe",
  resolve(root, "recipes/zig/join.zig"),
  "-target",
  "wasm32-freestanding",
  "-mcpu=mvp",
  "-O",
  "ReleaseSmall",
  "-fno-entry",
  "--export-memory",
  "--export=zig_join",
  "--export=zig_out_ptr",
  "--export=zig_out_len",
  "--stack",
  "4096",
  "--initial-memory=65536",
  "--cache-dir",
  zigCache,
  "--global-cache-dir",
  zigCache,
  `-femit-bin=${zigWasm}`,
]);

const rustEnv = {
  ...process.env,
  RUSTFLAGS: "-C target-feature=-simd128,-multivalue,-reference-types,-bulk-memory,-sign-ext,-nontrapping-fptoint",
};
run(
  "cargo",
  ["build", "--release", "--target", "wasm32-unknown-unknown", "--manifest-path", resolve(root, "recipes/rust/Cargo.toml")],
  { env: rustEnv },
);
const cargoTarget = process.env.CARGO_TARGET_DIR || resolve(root, "recipes/rust/target");
const rustBuilt = resolve(cargoTarget, "wasm32-unknown-unknown/release/wasm_fizzbuzz.wasm");
if (!existsSync(rustBuilt)) {
  console.error(`rust wasm が見つかりません: ${rustBuilt}`);
  process.exit(1);
}
copyFileSync(rustBuilt, resolve(guestDir, "rust.wasm"));

run("go", ["test", "./interp"], { cwd: resolve(root, "recipes/go") });

const tinygo = tinygoBin();
const tinygoProbe = spawnSync(tinygo, ["version"], { encoding: "utf8" });
if (tinygoProbe.status !== 0) {
  console.error("tinygo が見つかりません。TinyGo 0.44 以降を PATH に入れるか、TINYGO に実行ファイルのパスを入れてください。");
  process.exit(1);
}
run(tinygo, ["build", "-target", "wasm-unknown", "-opt=2", "-no-debug", "-o", resolve(outDir, "go.wasm"), "."], {
  cwd: resolve(root, "recipes/go"),
});

run(zig, [
  "c++",
  "-target",
  "wasm32-freestanding",
  "-mcpu=mvp",
  "-O2",
  "-g0",
  "-nostdlib",
  "-fno-exceptions",
  "-fno-rtti",
  "-Wl,--no-entry",
  "-Wl,--export-memory",
  "-Wl,--stack=4096",
  "-Wl,--initial-memory=65536",
  "-o",
  resolve(outDir, "cpp.wasm"),
  resolve(root, "recipes/cpp/checksum.cpp"),
]);

run(process.execPath, [
  resolve(root, "node_modules/assemblyscript/bin/asc"),
  resolve(root, "recipes/as/index.ts"),
  "-o",
  resolve(outDir, "as.wasm"),
  "--runtime",
  "stub",
  "-O",
  "--noAssert",
]);

copyFileSync(cWasm, resolve(outDir, "c.wasm"));
copyFileSync(zigWasm, resolve(outDir, "zig.wasm"));
copyFileSync(resolve(guestDir, "rust.wasm"), resolve(outDir, "rust.wasm"));

console.log(`wasm written to ${outDir}`);
