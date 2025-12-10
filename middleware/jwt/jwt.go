package jwt

import (
	"errors"
	"time"

	jwt "github.com/golang-jwt/jwt/v5"
)

var (
	ErrInvalidToken     = errors.New("invalid token")
	ErrExpiredToken     = errors.New("token has expired")
	ErrTokenNotYetValid = errors.New("token not yet valid")
)

// Claims JWT 声明
type Claims struct {
	UserID    string `json:"user_id"`
	UserName  string `json:"user_name"`
	UserEmail string `json:"user_email"`
	jwt.RegisteredClaims
}

type TokenManager struct {
	secret     []byte
	expireDur  time.Duration
	refreshDur time.Duration
}

func NewTokenManager(secret string, expireHours, refreshHours int) *TokenManager {
	return &TokenManager{
		secret:     []byte(secret),
		expireDur:  time.Duration(expireHours) * time.Hour,
		refreshDur: time.Duration(refreshHours) * time.Hour,
	}
}

func (tm *TokenManager) GenerateToken(userID, username, email string) (string, error) {
	now := time.Now()

	claims := Claims{
		UserID:    userID,
		UserName:  username,
		UserEmail: email,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(tm.expireDur)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(tm.secret)
	if err != nil {
		return "", err
	}
	return tokenString, nil
}

func (tm *TokenManager) ParseToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidToken
		}
		return tm.secret, nil
	})
	if err != nil {
		// Check for specific JWT errors
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrExpiredToken
		}
		if errors.Is(err, jwt.ErrTokenNotValidYet) {
			return nil, ErrTokenNotYetValid
		}
		return nil, ErrInvalidToken
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, ErrInvalidToken
	}
	return claims, nil
}

// RefreshToken generates a new token if the current token is within the refresh window
// The refresh window is defined as: token is still valid but will expire within refreshDur
func (tm *TokenManager) RefreshToken(tokenString string) (string, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidToken
		}
		return tm.secret, nil
	}, jwt.WithoutClaimsValidation())
	if err != nil {
		return "", ErrInvalidToken
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || claims.ExpiresAt == nil {
		return "", ErrInvalidToken
	}

	now := time.Now()
	expiryTime := claims.ExpiresAt.Time
	if now.After(expiryTime) {
		// Token is expired, check if it's within the refresh window
		if now.Sub(expiryTime) > tm.refreshDur {
			return "", errors.New("token expired beyond refresh window")
		}
	} else {
		// Token is still valid, check if it's close to expiry (within refresh window)
		if expiryTime.Sub(now) > tm.refreshDur {
			return "", errors.New("token not yet eligible for refresh")
		}
	}
	return tm.GenerateToken(claims.UserID, claims.UserName, claims.UserEmail)
}

func (tm *TokenManager) GetUserIDFromToken(tokenString string) (string, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (any, error) {
		return tm.secret, nil
	}, jwt.WithoutClaimsValidation())
	if err != nil {
		return "", err
	}

	claims, ok := token.Claims.(*Claims)
	if !ok {
		return "", ErrInvalidToken
	}
	return claims.UserID, nil
}
