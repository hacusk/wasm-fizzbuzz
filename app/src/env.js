/**
 * @returns {{
 *   ua: string,
 *   family: 'firefox' | 'chrome' | 'safari' | 'other',
 *   label: string,
 *   webAssembly: boolean,
 *   secure: boolean,
 *   local: boolean,
 *   origin: string,
 * }}
 */
export function detectEnvironment() {
  const ua = navigator.userAgent;
  const firefox = /Firefox\/\d+/.test(ua);
  const chrome = /Chrome\/\d+/.test(ua) && !/Edg\//.test(ua) && !firefox;
  const safari = /Safari\/\d+/.test(ua) && !/Chrome\/\d+/.test(ua);
  const family = firefox ? "firefox" : chrome ? "chrome" : safari ? "safari" : "other";
  const labels = {
    firefox: "Firefox",
    chrome: "Chrome",
    safari: "Safari",
    other: "このブラウザ",
  };
  const host = location.hostname;
  return {
    ua,
    family,
    label: labels[family],
    webAssembly: typeof WebAssembly !== "undefined" && typeof WebAssembly.instantiate === "function",
    secure: location.protocol === "https:" || host === "localhost" || host === "127.0.0.1",
    local: host === "localhost" || host === "127.0.0.1" || host === "[::1]",
    origin: location.origin,
  };
}

/**
 * Honest description of who executes content Wasm.
 * A page cannot see whether Firefox.wasm selected the in-process interpreter
 * or the host passthrough; the published demo's default is the interpreter.
 *
 * @param {ReturnType<typeof detectEnvironment>} env
 */
export function executionNote(env) {
  if (env.family === "chrome") {
    return "このタブの Wasm は Chrome の WebAssembly エンジン（V8）が実行しています。内側の Firefox.wasm で同じページを開くと、実行主体が入れ替わります。";
  }
  if (env.family === "firefox") {
    return "このタブの Wasm は Firefox の WebAssembly 実装が実行しています。Puter の Firefox.wasm は既定で、Gecko の中にある Wasm インタプリタ（エンジン自体が Wasm）でコンテンツの Wasm を実行します。ホストの JIT に渡すのは GECKO_WASM_PASSTHROUGH=1 のときだけです。デスクトップ版 Firefox ではネイティブのコンパイラが実行します。このページだけではその二つを区別できません。";
  }
  return "このタブの Wasm は、いま開いているブラウザの WebAssembly 実装が実行しています。";
}
