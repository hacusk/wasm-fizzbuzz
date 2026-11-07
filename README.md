# Wasm-FizzBuzz

Go.wasm が中に埋め込んだ Rust、C、Zig の Wasm を実行するデモです。C++ と AssemblyScript は横に並びます。入力は整数 n で、出てくるのはその FizzBuzz です。

## コマンド

```bash
pnpm install
pnpm run build:wasm
pnpm run dev
pnpm run build
pnpm test
```

`pnpm run dev` と `pnpm run build` は、その前に Wasm をビルドします。成果物は `dist/` です。静的ファイルとしてそのまま置けます。

## 必要なツール

- Node.js 20 以上
- Rust（`wasm32-unknown-unknown`）。未導入なら `rustup target add wasm32-unknown-unknown`
- Zig 0.17 以上。C のコンパイルに使う。`zig` が PATH にないときは `.tools/zig/zig` を見る。Emscripten は不要
- TinyGo 0.44 以上。`go.wasm` のコンパイルに使う。`tinygo` が PATH にないときは aqua の展開先を見る。`TINYGO` に実行ファイルのパスを入れてもよい
- AssemblyScript。`pnpm install` で入る。最終出力の Wasm に使う

## 文字列の作り方

JavaScript は n（-2147483648 から 2147483647）を三つの Wasm に渡すだけです。Go がメインで、Rust、C、Zig の Wasm をバイト列として持ち、自分の中のインタプリタで実行します。C++ と AssemblyScript は Go の中には入らず、横に並びます。Go の検証と AssemblyScript の出力が一致したときだけ、画面にその語を出します。

| 層 | 誰が動かすか | 仕事 |
|---|---|---|
| Go.wasm | ブラウザ | Rust、C、Zig を起動し、結果を検証する |
| Rust.wasm | Go.wasm の中のインタプリタ | 3 で割れるか |
| C.wasm | Go.wasm の中のインタプリタ | 5 で割れるか |
| Zig.wasm | Go.wasm の中のインタプリタ | 文字列を結合する |
| C++.wasm | ブラウザ | チェックサムを計算する |
| AssemblyScript.wasm | ブラウザ | 最終出力を作る |
