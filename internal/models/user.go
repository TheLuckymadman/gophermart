package models

import (
	"github.com/golang-jwt/jwt/v5"
)

type Users struct {
	ID       int    `json:"id"`
	Login    string `json:"login"`
	Password string `json:"-"`
	Hash     string `json:"-"`
}

type Claims struct {
	ID       int    `json:"id"`
	Username string `json:"username"`
	jwt.RegisteredClaims
}
