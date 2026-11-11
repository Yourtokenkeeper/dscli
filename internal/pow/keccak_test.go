package pow

import (
	"encoding/hex"
	"testing"
)

func TestKeccakStdVector(t *testing.T) {
	got := Keccak256Std([]byte("abc"))
	want := "4e03657aea45a94fc7d47ba826c8d667c0d1e6e33a64a036ec44f58fa12d6c45"
	if hex.EncodeToString(got[:]) != want {
		t.Fatalf("keccak256(abc)=%x want %s", got, want)
	}
	got = Keccak256Std(nil)
	want = "c5d2460186f7233c927e7db2dcc703c0e500b653ca82273b7bfad8045d85a470"
	if hex.EncodeToString(got[:]) != want {
		t.Fatalf("keccak256(empty)=%x", got)
	}
}
