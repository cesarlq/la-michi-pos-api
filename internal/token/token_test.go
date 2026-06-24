package token

import (
	"testing"
	"time"
)

func TestSignAndParse_RoundTrip(t *testing.T) {
	m := NewManager("secreto-de-prueba")
	branch := "branch-1"
	now := time.Now()

	tok, err := m.Sign("user-1", "Dueño", "dueno@lamichi.com", "owner", &branch, now)
	if err != nil {
		t.Fatalf("Sign falló: %v", err)
	}

	claims, err := m.Parse(tok)
	if err != nil {
		t.Fatalf("Parse falló: %v", err)
	}
	if claims.Subject != "user-1" {
		t.Errorf("Subject = %q, quería user-1", claims.Subject)
	}
	if claims.Role != "owner" {
		t.Errorf("Role = %q, quería owner", claims.Role)
	}
	if claims.BranchID == nil || *claims.BranchID != "branch-1" {
		t.Errorf("BranchID = %v, quería branch-1", claims.BranchID)
	}
}

func TestParse_WrongSecret(t *testing.T) {
	signer := NewManager("secreto-A")
	verifier := NewManager("secreto-B")

	tok, _ := signer.Sign("u", "User", "e@e.com", "owner", nil, time.Now())
	if _, err := verifier.Parse(tok); err == nil {
		t.Fatal("se esperaba error al verificar con otro secreto")
	}
}

func TestParse_Expired(t *testing.T) {
	m := NewManager("secreto")
	past := time.Now().Add(-9 * time.Hour) // ttl es 8h → ya expiró

	tok, _ := m.Sign("u", "User", "e@e.com", "owner", nil, past)
	if _, err := m.Parse(tok); err == nil {
		t.Fatal("se esperaba error por token expirado")
	}
}
