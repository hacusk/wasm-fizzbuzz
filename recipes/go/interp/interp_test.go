package interp

import (
	"math"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func guest(name string) []byte {
	_, file, _, _ := runtime.Caller(0)
	raw, err := os.ReadFile(filepath.Join(filepath.Dir(file), "..", "guest", name))
	if err != nil {
		panic(err)
	}
	return raw
}

func TestFizz(t *testing.T) {
	rust := guest("rust.wasm")
	c := guest("c.wasm")
	zig := guest("zig.wasm")
	samples := []int32{math.MinInt32, -15, -5, -3, -1, 0, 101, math.MaxInt32}
	for n := int32(1); n <= 100; n++ {
		samples = append(samples, n)
	}
	for _, n := range samples {
		got, err := Fizz(n, rust, c, zig)
		if err != nil {
			t.Fatalf("n=%d: %v", n, err)
		}
		want3, want5 := int32(0), int32(0)
		if n%3 == 0 {
			want3 = 1
		}
		if n%5 == 0 {
			want5 = 1
		}
		if got.Div3 != want3 || got.Div5 != want5 {
			t.Fatalf("n=%d flags got %d %d", n, got.Div3, got.Div5)
		}
		if got.Word != classic(n) {
			t.Fatalf("n=%d word %q", n, got.Word)
		}
	}
}

func classic(n int32) string {
	switch {
	case n%15 == 0:
		return "Fizz" + "Buzz"
	case n%3 == 0:
		return "Fizz"
	case n%5 == 0:
		return "Buzz"
	default:
		return itoa(n)
	}
}
