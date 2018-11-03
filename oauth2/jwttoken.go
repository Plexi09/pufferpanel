package oauth2

import (
	"encoding/base64"
	"strings"
	"time"

	"github.com/dgrijalva/jwt-go"
	"gopkg.in/oauth2.v3"
	"gopkg.in/oauth2.v3/errors"
	"gopkg.in/oauth2.v3/utils/uuid"
)

// JWTAccessClaims jwt claims
type JWTAccessClaims struct {
	ClientID  string `json:"client_id,omitempty"`
	UserID    string `json:"user_id,omitempty"`
	ExpiredAt int64  `json:"expired_at,omitempty"`
	Scopes    string `json:"scopes,omitempty"`
}

// Valid claims verification
func (a *JWTAccessClaims) Valid() error {
	if time.Unix(a.ExpiredAt, 0).Before(time.Now()) {
		return errors.ErrInvalidAccessToken
	}
	return nil
}

// NewJWTAccessGenerate create to generate the jwt access token instance
func NewJWTAccessGenerate(key []byte, method jwt.SigningMethod) *JWTAccessGenerate {
	return &JWTAccessGenerate{
		SignedKey:    key,
		SignedMethod: method,
	}
}

// JWTAccessGenerate generate the jwt access token
type JWTAccessGenerate struct {
	SignedKey    []byte
	SignedMethod jwt.SigningMethod
}

// Token based on the UUID generated token
func (a *JWTAccessGenerate) Token(data *oauth2.GenerateBasic, isGenRefresh bool) (access, refresh string, err error) {
	claims := &JWTAccessClaims{
		ClientID:  data.Client.GetID(),
		UserID:    data.UserID,
		ExpiredAt: data.TokenInfo.GetAccessCreateAt().Add(data.TokenInfo.GetAccessExpiresIn()).Unix(),
		Scopes:    data.TokenInfo.GetScope(),
	}

	token := jwt.NewWithClaims(a.SignedMethod, claims)
	access, err = token.SignedString(a.SignedKey)
	if err != nil {
		return
	}

	if isGenRefresh {
		refresh = base64.URLEncoding.EncodeToString(uuid.NewSHA1(uuid.Must(uuid.NewRandom()), []byte(access)).Bytes())
		refresh = strings.ToUpper(strings.TrimRight(refresh, "="))
	}

	return
}

func (a *JWTAccessGenerate) Validate(token string) bool {
	return true
}