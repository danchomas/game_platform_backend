package jwt

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

var ErrInvalid = errors.New("invalid token")

type Claims struct {
	Sub	string	`json:"sub"`
	Role	string	`json:"role"`
	Exp	int64	`json:"exp"`
}

var header = base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))

func mac(secret, data string) string {
	m := hmac.New(sha256.New, []byte(secret))
	m.Write([]byte(data))
	return base64.RawURLEncoding.EncodeToString(m.Sum(nil))
}

func Issue(secret, sub, role string, ttl time.Duration) string {
	b, _ := json.Marshal(Claims{Sub: sub, Role: role, Exp: time.Now().Add(ttl).Unix()})
	data := header + "." + base64.RawURLEncoding.EncodeToString(b)
	return data + "." + mac(secret, data)
}

func Parse(secret, token string) (Claims, error) {
	var c Claims
	p := strings.Split(token, ".")
	if len(p) != 3 || p[0] != header {
		return c, ErrInvalid
	}
	if !hmac.Equal([]byte(mac(secret, p[0]+"."+p[1])), []byte(p[2])) {
		return c, ErrInvalid
	}
	raw, err := base64.RawURLEncoding.DecodeString(p[1])
	if err != nil || json.Unmarshal(raw, &c) != nil {
		return c, ErrInvalid
	}
	if c.Exp < time.Now().Unix() || c.Sub == "" {
		return c, ErrInvalid
	}
	return c, nil
}
