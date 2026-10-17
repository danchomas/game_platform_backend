package auth

import (
	"testing"
	"time"
)

func TestJWT(t *testing.T) {
	tok := Issue("s", "user-1", "manager", time.Hour)
	c, err := Parse("s", tok)
	if err != nil || c.Sub != "user-1" || c.Role != "manager" {
		t.Fatalf("%+v %v", c, err)
	}
	if _, err := Parse("other-secret", tok); err == nil {
		t.Fatal("подпись другим секретом должна быть отклонена")
	}
	if _, err := Parse("s", tok+"x"); err == nil {
		t.Fatal("испорченная подпись должна быть отклонена")
	}
	if _, err := Parse("s", Issue("s", "u", "player", -time.Minute)); err == nil {
		t.Fatal("просроченный токен должен быть отклонён")
	}
}
