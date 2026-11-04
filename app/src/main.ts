import { detectEnvironment } from "./env.js";

const MIN_N = 1;
const MAX_N = 100;

const log = document.querySelector("#log");
const out = document.querySelector("#out");
const field = document.querySelector<HTMLInputElement>("#n");

let ticket = 0;

type Modules = {
  go: WebAssembly.Instance;
  cpp: WebAssembly.Instance;
  as: WebAssembly.Instance;
};

let modules: Promise<Modules> | null = null;

if (log && out && field) {
  const env = detectEnvironment();
  field.addEventListener("change", () => {
    void run(env);
  });
  field.addEventListener("keydown", (event) => {
    if (event.key === "Enter") {
      event.preventDefault();
      void run(env);
    }
  });
  void run(env);
}

async function run(env: ReturnType<typeof detectEnvironment>): Promise<void> {
  if (!log || !out) return;
  const mine = ++ticket;
  log.textContent = "実行中";
  out.textContent = "";
  const n = readN();
  if (n === null) {
    log.textContent = `n は ${MIN_N} から ${MAX_N} の整数。`;
    return;
  }
  if (!env.webAssembly) {
    log.textContent = "このブラウザには WebAssembly がない。";
    return;
  }
  try {
    const loaded = await loadModules();
    if (mine !== ticket) return;
    const goExports = loaded.go.exports;
    const status = call(goExports.run, n);
    if (status !== 0) {
      if (mine !== ticket) return;
      log.textContent = text(loaded.go, call(goExports.log_ptr), call(goExports.log_len)) || "Go.wasm が失敗した";
      return;
    }
    const goWord = text(loaded.go, call(goExports.out_ptr), call(goExports.out_len));
    const flag3 = call(goExports.flag3);
    const flag5 = call(goExports.flag5);
    const bytes = new TextEncoder().encode(goWord);
    const cppExports = loaded.cpp.exports;
    const cppPtr = call(cppExports.cpp_ptr);
    write(loaded.cpp, cppPtr, bytes);
    const checksum = call(cppExports.checksum, bytes.length) >>> 0;
    const asExports = loaded.as.exports;
    const asIn = call(asExports.as_in_ptr);
    write(loaded.as, asIn, bytes);
    const asLen = call(asExports.as_emit, bytes.length, checksum);
    if (mine !== ticket) return;
    if (asLen < 0) {
      log.textContent = "AssemblyScript がチェックサムを拒んだ";
      return;
    }
    const asWord = text(loaded.as, call(asExports.as_out_ptr), asLen);
    const agreed = asWord === goWord && asWord.length > 0;
    log.textContent = report(env.label, flag3, flag5, goWord, checksum, asWord, agreed);
    if (agreed) {
      out.textContent = asWord;
    }
  } catch (error) {
    if (mine !== ticket) return;
    log.textContent = error instanceof Error ? error.message : String(error);
  }
}

function readN(): number | null {
  if (!field) return null;
  const n = Number(field.value);
  if (!Number.isInteger(n) || n < MIN_N || n > MAX_N) return null;
  return n;
}

function loadModules(): Promise<Modules> {
  if (!modules) {
    modules = createModules().catch((error: unknown) => {
      modules = null;
      throw error;
    });
  }
  return modules;
}

async function createModules(): Promise<Modules> {
  const [goBytes, cppBytes, asBytes] = await Promise.all([
    fetchWasm("go.wasm"),
    fetchWasm("cpp.wasm"),
    fetchWasm("as.wasm"),
  ]);
  const go = await WebAssembly.instantiate(goBytes);
  const cpp = await WebAssembly.instantiate(cppBytes);
  const as = await WebAssembly.instantiate(asBytes, {
    env: {
      abort() {
        throw new Error("AssemblyScript が中断した");
      },
    },
  });
  const init = go.instance.exports._initialize;
  if (typeof init === "function") init();
  return { go: go.instance, cpp: cpp.instance, as: as.instance };
}

async function fetchWasm(name: string): Promise<ArrayBuffer> {
  const response = await fetch(`/wasm/${name}`);
  if (!response.ok) throw new Error(`${name} を読めなかった (${response.status})`);
  return response.arrayBuffer();
}

function report(
  host: string,
  flag3: number,
  flag5: number,
  goWord: string,
  checksum: number,
  asWord: string,
  agreed: boolean,
): string {
  const verdict = agreed ? "一致" : "一致しない";
  return `${host}
  Go.wasm
    Rust.wasm
    C.wasm
    Zig.wasm
  C++.wasm
  AssemblyScript.wasm

Rust.wasm: ${flag3 ? "3で割れる" : "3で割れない"}
C.wasm: ${flag5 ? "5で割れる" : "5で割れない"}
Zig.wasm: ${goWord}
Go.wasm: ${verdict}
C++.wasm: チェックサム ${checksum}
AssemblyScript.wasm: ${asWord}`;
}

function write(instance: WebAssembly.Instance, ptr: number, bytes: Uint8Array): void {
  const memory = instance.exports.memory;
  if (!(memory instanceof WebAssembly.Memory)) throw new Error("memory がない");
  new Uint8Array(memory.buffer, ptr, bytes.length).set(bytes);
}

function text(instance: WebAssembly.Instance, ptr: number, len: number): string {
  const memory = instance.exports.memory;
  if (!(memory instanceof WebAssembly.Memory) || len < 0) return "";
  return new TextDecoder().decode(new Uint8Array(memory.buffer, ptr, len));
}

function call(fn: unknown, ...args: number[]): number {
  if (typeof fn !== "function") throw new Error("wasm の export が見つからない");
  return Number(fn(...args));
}
