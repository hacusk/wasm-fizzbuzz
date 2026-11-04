package main

import (
	_ "embed"

	"wasm-fizzbuzz/interp"
)

//go:embed guest/rust.wasm
var rustWasm []byte

//go:embed guest/c.wasm
var cWasm []byte

//go:embed guest/zig.wasm
var zigWasm []byte

var (
	outBuf [64]byte
	outLen int
	logBuf [256]byte
	logLen int
	div3   int32
	div5   int32
)

//export run
func run(n int32) int32 {
	outLen = 0
	logLen = 0
	div3 = 0
	div5 = 0
	result, err := interp.Fizz(n, rustWasm, cWasm, zigWasm)
	if err != nil {
		setLog(err.Error())
		return -1
	}
	div3 = result.Div3
	div5 = result.Div5
	copy(outBuf[:], result.Word)
	outLen = len(result.Word)
	return 0
}

//export out_ptr
func outPtr() *byte { return &outBuf[0] }

//export out_len
func outLenFn() int32 { return int32(outLen) }

//export log_ptr
func logPtr() *byte { return &logBuf[0] }

//export log_len
func logLenFn() int32 { return int32(logLen) }

//export flag3
func flag3() int32 { return div3 }

//export flag5
func flag5() int32 { return div5 }

func setLog(message string) {
	n := copy(logBuf[:], message)
	logLen = n
}

func main() {}
