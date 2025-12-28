package auth

import (
	"fmt"
	"time"

	"github.com/TheLuckymadman/gophermart/internal/models"
	"github.com/golang-jwt/jwt/v5"
)

func IssueJWT(key []byte, userID int, login string, minExp int) (string, error) {
	expirationTime := time.Now().Add(time.Minute * time.Duration(minExp))

	claims := &models.Claims{
		Username: login,
		ID:       userID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    "gophermart",
			Subject:   "user token",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	fmt.Printf("token %v", token)
	tokenString, err := token.SignedString([]byte(key))
	if err != nil {
		return "", err
	}
	fmt.Printf("tokenString %v", tokenString)
	return tokenString, nil
}

func VerifyJWT(tokenString string, key []byte) (*models.Claims, error) {
	claims := &models.Claims{}

	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		return key, nil
	})
	if err != nil {
		fmt.Printf("%v", err)
		return nil, fmt.Errorf("invalid token: %w", err)
	}

	if !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}
	return claims, err
}
