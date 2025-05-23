package middleware

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
	"homeApplications/models"
	"log"
	"net/http"
	"strings"
)

const ContentTypeJSON = "application/json"

var (
	dbPool *pgxpool.Pool
)

func SetDBConnection(pool *pgxpool.Pool) {
	dbPool = pool
}

func JSONMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r)
		if w.Header().Get("Content-Type") == "" {
			w.Header().Set("Content-Type", ContentTypeJSON)
		}
	})
}

func CorsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		EnableCors(&w)
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func EnableCors(w *http.ResponseWriter) {
	(*w).Header().Set("Access-Control-Allow-Origin", "*")
	(*w).Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS, PUT, DELETE, PATCH")
	(*w).Header().Set("Access-Control-Allow-Headers", "Content-Type, Access-Control-Allow-Headers, Authorization, X-Requested-With")
}

func HashPassword(password string) (string, error) {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hashedPassword), nil
}

func CheckPassword(hashedPassword, plainPassword string) error {
	return bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(plainPassword))
}

func AuthenticateUser(r *http.Request) (models.AppUser, error) {
	authHeader := r.Header.Get("Authorization")
	errUser := models.AppUser{}
	if !strings.HasPrefix(authHeader, "Basic ") {
		return errUser, errors.New("invalid authorization header")
	}

	encodedCredential := strings.TrimPrefix(authHeader, "Basic ")
	decodedBytes, err := base64.StdEncoding.DecodeString(encodedCredential)
	if err != nil {
		return errUser, errors.New("failed to decode authorization header")
	}

	credentials := strings.SplitN(string(decodedBytes), ":", 2)
	if len(credentials) != 2 {
		return errUser, errors.New("invalid authorization header format")
	}

	username, password := credentials[0], credentials[1]

	var user models.AppUser
	var hashedPassword string
	err = dbPool.QueryRow(context.Background(), "SELECT id, name, access_level, password FROM users WHERE name=$1", username).
		Scan(&user.ID, &user.Name, &user.Access, &hashedPassword)
	invalidUsernameOrPassword := "invalid username or password"
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return errUser, errors.New(invalidUsernameOrPassword)
		}
		return errUser, err
	}
	// Compare the provided password with the hashed password
	if err := CheckPassword(hashedPassword, password); err != nil {
		return errUser, errors.New(invalidUsernameOrPassword)
	}

	return user, nil
}

// CheckAuthorization validates the user's authorization and access level.
func CheckAuthorization(r *http.Request, requiredAccess models.AccessLevel) (*models.AppUser, error) {
	user, err := AuthenticateUser(r)
	if err != nil {
		log.Println("error: " + err.Error() + " in CheckAuthorization")
		return nil, fmt.Errorf("unauthorized: %w", err)
	}

	if user.Access != requiredAccess {
		log.Println(fmt.Sprintf("user '%s' with access level '%s' is not authorized for this action", user.Name, user.Access))
		return nil, fmt.Errorf("forbidden: user does not have the required access level")
	}

	return &user, nil
}

func HandleError(w http.ResponseWriter, err error) {
	switch err.Error() {
	case "unauthorized":
		http.Error(w, err.Error(), http.StatusUnauthorized)
	case "forbidden":
		http.Error(w, err.Error(), http.StatusForbidden)
	default:
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
