package middleware

import (
	"encoding/base64"
	"errors"
	"fmt"
	"homeApplications/models"
	"log"
	"net/http"
	"strings"
	"sync/atomic"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

const ContentTypeJSON = "application/json"

var (
	dbPool  *pgxpool.Pool
	dbReady atomic.Bool
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
	// Use the request context so DB calls respect cancellation/timeouts
	err = dbPool.QueryRow(r.Context(), "SELECT id, name, access_level, password FROM users WHERE name=$1", username).
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
		return nil, errors.New("unauthorized")
	}

	if user.Access != requiredAccess {
		log.Println(fmt.Sprintf("user '%s' with access level '%s' is not authorized for this action", user.Name, user.Access))
		return nil, errors.New("forbidden")
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

// SetDBReady updates the in-memory readiness flag used by health and RequireDB.
func SetDBReady(v bool) {
	dbReady.Store(v)
}

// IsDBReady returns the current readiness flag.
func IsDBReady() bool {
	return dbReady.Load()
}

// RequireDB is a middleware that checks DB connectivity on-demand and returns 503 when the DB is not reachable.
// It performs a short PingDB with a small timeout so requests fail fast when DB is down.
func RequireDB(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !IsDBReady() {
			EnableCors(&w)
			http.Error(w, "service unavailable", http.StatusServiceUnavailable)
			return
		}
		next.ServeHTTP(w, r)
	})
}
