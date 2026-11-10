package auth

import (
	"testing"
	"time"
)

func TestPINHashing(t *testing.T) {
	h, err := HashPIN("2468")
	if err != nil {
		t.Fatal(err)
	}
	h2, _ := HashPIN("2468")
	if h == h2 {
		t.Fatal("salt must differ")
	}
	if !VerifyPIN("2468", h) || VerifyPIN("2469", h) || VerifyPIN("", h) {
		t.Fatal("verify")
	}
	for _, bad := range []string{"", "plain", "pbkdf2-sha256$1$x$y", "pbkdf2-sha256$600000$!!$!!", "md5$1$a$b"} {
		if VerifyPIN("2468", bad) {
			t.Fatalf("malformed hash %q matched", bad)
		}
	}
	for pin, ok := range map[string]bool{"123": false, "1234": true, "    ": false, "correct horse battery": true} {
		if (ValidatePIN(pin) == nil) != ok {
			t.Errorf("ValidatePIN(%q)", pin)
		}
	}
}

func TestThrottle(t *testing.T) {
	clock := time.Unix(1000, 0)
	th := NewThrottle()
	th.now = func() time.Time { return clock }
	for i := 0; i < 5; i++ {
		if d := th.Fail("a"); d != 0 {
			t.Fatalf("attempt %d locked", i+1)
		}
	}
	if d := th.Fail("a"); d != 30*time.Second {
		t.Fatalf("6th failure: %v", d)
	}
	if th.Wait("a") == 0 || th.Wait("b") != 0 {
		t.Fatal("lockout must be per IP")
	}
	clock = clock.Add(31 * time.Second)
	if th.Wait("a") != 0 {
		t.Fatal("lockout should expire")
	}
	if d := th.Fail("a"); d != 60*time.Second {
		t.Fatalf("doubling: %v", d)
	}
	for i := 0; i < 20; i++ {
		th.Fail("a")
	}
	if d := th.Fail("a"); d != 15*time.Minute {
		t.Fatalf("cap: %v", d)
	}
	th.Success("a")
	if th.Wait("a") != 0 {
		t.Fatal("success clears")
	}
}
