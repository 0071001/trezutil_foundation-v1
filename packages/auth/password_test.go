package auth

import (
	"crypto/pbkdf2"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"strings"
	"testing"
)

func TestPasswordHashRoundTrip(t *testing.T) {
	hash, err := HashPassword("Correct-Horse-9")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(hash, "pbkdf2-sha256$600000$") {
		t.Fatalf("the hash must carry its scheme and cost: %q", hash)
	}
	if strings.Contains(hash, "Correct-Horse-9") {
		t.Fatal("the hash must not contain the password")
	}
	if !VerifyPassword(hash, "Correct-Horse-9") {
		t.Fatal("the right password must verify")
	}
	for _, wrong := range []string{"", "correct-horse-9", "Correct-Horse-9 ", "Correct-Horse-8"} {
		if VerifyPassword(hash, wrong) {
			t.Fatalf("%q must not verify", wrong)
		}
	}
}

func TestPasswordHashesAreSalted(t *testing.T) {
	a, _ := HashPassword("Same-Password-1")
	b, _ := HashPassword("Same-Password-1")
	if a == b {
		t.Fatal("two hashes of the same password must differ (random salt)")
	}
}

func TestMalformedHashNeverVerifies(t *testing.T) {
	for _, bad := range []string{"", "x", "pbkdf2-sha256$600000$$", "pbkdf2-sha256$0$AA$AA", "scrypt$1$AA$AA", "pbkdf2-sha256$abc$AA$AA", "pbkdf2-sha256$600000$!!$!!"} {
		if VerifyPassword(bad, "anything") {
			t.Fatalf("malformed hash %q verified", bad)
		}
	}
}

func TestAHashMadeWithFewerIterationsStillVerifies(t *testing.T) {
	// The cost is stored in the hash, so raising it later does not lock anyone out.
	salt := []byte("0123456789abcdef")
	key, err := pbkdf2.Key(sha256.New, "Older-Hash-1", salt, 1000, 32)
	if err != nil {
		t.Fatal(err)
	}
	enc := base64.RawStdEncoding
	old := "pbkdf2-sha256$1000$" + enc.EncodeToString(salt) + "$" + enc.EncodeToString(key)
	if !VerifyPassword(old, "Older-Hash-1") || VerifyPassword(old, "Older-Hash-2") {
		t.Fatal("a hash with a lower stored cost must still verify correctly")
	}
}

func TestValidatePassword(t *testing.T) {
	const email = "guilbertmkap@gmail.com"
	for _, p := range []string{"Kap0071001", "Une-Phrase-Longue-2026", "abcDEF1234", "tout en minuscule 12345"} {
		if err := ValidatePassword(p, email); err != nil {
			t.Errorf("%q should be accepted: %v", p, err)
		}
	}
	refused := []struct{ password, reason string }{
		{"Short1A", "at least 10"},
		{strings.Repeat("aB1", 50), "at most 128"},
		{"alllowercaseletters", "three of"},
		{"ALLUPPER1234", "three of"},
		{"guilbertmkap-Secret1", "email"},
		{"Trezutil", "at least 10"},
	}
	for _, c := range refused {
		err := ValidatePassword(c.password, email)
		if err == nil || !errors.Is(err, ErrWeakPassword) || !strings.Contains(err.Error(), c.reason) {
			t.Errorf("%q: error = %v, want a weak-password error mentioning %q", c.password, err, c.reason)
		}
	}
}

func TestBurnPasswordCheckDoesNotPanic(t *testing.T) {
	BurnPasswordCheck("anything")
	BurnPasswordCheck("")
}
