package secrets

import "testing"

func TestPasswordHashAndVerify(t *testing.T) {
	h, err := HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatal(err)
	}
	if !VerifyPassword(h, "correct horse battery staple") {
		t.Fatal("expected password to verify")
	}
	if VerifyPassword(h, "incorrect password") {
		t.Fatal("unexpected password verification")
	}
	if _, err := HashPassword("short"); err == nil {
		t.Fatal("expected short password rejection")
	}
}
