// Package pow implements the DeepSeekHashV1 proof-of-work used by
// chat.deepseek.com before accepting a chat completion request.
//
// The algorithm is the original Keccak-f[1600] permutation (padding 0x01,
// not SHA-3's 0x06) reduced to 23 rounds instead of the standard 24. The
// client must find a nonce in [0, difficulty) such that
//
//	Keccak23("<salt>_<expire_at>_<nonce>") == challenge
//
// The 23-round variant is why a standard Sha3/Keccak library cannot be used
// directly; see docs/protocol.md.
package pow

import "encoding/binary"

// roundConstants are the 24 iota-derived lanes of the Keccak-f[1600] round
// constants. DeepSeekHashV1 consumes the first 23 of them.
var roundConstants = [24]uint64{
	0x0000000000000001, 0x0000000000008082, 0x800000000000808a, 0x8000000080008000,
	0x000000000000808b, 0x0000000080000001, 0x8000000080008081, 0x8000000000008009,
	0x000000000000008a, 0x0000000000000088, 0x0000000080008009, 0x000000008000000a,
	0x000000008000808b, 0x800000000000008b, 0x8000000000008089, 0x8000000000008003,
	0x8000000000008002, 0x8000000000000080, 0x000000000000800a, 0x800000008000000a,
	0x8000000080008081, 0x8000000000008080, 0x0000000080000001, 0x8000000080008008,
}

// rotation offsets, indexed x*5+y
var rotc = [25]uint{
	0, 36, 3, 41, 18,
	1, 44, 10, 45, 2,
	62, 6, 43, 15, 61,
	28, 55, 25, 21, 56,
	27, 20, 39, 8, 14,
}

func rotl(v uint64, n uint) uint64 {
	if n == 0 {
		return v
	}
	return (v << n) | (v >> (64 - n))
}

// keccakF applies rounds of the Keccak-f[1600] permutation in place.
func keccakF(a *[25]uint64, rounds int) {
	var c [5]uint64
	var b [25]uint64
	for round := 0; round < rounds; round++ {
		// theta
		for x := 0; x < 5; x++ {
			c[x] = a[x] ^ a[x+5] ^ a[x+10] ^ a[x+15] ^ a[x+20]
		}
		for x := 0; x < 5; x++ {
			d := c[(x+4)%5] ^ rotl(c[(x+1)%5], 1)
			for y := 0; y < 25; y += 5 {
				a[y+x] ^= d
			}
		}
		// rho + pi
		for x := 0; x < 5; x++ {
			for y := 0; y < 5; y++ {
				b[y+5*((2*x+3*y)%5)] = rotl(a[x+5*y], rotc[x*5+y])
			}
		}
		// chi
		for y := 0; y < 25; y += 5 {
			for x := 0; x < 5; x++ {
				a[y+x] = b[y+x] ^ (^b[y+(x+1)%5] & b[y+(x+2)%5])
			}
		}
		// iota
		a[0] ^= roundConstants[round]
	}
}

// Keccak256 hashes msg with the given number of permutation rounds and the
// original Keccak padding (0x01 ... 0x80). rounds=24 equals standard Keccak-256.
func Keccak256Pad(msg []byte, rounds int, pad byte) [32]byte {
	const rate = 136 // 1088 bits
	var st [25]uint64
	buf := make([]byte, 0, len(msg)+rate)
	buf = append(buf, msg...)
	// pad10*1 with Keccak's 0x01 domain separator
	buf = append(buf, pad)
	for len(buf)%rate != 0 {
		buf = append(buf, 0x00)
	}
	buf[len(buf)-1] |= 0x80

	for off := 0; off < len(buf); off += rate {
		for i := 0; i < rate/8; i++ {
			st[i] ^= binary.LittleEndian.Uint64(buf[off+i*8:])
		}
		keccakF(&st, rounds)
	}

	var out [32]byte
	for i := 0; i < 4; i++ {
		binary.LittleEndian.PutUint64(out[i*8:], st[i])
	}
	return out
}

// Keccak256 is the original Keccak-256 (0x01 padding).
func Keccak256(msg []byte, rounds int) [32]byte { return Keccak256Pad(msg, rounds, 0x01) }

// Keccak256Std is the standard 24-round Keccak-256, kept for test vectors.
func Keccak256Std(msg []byte) [32]byte { return Keccak256(msg, 24) }

// SolveKeccak finds the nonce n in [0, difficulty) with
// Keccak23("<salt>_<expireAt>_<n>") == challenge.
// ok is false when the whole range was exhausted without a hit.
func SolveKeccak(salt string, expireAt int64, difficulty int, challenge []byte, workers int) (nonce int, ok bool) {
	if workers <= 0 {
		workers = 1
	}
	prefix := []byte(salt + "_" + itoa(expireAt) + "_")
	type hit struct {
		n  int
		ok bool
	}
	ch := make(chan hit, workers)
	per := difficulty / workers
	if per == 0 {
		per = 1
	}
	for w := 0; w < workers; w++ {
		lo := w * per
		hi := lo + per
		if w == workers-1 {
			hi = difficulty
		}
		go func(lo, hi int) {
			buf := make([]byte, len(prefix)+20)
			copy(buf, prefix)
			for n := lo; n < hi; n++ {
				b := buf[:len(prefix)]
				b = append(b, []byte(itoa(int64(n)))...)
				h := Keccak256(b, 23)
				if string(h[:]) == string(challenge) {
					ch <- hit{n, true}
					return
				}
			}
			ch <- hit{0, false}
		}(lo, hi)
	}
	for i := 0; i < workers; i++ {
		if r := <-ch; r.ok {
			return r.n, true
		}
	}
	return 0, false
}

func itoa(v int64) string {
	if v == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for v > 0 {
		i--
		b[i] = byte('0' + v%10)
		v /= 10
	}
	return string(b[i:])
}
