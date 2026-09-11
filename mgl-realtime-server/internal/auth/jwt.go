package auth

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// UserClaims is the client JWT payload (spec §21).
type UserClaims struct {
	App      string `json:"app"`
	DeviceID string `json:"device_id,omitempty"`
	jwt.RegisteredClaims
}

type JWTValidator struct {
	secret []byte
	issuer string
}

func NewJWTValidator(secret, issuer string) *JWTValidator {
	return &JWTValidator{secret: []byte(secret), issuer: issuer}
}

func (v *JWTValidator) Validate(tokenString string) (*UserClaims, error) {
	if tokenString == "" {
		return nil, fmt.Errorf("empty token")
	}
	token, err := jwt.ParseWithClaims(tokenString, &UserClaims{}, func(t *jwt.Token) (any, error) {
		if t.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return v.secret, nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*UserClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}
	if claims.Subject == "" {
		return nil, fmt.Errorf("missing sub")
	}
	if v.issuer != "" && claims.Issuer != "" && claims.Issuer != v.issuer {
		return nil, fmt.Errorf("invalid issuer")
	}
	return claims, nil
}

// IssueDevToken issues a short-lived HS256 JWT (development / tests).
func (v *JWTValidator) IssueDevToken(userID, appID, deviceID string, ttl time.Duration) (string, error) {
	now := time.Now()
	claims := UserClaims{
		App:      appID,
		DeviceID: deviceID,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			Issuer:    v.issuer,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(v.secret)
}

// CallClaims is a short-lived call / SFU token (spec §23).
type CallClaims struct {
	CallID string `json:"call_id"`
	UserID string `json:"user_id"`
	RoomID string `json:"room_id"`
	Role   string `json:"role"`
	App    string `json:"app"`
	jwt.RegisteredClaims
}

func (v *JWTValidator) IssueCallToken(callID, userID, roomID, role, appID string, ttl time.Duration) (string, error) {
	now := time.Now()
	claims := CallClaims{
		CallID: callID,
		UserID: userID,
		RoomID: roomID,
		Role:   role,
		App:    appID,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			Issuer:    v.issuer,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(v.secret)
}

func (v *JWTValidator) ValidateCallToken(tokenString string) (*CallClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &CallClaims{}, func(t *jwt.Token) (any, error) {
		if t.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return v.secret, nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*CallClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}
	if claims.CallID == "" || claims.UserID == "" {
		return nil, fmt.Errorf("incomplete call token")
	}
	return claims, nil
}
