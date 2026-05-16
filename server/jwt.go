package main

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var secret = []byte("secret-key")
var admin_secret = "secret-admin"

func Token(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Role string `json:"role"`
		Name string `json:"name"`
		Id   int    `json:"id"`
	}
	json.NewDecoder(r.Body).Decode(&body)

	if body.Role == "ADMIN" {
		if body.Name != admin_secret {
			w.WriteHeader(http.StatusForbidden)
			return
		}
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"role": body.Role,
		"name": body.Name,
		"id":   body.Id,
		"exp":  time.Now().Add(time.Minute).Unix(),
	})

	signed, err := token.SignedString(secret)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]string{"token": signed})

}

func ExtractClaims(r *http.Request) jwt.MapClaims {
	tokenStr := r.Header.Get("Authorization")

	token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (any, error) {
		return secret, nil
	})
	if err != nil || !token.Valid {
		// w.WriteHeader(http.StatusUnauthorized)
		return nil
	}

	claims := token.Claims.(jwt.MapClaims)

	return claims
}

func VerifyAdmin(claims jwt.MapClaims) bool {

	role, exists := claims["role"]
	if !exists {
		return false
	}

	if role != "ADMIN" {
		return false
	}

	if claims["name"].(string) != admin_secret {

		return false
	}

	return true
}

func Verify(claims jwt.MapClaims, role string, name string, id string) bool {
	claim_role, exists := claims["role"].(string)
	if !exists {
		return false
	}

	if role != claim_role {
		return false
	}

	claim_name, exists := claims["name"].(string)
	if !exists {
		return false
	}

	if name != claim_name {
		return false
	}

	claim_id, exists := claims["id"].(string)
	if !exists {
		return false
	}

	if claim_id != id {
		return false
	}

	return true
}
