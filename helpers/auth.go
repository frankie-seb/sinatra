package base_helpers

import (
	"context"
	"fmt"
	"net/http"

	"github.com/dgrijalva/jwt-go"
	"github.com/gorilla/sessions"
	"github.com/spf13/viper"
	"golang.org/x/crypto/bcrypt"
)

var UserCtxKey = &ContextKey{name: "user"}

type ContextKey struct {
	name string
}

type UserClaims struct {
	UserID     string `json:"user_id"`
	Authorized string `json:"authorized"`
	Role       string `json:"role"`
	UUID       string `json:"uuid"`
	AccountId  string `json:"account_id"`
	jwt.StandardClaims
}

type UserAuth struct {
	UserID       string
	UUID         string
	Roles        []string
	IPAddress    string
	Token        string
	RefreshToken string
	IsAuthorized bool
}

/**
 * Hash the password
 */
func HashPassword(password string) (string, error) {
	pass, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)

	return string(pass), err
}

/**
 * Compare password w/ hash
 */
func ComparePassword(password string, hash string) bool {
	byteHash := []byte(hash)
	bytePassword := []byte(password)
	err := bcrypt.CompareHashAndPassword(byteHash, bytePassword)
	return err == nil
}

/**
 * Decode Token
 */
func DecodeToken(token string, tokenType string) (*jwt.Token, error) {
	secret := viper.GetString("infra.jwt.secret")
	refreshSecret := viper.GetString("infra.jwt.refreshSecret")

	s := secret

	if tokenType == "refreshToken" {
		s = refreshSecret
	}
	return jwt.Parse(token, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}

		return []byte(s), nil
	})
}

/**
 * Get auth from context
 */
func GetAuthFromContext(ctx context.Context) *UserClaims {
	raw := ctx.Value(UserCtxKey)

	if raw == nil {
		return nil
	}

	return raw.(*UserClaims)
}

/**
 * Get userId from context
 */
func GetUserIDFromContext(ctx context.Context) int {
	user := GetAuthFromContext(ctx)
	if user != nil {
		return int(IDToBoiler(user.UserID))
	}
	return 0
}

/**
 * Http Key to extract the HTTP struct
 */
type HTTPKey string

/**
 * HTTP is the struct used to inject the response writer and request http structs.
 */
type HTTP struct {
	W *http.ResponseWriter
	R *http.Request
}

/**
 * Returns the Refresh Token.
 */
func GetSession(ctx context.Context, name string) *sessions.Session {
	store := ctx.Value(HTTPKey("fcare")).(*sessions.CookieStore)
	httpContext := ctx.Value(HTTPKey("http")).(HTTP)

	// Ignore err because a session is always returned even if one doesn't exist
	session, _ := store.Get(httpContext.R, name)

	return session
}

/**
 * Saves the Refresh Token.
 */
func SaveSession(ctx context.Context, session *sessions.Session) error {
	httpContext := ctx.Value(HTTPKey("http")).(HTTP)

	err := session.Save(httpContext.R, *httpContext.W)

	return err
}
