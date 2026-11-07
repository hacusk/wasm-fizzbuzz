// A small WebAssembly interpreter for the integer MVP subset used by the
// guest modules. Go.wasm uses it to run Rust, C, and Zig.
package interp

import (
	"encoding/binary"
	"fmt"
	"math"
)

const pageSize = 65536

type Result struct {
	Word string
	Div3 int32
	Div5 int32
}

type Instance struct {
	funcs   []function
	globals []uint32
	mem     []byte
	memMax  int
	exports map[string]int
}

type function struct {
	params  int
	results int
	extra   int
	body    []instr
}

type op uint8

const (
	opUnreachable op = iota
	opNop
	opDrop
	opSelect
	opI32Const
	opLocalGet
	opLocalSet
	opLocalTee
	opGlobalGet
	opGlobalSet
	opBlock
	opLoop
	opIf
	opBr
	opBrIf
	opBrTable
	opReturn
	opCall
	opLoad
	opStore
	opMemorySize
	opMemoryGrow
	opUnary
	opBinary
	opCompare
)

type width uint8

const (
	w32 width = iota
	w8s
	w8u
	w16s
	w16u
)

type instr struct {
	op      op
	a       uint32
	imm     int32
	body    []instr
	other   []instr
	labels  []uint32
	width   width
	brArity int
	loop    bool
	code    byte
}

type sigKind uint8

const (
	sigNone sigKind = iota
	sigBr
	sigReturn
)

type sig struct {
	kind  sigKind
	depth int
}

type vm struct {
	inst   *Instance
	stack  []uint32
	locals []uint32
}

type reader struct {
	b []byte
	i int
}

func Fizz(n int32, rustWASM, cWASM, zigWASM []byte) (Result, error) {
	rust, err := Load(rustWASM)
	if err != nil {
		return Result{}, fmt.Errorf("Rust.wasm: %w", err)
	}
	cmod, err := Load(cWASM)
	if err != nil {
		return Result{}, fmt.Errorf("C.wasm: %w", err)
	}
	zig, err := Load(zigWASM)
	if err != nil {
		return Result{}, fmt.Errorf("Zig.wasm: %w", err)
	}
	div3, err := rust.Call("div3", uint32(n))
	if err != nil {
		return Result{}, fmt.Errorf("div3: %w", err)
	}
	div5, err := cmod.Call("div5", uint32(n))
	if err != nil {
		return Result{}, fmt.Errorf("div5: %w", err)
	}
	if (div3 != 0 && div3 != 1) || (div5 != 0 && div5 != 1) {
		return Result{}, fmt.Errorf("判定が 0 と 1 以外")
	}
	length, err := zig.Call("zig_join", div3, div5, uint32(n))
	if err != nil {
		return Result{}, fmt.Errorf("zig_join: %w", err)
	}
	if int32(length) < 0 || length > 32 {
		return Result{}, fmt.Errorf("Zig.wasm の長さが異常")
	}
	ptr, err := zig.Call("zig_out_ptr")
	if err != nil {
		return Result{}, fmt.Errorf("zig_out_ptr: %w", err)
	}
	if int(ptr)+int(length) > len(zig.mem) {
		return Result{}, fmt.Errorf("Zig.wasm の出力がメモリの外")
	}
	word := string(zig.mem[ptr : ptr+length])
	if word != expect(div3, div5, n) {
		return Result{}, fmt.Errorf("Zig.wasm の文字列が判定と一致しない")
	}
	return Result{Word: word, Div3: int32(div3), Div5: int32(div5)}, nil
}

func expect(div3, div5 uint32, n int32) string {
	var word string
	if div3 != 0 {
		word += "Fizz"
	}
	if div5 != 0 {
		word += "Buzz"
	}
	if word == "" {
		return itoa(n)
	}
	return word
}

func itoa(n int32) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	var v uint32
	switch {
	case n == math.MinInt32:
		v = 1 << 31
	case neg:
		v = uint32(-n)
	default:
		v = uint32(n)
	}
	var buf [16]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

func Load(wasm []byte) (*Instance, error) {
	if len(wasm) < 8 || string(wasm[:4]) != "\x00asm" || binary.LittleEndian.Uint32(wasm[4:8]) != 1 {
		return nil, fmt.Errorf("Wasm のヘッダではない")
	}
	inst := &Instance{exports: map[string]int{}}
	var types []struct{ params, results int }
	var typeIdx []uint32
	var codeReaders []reader
	r := reader{b: wasm, i: 8}
	for r.i < len(r.b) {
		id, err := r.u8()
		if err != nil {
			return nil, err
		}
		size, err := r.u32()
		if err != nil {
			return nil, err
		}
		if int(size) < 0 || r.i+int(size) > len(r.b) {
			return nil, fmt.Errorf("セクションがはみ出している")
		}
		body := reader{b: r.b[r.i : r.i+int(size)]}
		r.i += int(size)
		switch id {
		case 0, 4, 9: // custom, table, element
		case 1:
			n, err := body.u32()
			if err != nil {
				return nil, err
			}
			for i := uint32(0); i < n; i++ {
				form, err := body.u8()
				if err != nil || form != 0x60 {
					return nil, fmt.Errorf("関数型ではない")
				}
				params, err := body.u32()
				if err != nil {
					return nil, err
				}
				if err := body.skip(int(params)); err != nil {
					return nil, err
				}
				results, err := body.u32()
				if err != nil {
					return nil, err
				}
				if err := body.skip(int(results)); err != nil {
					return nil, err
				}
				types = append(types, struct{ params, results int }{int(params), int(results)})
			}
		case 2:
			n, err := body.u32()
			if err != nil {
				return nil, err
			}
			if n != 0 {
				return nil, fmt.Errorf("import がある")
			}
		case 3:
			n, err := body.u32()
			if err != nil {
				return nil, err
			}
			for i := uint32(0); i < n; i++ {
				idx, err := body.u32()
				if err != nil {
					return nil, err
				}
				typeIdx = append(typeIdx, idx)
			}
		case 5:
			n, err := body.u32()
			if err != nil {
				return nil, err
			}
			if n != 1 {
				return nil, fmt.Errorf("メモリが1つではない")
			}
			flags, err := body.u8()
			if err != nil {
				return nil, err
			}
			min, err := body.u32()
			if err != nil {
				return nil, err
			}
			maxPages := 0
			if flags&0x1 != 0 {
				maxv, err := body.u32()
				if err != nil {
					return nil, err
				}
				maxPages = int(maxv)
			}
			if min > 64 {
				return nil, fmt.Errorf("メモリが大きすぎる")
			}
			inst.mem = make([]byte, int(min)*pageSize)
			inst.memMax = maxPages
		case 6:
			n, err := body.u32()
			if err != nil {
				return nil, err
			}
			for i := uint32(0); i < n; i++ {
				if _, err := body.u8(); err != nil {
					return nil, err
				}
				if _, err := body.u8(); err != nil {
					return nil, err
				}
				value, err := evalConst(&body)
				if err != nil {
					return nil, err
				}
				inst.globals = append(inst.globals, value)
			}
		case 7:
			n, err := body.u32()
			if err != nil {
				return nil, err
			}
			for i := uint32(0); i < n; i++ {
				name, err := body.name()
				if err != nil {
					return nil, err
				}
				kind, err := body.u8()
				if err != nil {
					return nil, err
				}
				idx, err := body.u32()
				if err != nil {
					return nil, err
				}
				if kind == 0 {
					inst.exports[name] = int(idx)
				}
			}
		case 8:
			return nil, fmt.Errorf("start 関数には未対応")
		case 10:
			n, err := body.u32()
			if err != nil {
				return nil, err
			}
			for i := uint32(0); i < n; i++ {
				size, err := body.u32()
				if err != nil {
					return nil, err
				}
				if body.i+int(size) > len(body.b) {
					return nil, fmt.Errorf("コードがはみ出している")
				}
				codeReaders = append(codeReaders, reader{b: body.b[body.i : body.i+int(size)]})
				body.i += int(size)
			}
		case 11:
			if inst.mem == nil {
				return nil, fmt.Errorf("データがあるのにメモリがない")
			}
			n, err := body.u32()
			if err != nil {
				return nil, err
			}
			for i := uint32(0); i < n; i++ {
				flags, err := body.u8()
				if err != nil {
					return nil, err
				}
				if flags&0x1 != 0 {
					return nil, fmt.Errorf("passive なデータには未対応")
				}
				if flags&0x2 != 0 {
					if _, err := body.u32(); err != nil {
						return nil, err
					}
				}
				offset, err := evalConst(&body)
				if err != nil {
					return nil, err
				}
				nbyte, err := body.u32()
				if err != nil {
					return nil, err
				}
				raw, err := body.take(int(nbyte))
				if err != nil {
					return nil, err
				}
				end := int(offset) + len(raw)
				if end > len(inst.mem) {
					return nil, fmt.Errorf("データがメモリの外")
				}
				copy(inst.mem[offset:], raw)
			}
		default:
			return nil, fmt.Errorf("未対応のセクション %d", id)
		}
	}
	if len(codeReaders) != len(typeIdx) {
		return nil, fmt.Errorf("関数の数とコードの数が違う")
	}
	inst.funcs = make([]function, len(typeIdx))
	for i, idx := range typeIdx {
		if int(idx) >= len(types) {
			return nil, fmt.Errorf("型インデックスが範囲外")
		}
		fn, err := decodeFunc(codeReaders[i], types[idx].params, types[idx].results)
		if err != nil {
			return nil, fmt.Errorf("関数 %d: %w", i, err)
		}
		inst.funcs[i] = fn
	}
	return inst, nil
}

func (inst *Instance) Call(name string, args ...uint32) (uint32, error) {
	idx, ok := inst.exports[name]
	if !ok || idx < 0 || idx >= len(inst.funcs) {
		return 0, fmt.Errorf("%s がない", name)
	}
	fn := inst.funcs[idx]
	if len(args) != fn.params {
		return 0, fmt.Errorf("%s の引数の数が違う", name)
	}
	locals := make([]uint32, fn.params+fn.extra)
	copy(locals, args)
	machine := vm{inst: inst, locals: locals}
	end := machine.exec(fn.body)
	if end.kind == sigBr {
		return 0, fmt.Errorf("%s の分岐が関数の外に出た", name)
	}
	if fn.results == 0 {
		return 0, nil
	}
	if len(machine.stack) < 1 {
		return 0, fmt.Errorf("%s の戻り値がない", name)
	}
	return machine.stack[len(machine.stack)-1], nil
}

func decodeFunc(r reader, params, results int) (function, error) {
	groups, err := r.u32()
	if err != nil {
		return function{}, err
	}
	extra := 0
	for i := uint32(0); i < groups; i++ {
		count, err := r.u32()
		if err != nil {
			return function{}, err
		}
		if _, err := r.u8(); err != nil {
			return function{}, err
		}
		extra += int(count)
	}
	body, term, err := r.seq()
	if err != nil {
		return function{}, err
	}
	if term != 0x0b {
		return function{}, fmt.Errorf("関数が end で終わっていない")
	}
	return function{params: params, results: results, extra: extra, body: body}, nil
}

func (r *reader) seq() ([]instr, byte, error) {
	var out []instr
	for {
		if r.i >= len(r.b) {
			return nil, 0, fmt.Errorf("命令が途中で終わった")
		}
		mark := r.i
		raw, err := r.u8()
		if err != nil {
			return nil, 0, err
		}
		if raw == 0x0b || raw == 0x05 {
			return out, raw, nil
		}
		r.i = mark
		in, err := r.instr()
		if err != nil {
			return nil, 0, err
		}
		out = append(out, in)
	}
}

func (r *reader) instr() (instr, error) {
	raw, err := r.u8()
	if err != nil {
		return instr{}, err
	}
	switch raw {
	case 0x00:
		return instr{op: opUnreachable}, nil
	case 0x01:
		return instr{op: opNop}, nil
	case 0x1a:
		return instr{op: opDrop}, nil
	case 0x1b:
		return instr{op: opSelect}, nil
	case 0x02, 0x03, 0x04:
		bt, err := r.u8()
		if err != nil {
			return instr{}, err
		}
		arity := 0
		if bt == 0x7f || bt == 0x7e {
			arity = 1
		} else if bt != 0x40 {
			return instr{}, fmt.Errorf("未対応の blocktype 0x%02x", bt)
		}
		body, term, err := r.seq()
		if err != nil {
			return instr{}, err
		}
		in := instr{body: body}
		if raw == 0x03 {
			in.op = opLoop
			in.loop = true
			in.brArity = 0
		} else if raw == 0x02 {
			in.op = opBlock
			in.brArity = arity
		} else {
			in.op = opIf
			in.brArity = arity
			if term == 0x05 {
				other, term2, err := r.seq()
				if err != nil {
					return instr{}, err
				}
				if term2 != 0x0b {
					return instr{}, fmt.Errorf("if が end で終わっていない")
				}
				in.other = other
			}
		}
		return in, nil
	case 0x0c:
		depth, err := r.u32()
		if err != nil {
			return instr{}, err
		}
		return instr{op: opBr, a: depth}, nil
	case 0x0d:
		depth, err := r.u32()
		if err != nil {
			return instr{}, err
		}
		return instr{op: opBrIf, a: depth}, nil
	case 0x0e:
		n, err := r.u32()
		if err != nil {
			return instr{}, err
		}
		labels := make([]uint32, int(n)+1)
		for i := range labels {
			labels[i], err = r.u32()
			if err != nil {
				return instr{}, err
			}
		}
		return instr{op: opBrTable, labels: labels}, nil
	case 0x0f:
		return instr{op: opReturn}, nil
	case 0x10:
		idx, err := r.u32()
		if err != nil {
			return instr{}, err
		}
		return instr{op: opCall, a: idx}, nil
	case 0x20, 0x21, 0x22:
		idx, err := r.u32()
		if err != nil {
			return instr{}, err
		}
		kind := opLocalGet
		if raw == 0x21 {
			kind = opLocalSet
		} else if raw == 0x22 {
			kind = opLocalTee
		}
		return instr{op: kind, a: idx}, nil
	case 0x23, 0x24:
		idx, err := r.u32()
		if err != nil {
			return instr{}, err
		}
		kind := opGlobalGet
		if raw == 0x24 {
			kind = opGlobalSet
		}
		return instr{op: kind, a: idx}, nil
	case 0x28, 0x2c, 0x2d, 0x2e, 0x2f, 0x36, 0x3a, 0x3b:
		if _, err := r.u32(); err != nil {
			return instr{}, err
		}
		offset, err := r.u32()
		if err != nil {
			return instr{}, err
		}
		in := instr{a: offset}
		switch raw {
		case 0x28:
			in.op, in.width = opLoad, w32
		case 0x2c:
			in.op, in.width = opLoad, w8s
		case 0x2d:
			in.op, in.width = opLoad, w8u
		case 0x2e:
			in.op, in.width = opLoad, w16s
		case 0x2f:
			in.op, in.width = opLoad, w16u
		case 0x36:
			in.op, in.width = opStore, w32
		case 0x3a:
			in.op, in.width = opStore, w8u
		case 0x3b:
			in.op, in.width = opStore, w16u
		}
		return in, nil
	case 0x3f, 0x40:
		if _, err := r.u8(); err != nil {
			return instr{}, err
		}
		if raw == 0x3f {
			return instr{op: opMemorySize}, nil
		}
		return instr{op: opMemoryGrow}, nil
	case 0x41:
		value, err := r.s32()
		if err != nil {
			return instr{}, err
		}
		return instr{op: opI32Const, imm: value}, nil
	case 0x45:
		return instr{op: opUnary, code: raw}, nil
	case 0x46, 0x47, 0x48, 0x49, 0x4a, 0x4b, 0x4c, 0x4d, 0x4e, 0x4f:
		return instr{op: opCompare, code: raw}, nil
	case 0x67, 0x68, 0x69:
		return instr{op: opUnary, code: raw}, nil
	case 0x6a, 0x6b, 0x6c, 0x6d, 0x6e, 0x6f, 0x70, 0x71, 0x72, 0x73, 0x74, 0x75, 0x76, 0x77, 0x78:
		return instr{op: opBinary, code: raw}, nil
	default:
		return instr{}, fmt.Errorf("未対応の命令 0x%02x", raw)
	}
}

func evalConst(r *reader) (uint32, error) {
	op, err := r.u8()
	if err != nil {
		return 0, err
	}
	if op != 0x41 {
		return 0, fmt.Errorf("定数式が i32.const ではない")
	}
	value, err := r.s32()
	if err != nil {
		return 0, err
	}
	end, err := r.u8()
	if err != nil || end != 0x0b {
		return 0, fmt.Errorf("定数式が end で終わっていない")
	}
	return uint32(value), nil
}

func (m *vm) exec(code []instr) sig {
	for i := range code {
		in := &code[i]
		switch in.op {
		case opUnreachable:
			m.stack = append(m.stack, 0xdead)
			return sig{kind: sigReturn}
		case opNop:
		case opDrop:
			m.pop()
		case opSelect:
			cond := m.pop()
			v2 := m.pop()
			v1 := m.pop()
			if cond != 0 {
				m.push(v1)
			} else {
				m.push(v2)
			}
		case opI32Const:
			m.push(uint32(in.imm))
		case opLocalGet:
			m.push(m.locals[in.a])
		case opLocalSet:
			m.locals[in.a] = m.pop()
		case opLocalTee:
			m.locals[in.a] = m.top()
		case opGlobalGet:
			m.push(m.inst.globals[in.a])
		case opGlobalSet:
			m.inst.globals[in.a] = m.pop()
		case opBlock, opLoop, opIf:
			if end := m.block(in); end.kind != sigNone {
				return end
			}
		case opBr:
			return sig{kind: sigBr, depth: int(in.a)}
		case opBrIf:
			if m.pop() != 0 {
				return sig{kind: sigBr, depth: int(in.a)}
			}
		case opBrTable:
			idx := m.pop()
			target := in.labels[len(in.labels)-1]
			if int(idx) < len(in.labels)-1 {
				target = in.labels[idx]
			}
			return sig{kind: sigBr, depth: int(target)}
		case opReturn:
			return sig{kind: sigReturn}
		case opCall:
			if end := m.call(in.a); end.kind != sigNone {
				return end
			}
		case opLoad:
			addr := m.pop() + in.a
			m.push(m.load(addr, in.width))
		case opStore:
			value := m.pop()
			addr := m.pop() + in.a
			m.store(addr, value, in.width)
		case opMemorySize:
			m.push(uint32(len(m.inst.mem) / pageSize))
		case opMemoryGrow:
			m.push(m.grow(m.pop()))
		case opUnary:
			m.push(unary(in.code, m.pop()))
		case opBinary:
			rhs := m.pop()
			lhs := m.pop()
			m.push(binaryOp(in.code, lhs, rhs))
		case opCompare:
			rhs := m.pop()
			lhs := m.pop()
			m.push(compare(in.code, lhs, rhs))
		}
	}
	return sig{}
}

func (m *vm) block(in *instr) sig {
	body := in.body
	if in.op == opIf && m.pop() == 0 {
		body = in.other
	}
	hi := len(m.stack)
	for {
		end := m.exec(body)
		if end.kind == sigReturn {
			return end
		}
		if end.kind == sigBr {
			if end.depth != 0 {
				end.depth--
				return end
			}
			var result uint32
			if in.brArity == 1 && len(m.stack) > 0 {
				result = m.pop()
			}
			if hi <= len(m.stack) {
				m.stack = m.stack[:hi]
			}
			if in.brArity == 1 {
				m.push(result)
			}
			if in.loop {
				continue
			}
			return sig{}
		}
		return sig{}
	}
}

func (m *vm) call(idx uint32) sig {
	if int(idx) >= len(m.inst.funcs) {
		return sig{kind: sigReturn}
	}
	fn := m.inst.funcs[idx]
	locals := make([]uint32, fn.params+fn.extra)
	for i := fn.params - 1; i >= 0; i-- {
		locals[i] = m.pop()
	}
	saved := m.locals
	m.locals = locals
	end := m.exec(fn.body)
	m.locals = saved
	if end.kind == sigBr {
		return sig{kind: sigReturn}
	}
	return sig{}
}

func (m *vm) push(v uint32) { m.stack = append(m.stack, v) }

func (m *vm) pop() uint32 {
	v := m.stack[len(m.stack)-1]
	m.stack = m.stack[:len(m.stack)-1]
	return v
}

func (m *vm) top() uint32 { return m.stack[len(m.stack)-1] }

func (m *vm) load(addr uint32, w width) uint32 {
	mem := m.inst.mem
	switch w {
	case w8u:
		if int(addr) >= len(mem) {
			return 0
		}
		return uint32(mem[addr])
	case w8s:
		if int(addr) >= len(mem) {
			return 0
		}
		return uint32(int32(int8(mem[addr])))
	case w16u:
		if int(addr)+2 > len(mem) {
			return 0
		}
		return uint32(binary.LittleEndian.Uint16(mem[addr:]))
	case w16s:
		if int(addr)+2 > len(mem) {
			return 0
		}
		return uint32(int32(int16(binary.LittleEndian.Uint16(mem[addr:]))))
	default:
		if int(addr)+4 > len(mem) {
			return 0
		}
		return binary.LittleEndian.Uint32(mem[addr:])
	}
}

func (m *vm) store(addr, value uint32, w width) {
	mem := m.inst.mem
	switch w {
	case w8u, w8s:
		if int(addr) >= len(mem) {
			return
		}
		mem[addr] = byte(value)
	case w16u, w16s:
		if int(addr)+2 > len(mem) {
			return
		}
		binary.LittleEndian.PutUint16(mem[addr:], uint16(value))
	default:
		if int(addr)+4 > len(mem) {
			return
		}
		binary.LittleEndian.PutUint32(mem[addr:], value)
	}
}

func (m *vm) grow(pages uint32) uint32 {
	old := uint32(len(m.inst.mem) / pageSize)
	next := old + pages
	if pages > 64 || (m.inst.memMax > 0 && int(next) > m.inst.memMax) || next > 64 {
		return 0xffffffff
	}
	mem := make([]byte, int(next)*pageSize)
	copy(mem, m.inst.mem)
	m.inst.mem = mem
	return old
}

func unary(code byte, v uint32) uint32 {
	switch code {
	case 0x45:
		if v == 0 {
			return 1
		}
		return 0
	case 0x67:
		return clz(v)
	case 0x68:
		return ctz(v)
	case 0x69:
		return popcnt(v)
	default:
		return 0
	}
}

func binaryOp(code byte, lhs, rhs uint32) uint32 {
	switch code {
	case 0x6a:
		return lhs + rhs
	case 0x6b:
		return lhs - rhs
	case 0x6c:
		return lhs * rhs
	case 0x6d:
		if rhs == 0 || (int32(lhs) == -2147483648 && int32(rhs) == -1) {
			return 0
		}
		return uint32(int32(lhs) / int32(rhs))
	case 0x6e:
		if rhs == 0 {
			return 0
		}
		return lhs / rhs
	case 0x6f:
		if rhs == 0 || (int32(lhs) == -2147483648 && int32(rhs) == -1) {
			return 0
		}
		return uint32(int32(lhs) % int32(rhs))
	case 0x70:
		if rhs == 0 {
			return 0
		}
		return lhs % rhs
	case 0x71:
		return lhs & rhs
	case 0x72:
		return lhs | rhs
	case 0x73:
		return lhs ^ rhs
	case 0x74:
		return lhs << (rhs & 31)
	case 0x75:
		return uint32(int32(lhs) >> (rhs & 31))
	case 0x76:
		return lhs >> (rhs & 31)
	case 0x77:
		n := rhs & 31
		if n == 0 {
			return lhs
		}
		return (lhs << n) | (lhs >> (32 - n))
	case 0x78:
		n := rhs & 31
		if n == 0 {
			return lhs
		}
		return (lhs >> n) | (lhs << (32 - n))
	default:
		return 0
	}
}

func compare(code byte, lhs, rhs uint32) uint32 {
	ok := false
	switch code {
	case 0x46:
		ok = lhs == rhs
	case 0x47:
		ok = lhs != rhs
	case 0x48:
		ok = int32(lhs) < int32(rhs)
	case 0x49:
		ok = lhs < rhs
	case 0x4a:
		ok = int32(lhs) > int32(rhs)
	case 0x4b:
		ok = lhs > rhs
	case 0x4c:
		ok = int32(lhs) <= int32(rhs)
	case 0x4d:
		ok = lhs <= rhs
	case 0x4e:
		ok = int32(lhs) >= int32(rhs)
	case 0x4f:
		ok = lhs >= rhs
	}
	if ok {
		return 1
	}
	return 0
}

func clz(v uint32) uint32 {
	if v == 0 {
		return 32
	}
	n := uint32(0)
	for v&0x80000000 == 0 {
		v <<= 1
		n++
	}
	return n
}

func ctz(v uint32) uint32 {
	if v == 0 {
		return 32
	}
	n := uint32(0)
	for v&1 == 0 {
		v >>= 1
		n++
	}
	return n
}

func popcnt(v uint32) uint32 {
	n := uint32(0)
	for v != 0 {
		n += v & 1
		v >>= 1
	}
	return n
}

func (r *reader) u8() (byte, error) {
	if r.i >= len(r.b) {
		return 0, fmt.Errorf("バイトが足りない")
	}
	v := r.b[r.i]
	r.i++
	return v, nil
}

func (r *reader) u32() (uint32, error) {
	var result uint32
	var shift uint
	for {
		b, err := r.u8()
		if err != nil {
			return 0, err
		}
		result |= uint32(b&0x7f) << shift
		if b&0x80 == 0 {
			return result, nil
		}
		shift += 7
		if shift > 28 {
			return 0, fmt.Errorf("LEB128 が長すぎる")
		}
	}
}

func (r *reader) s32() (int32, error) {
	var result uint32
	var shift uint
	var b byte
	for {
		next, err := r.u8()
		if err != nil {
			return 0, err
		}
		b = next
		result |= uint32(b&0x7f) << shift
		shift += 7
		if b&0x80 == 0 {
			break
		}
		if shift > 31 {
			return 0, fmt.Errorf("符号付き LEB128 が長すぎる")
		}
	}
	if shift < 32 && b&0x40 != 0 {
		result |= ^uint32(0) << shift
	}
	return int32(result), nil
}

func (r *reader) name() (string, error) {
	n, err := r.u32()
	if err != nil {
		return "", err
	}
	raw, err := r.take(int(n))
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

func (r *reader) take(n int) ([]byte, error) {
	if n < 0 || r.i+n > len(r.b) {
		return nil, fmt.Errorf("バイト列がはみ出している")
	}
	raw := r.b[r.i : r.i+n]
	r.i += n
	return raw, nil
}

func (r *reader) skip(n int) error {
	_, err := r.take(n)
	return err
}
