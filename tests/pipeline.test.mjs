import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { test } from "node:test";

const wasmDir = new URL("../app/public/wasm/", import.meta.url);

function load(name) {
  return readFileSync(new URL(name, wasmDir));
}

function instantiate(bytes, imports) {
  return WebAssembly.instantiate(bytes, imports).then((result) => result.instance);
}

function text(instance, ptr, len) {
  return new TextDecoder().decode(new Uint8Array(instance.exports.memory.buffer, ptr, len));
}

function fnv(bytes) {
  let hash = 2166136261;
  for (const byte of bytes) hash = Math.imul(hash ^ byte, 16777619) >>> 0;
  return hash;
}

function classic(n) {
  if (n % 15 === 0) return "FizzBuzz";
  if (n % 3 === 0) return "Fizz";
  if (n % 5 === 0) return "Buzz";
  return String(n);
}

test("go.wasm runs rust, c, and zig", async () => {
  const go = await instantiate(load("go.wasm"));
  if (typeof go.exports._initialize === "function") go.exports._initialize();
  const rust = load("rust.wasm");
  const c = load("c.wasm");
  const zig = load("zig.wasm");
  assert.ok(load("go.wasm").includes(rust));
  assert.ok(load("go.wasm").includes(c));
  assert.ok(load("go.wasm").includes(zig));
  for (let n = 1; n <= 100; n++) {
    assert.equal(go.exports.run(n), 0);
    const word = text(go, go.exports.out_ptr(), go.exports.out_len());
    assert.equal(word, classic(n));
    assert.equal(go.exports.flag3(), n % 3 === 0 ? 1 : 0);
    assert.equal(go.exports.flag5(), n % 5 === 0 ? 1 : 0);
  }
  assert.equal(go.exports.run(0), -1);
  assert.equal(go.exports.run(101), -1);
});

test("guest modules answer their own part", async () => {
  const rust = await instantiate(load("rust.wasm"));
  const c = await instantiate(load("c.wasm"));
  const zig = await instantiate(load("zig.wasm"));
  assert.equal(rust.exports.div3(3), 1);
  assert.equal(rust.exports.div3(1), 0);
  assert.equal(c.exports.div5(5), 1);
  assert.equal(c.exports.div5(1), 0);
  const len = zig.exports.zig_join(1, 1, 15);
  assert.equal(text(zig, zig.exports.zig_out_ptr(), len), "FizzBuzz");
});

test("c++ checksum and assemblyscript agree", async () => {
  const cpp = await instantiate(load("cpp.wasm"));
  const as = await instantiate(load("as.wasm"), { env: { abort() {} } });
  for (let n = 1; n <= 100; n++) {
    const bytes = new TextEncoder().encode(classic(n));
    new Uint8Array(cpp.exports.memory.buffer, cpp.exports.cpp_ptr(), bytes.length).set(bytes);
    const checksum = cpp.exports.checksum(bytes.length) >>> 0;
    assert.equal(checksum, fnv(bytes));
    new Uint8Array(as.exports.memory.buffer, as.exports.as_in_ptr(), bytes.length).set(bytes);
    assert.equal(as.exports.as_emit(bytes.length, checksum), bytes.length, String(n));
    assert.equal(text(as, as.exports.as_out_ptr(), bytes.length), classic(n));
  }
  assert.equal(as.exports.as_emit(1, 1), -1);
});

test("go, c++, and assemblyscript import only what they must", () => {
  assert.deepEqual(WebAssembly.Module.imports(new WebAssembly.Module(load("go.wasm"))), []);
  assert.deepEqual(WebAssembly.Module.imports(new WebAssembly.Module(load("cpp.wasm"))), []);
  assert.deepEqual(
    WebAssembly.Module.imports(new WebAssembly.Module(load("as.wasm"))).map((item) => item.module + "." + item.name),
    ["env.abort"],
  );
});

test("page script does not contain the answer words", () => {
  const source = readFileSync(new URL("../app/src/main.ts", import.meta.url), "utf8");
  assert.equal(source.includes("Fizz"), false);
  assert.equal(source.includes("Buzz"), false);
  assert.equal(source.includes("Hello, World!"), false);
});
