package main

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var secret = []byte("secret-key")

func Token(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Role string `json:"role"`
		Name string `json:"name"`
	}
	json.NewDecoder(r.Body).Decode(&body)

	if body.Role == "ADMIN" {
		if body.Name != "secret-admin" {
			w.WriteHeader(http.StatusForbidden)
			return
		}
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"role": body.Role,
		"name": body.Name,
		"exp":  time.Now().Add(time.Minute).Unix(),
	})

	signed, err := token.SignedString(secret)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]string{"token": signed})

}

func requireAuth(role string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tokenStr := r.Header.Get("Authorization")

		token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (any, error) {
			return secret, nil
		})
		if err != nil || !token.Valid {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		claims := token.Claims.(jwt.MapClaims)
		if claims["role"] != role {
			w.WriteHeader(http.StatusForbidden)
			return
		}

		next(w, r)
	}
}
