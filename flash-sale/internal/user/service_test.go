package user

import (
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestPasswordHashing(t *testing.T) {
	password := "mysecretpassword"
	hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("GenerateFromPassword error: %v", err)
	}

	if err := bcrypt.CompareHashAndPassword(hashed, []byte(password)); err != nil {
		t.Error("CompareHashAndPassword failed for correct password")
	}

	if err := bcrypt.CompareHashAndPassword(hashed, []byte("wrongpassword")); err == nil {
		t.Error("CompareHashAndPassword should fail for wrong password")
	}
}
