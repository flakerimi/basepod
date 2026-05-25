package crypto

import (
	"bytes"
	"testing"
)

func TestSealOpenRoundtrip(t *testing.T) {
	key, err := NewKey()
	if err != nil {
		t.Fatal(err)
	}
	pt := []byte("hello world secret")
	ct, err := Seal(key, pt)
	if err != nil {
		t.Fatal(err)
	}
	got, err := Open(key, ct)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, pt) {
		t.Fatalf("roundtrip mismatch: %q != %q", got, pt)
	}
}

func TestOpenWrongKeyFails(t *testing.T) {
	k1, _ := NewKey()
	k2, _ := NewKey()
	ct, _ := Seal(k1, []byte("x"))
	if _, err := Open(k2, ct); err == nil {
		t.Fatal("expected error opening with wrong key")
	}
}

func TestPassword(t *testing.T) {
	h, err := HashPassword("hunter2")
	if err != nil {
		t.Fatal(err)
	}
	if !CheckPassword(h, "hunter2") {
		t.Fatal("verify failed")
	}
	if CheckPassword(h, "wrong") {
		t.Fatal("wrong password accepted")
	}
}
