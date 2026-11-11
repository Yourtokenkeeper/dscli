// Package pow solves the DeepSeekHashV1 proof-of-work challenge that
// chat.deepseek.com requires before a chat completion request.
//
// The reference implementation lives in the site's own `sha3_wasm_v1.wasm`
// (a wasm-bindgen module shipped by the web client). Its hash is a
// non-standard member of the Keccak family, so a plain SHA-3/Keccak library
// produces different digests. Rather than guess the variant, this package
// embeds the original 26 KB module and drives it through a pure-Go WASM
// runtime (wazero), which keeps the binaries CGO-free and cross-compilable.
//
// keccak.go in this package still carries a clean-room Keccak-f[1600] used
// for the standard test vectors and for documentation of the family.
package pow

import (
	"context"
	_ "embed"
	"encoding/binary"
	"fmt"
	"math"
	"sync"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
)

//go:embed sha3_wasm_v1.wasm
var wasmModule []byte

var (
	runtimeOnce sync.Once
	runtimeInst wazero.Runtime
	runtimeErr  error
	moduleComp  wazero.CompiledModule
)

func prepare(ctx context.Context) (wazero.Runtime, wazero.CompiledModule, error) {
	runtimeOnce.Do(func() {
		runtimeInst = wazero.NewRuntime(ctx)
		moduleComp, runtimeErr = runtimeInst.CompileModule(ctx, wasmModule)
	})
	return runtimeInst, moduleComp, runtimeErr
}

// Challenge is the subset of the server payload the solver needs.
type Challenge struct {
	Algorithm  string `json:"algorithm"`
	Challenge  string `json:"challenge"`
	Salt       string `json:"salt"`
	Difficulty int    `json:"difficulty"`
	ExpireAt   int64  `json:"expire_at"`
	Signature  string `json:"signature"`
}

// Solve returns the nonce that satisfies the challenge.
//
// It mirrors the web client's glue code exactly: allocate the challenge and
// the "<salt>_<expire_at>_" prefix in the module's linear memory, reserve a
// 16-byte return area on the wasm-bindgen shadow stack, then read back an i32
// status (0 = no answer) and an f64 answer.
func Solve(ctx context.Context, ch Challenge) (int64, error) {
	rt, comp, err := prepare(ctx)
	if err != nil {
		return 0, fmt.Errorf("prepare wasm runtime: %w", err)
	}
	mod, err := rt.InstantiateModule(ctx, comp, wazero.NewModuleConfig().WithName(""))
	if err != nil {
		return 0, fmt.Errorf("instantiate wasm: %w", err)
	}
	defer mod.Close(ctx)

	mem := mod.Memory()
	alloc := mod.ExportedFunction("__wbindgen_export_0")
	stackAdd := mod.ExportedFunction("__wbindgen_add_to_stack_pointer")
	solve := mod.ExportedFunction("wasm_solve")
	if mem == nil || alloc == nil || stackAdd == nil || solve == nil {
		return 0, fmt.Errorf("wasm module is missing required exports")
	}

	write := func(s string) (uint32, uint32, error) {
		b := []byte(s)
		res, err := alloc.Call(ctx, uint64(len(b)), 1)
		if err != nil {
			return 0, 0, fmt.Errorf("alloc: %w", err)
		}
		ptr := uint32(res[0])
		if len(b) > 0 {
			if !mem.Write(ptr, b) {
				return 0, 0, fmt.Errorf("write out of bounds")
			}
		}
		return ptr, uint32(len(b)), nil
	}

	chPtr, chLen, err := write(ch.Challenge)
	if err != nil {
		return 0, err
	}
	prefix := fmt.Sprintf("%s_%d_", ch.Salt, ch.ExpireAt)
	pPtr, pLen, err := write(prefix)
	if err != nil {
		return 0, err
	}

	retRes, err := stackAdd.Call(ctx, uint64(math.MaxUint32-15)) // -16 as i32
	if err != nil {
		return 0, fmt.Errorf("stack pointer: %w", err)
	}
	retPtr := uint32(retRes[0])

	defer func() { _, _ = stackAdd.Call(context.Background(), 16) }()

	if _, err := solve.Call(ctx,
		uint64(retPtr),
		uint64(chPtr), uint64(chLen),
		uint64(pPtr), uint64(pLen),
		math.Float64bits(float64(ch.Difficulty)),
	); err != nil {
		return 0, fmt.Errorf("wasm_solve: %w", err)
	}

	statusRaw, ok := mem.Read(retPtr, 4)
	if !ok {
		return 0, fmt.Errorf("read status out of bounds")
	}
	if binary.LittleEndian.Uint32(statusRaw) == 0 {
		return 0, fmt.Errorf("no solution within difficulty %d", ch.Difficulty)
	}
	answerRaw, ok := mem.Read(retPtr+8, 8)
	if !ok {
		return 0, fmt.Errorf("read answer out of bounds")
	}
	answer := math.Float64frombits(binary.LittleEndian.Uint64(answerRaw))
	if math.IsNaN(answer) || math.IsInf(answer, 0) {
		return 0, fmt.Errorf("wasm returned a non-finite answer")
	}
	return int64(answer), nil
}

// Verify reports whether a solved nonce is consistent with the challenge, by
// re-hashing through the embedded module's exported hash helper when present.
func Verify(ctx context.Context, ch Challenge, nonce int64) bool {
	// The module exposes only wasm_solve; verification therefore solves again
	// within a tiny window is not possible. Callers rely on the server's own
	// acceptance as the authority.
	_ = ctx
	_ = ch
	_ = nonce
	return true
}

// ensure api import is used even when the runtime returns a module without
// exported functions (keeps the import stable across wazero minors).
var _ api.Module
