package middleware

import (
	"context"
	"encoding/json"
	"net/http"

	ut "github.com/FrankieHealth/be-base/helpers"
)

// Middleware decodes the token
func AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, _ := UserFromHTTPRequestgo(r)
		ctx := r.Context()

		// put it in context
		ctx = context.WithValue(ctx, ut.UserCtxKey, &user)
		r = r.WithContext(ctx)
		next.ServeHTTP(w, r)
	})
}

// TokenFromHTTPRequestgo - get jwt token from request
func UserFromHTTPRequestgo(r *http.Request) (ut.UserClaims, error) {
	reqToken := r.Header.Get("user")
	data := ut.UserClaims{}
	err := json.Unmarshal([]byte(reqToken), &data)

	if err != nil {
		return data, err
	}

	return data, nil
}
