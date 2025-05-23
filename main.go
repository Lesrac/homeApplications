package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"homeApplications/health"
	"homeApplications/middleware"
	"homeApplications/models"
	"homeApplications/music"
	"homeApplications/pocketMoney"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
)

var dbPool *pgxpool.Pool

func main() {
	if os.Getenv("DISABLE_MIGRATIONS") == "" || os.Getenv("DISABLE_MIGRATIONS") == "false" {
		cmd := exec.Command("flyway", "migrate")
		if err := cmd.Run(); err != nil {
			log.Fatalf("Failed to execute Flyway migrations: %v", err)
		}
		log.Println("Database migrations applied successfully.")
	} else {
		log.Println("Migrations are disabled. Remove DISABLE_MIGRATIONS or set it to false to enable.")
	}
	// urlExample := "postgres://homeApp:S3cret@localhost:5432/homeapp_db"
	// should be: os.Getenv("DATABASE_URL")
	var err error
	dbPool, err = pgxpool.New(context.Background(), os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatal(fmt.Fprintf(os.Stderr, "Unable to connect to database: %v\n", err))
	}
	defer dbPool.Close()

	middleware.SetDBConnection(dbPool)
	pocketMoney.SetDBConnection(dbPool)

	mux := http.NewServeMux()
	mux.HandleFunc("/health", health.HealthCheck)
	mux.HandleFunc("/login", Login)
	mux.HandleFunc("/users", GetUsers)
	mux.HandleFunc("/user", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			AddUser(w, r)
		case http.MethodPatch:
			ChangePassword(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})
	mux.HandleFunc("/pocketMoney/addAction", pocketMoney.CreateAction)
	mux.HandleFunc("/pocketMoney/acknowledgeAction", pocketMoney.AcknowledgeAction)
	mux.HandleFunc("/pocketMoney/", pocketMoney.GetActions)
	mux.HandleFunc("/audio/", music.StreamMusic)
	mux.HandleFunc("/songs/", music.FetchSongTitles)
	log.Println("Server is starting on port 8080...")
	log.Fatal(http.ListenAndServe(":8080", middleware.CorsMiddleware(middleware.JSONMiddleware(mux))))
}

func Login(w http.ResponseWriter, r *http.Request) {
	log.Println("login")
	appUser, err := middleware.AuthenticateUser(r)
	if err != nil {
		log.Println("error: " + err.Error() + " in Login")
		middleware.HandleError(w, err)
		return
	}
	appUser.Password = ""
	log.Println("user '" + appUser.Name + "' logged in")
	json.NewEncoder(w).Encode(appUser)
}

func GetUsers(w http.ResponseWriter, r *http.Request) {
	_, err := middleware.CheckAuthorization(r, models.Admin)
	if err != nil {
		middleware.HandleError(w, err)
		return
	}

	// Implementation for fetching users
	rows, err := dbPool.Query(context.Background(), "SELECT id, name, access_level FROM users")
	if err != nil {
		log.Println("Failed to execute query: " + err.Error())
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var users []models.AppUser
	for rows.Next() {
		var user models.AppUser
		if err := rows.Scan(&user.ID, &user.Name, &user.Access); err != nil {
			log.Println("Failed to scan row: " + err.Error())
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		users = append(users, user)
	}

	json.NewEncoder(w).Encode(users)
}

func ChangePassword(w http.ResponseWriter, r *http.Request) {
	println("change password")
	// Implementation for changing password
	user, err := middleware.AuthenticateUser(r)
	if err != nil {
		middleware.HandleError(w, err)
		return
	}

	var req models.AppUser
	body, _ := io.ReadAll(r.Body)
	r.Body = io.NopCloser(bytes.NewBuffer(body))
	if err = json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Println("error: " + err.Error() + " in ChangePassword. " + string(body))
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}
	if user.ID != req.ID {
		log.Println("User '" + user.Name + " (" + string(rune(user.ID)) + ")' tried to change password of user '" + string(rune(req.ID)) + "'")
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	hashedPassword, err := middleware.HashPassword(req.Password)
	if err != nil {
		log.Println("Failed to hash password: " + err.Error())
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	_, err = dbPool.Exec(context.Background(), "UPDATE users SET password=$1 WHERE id=$2", hashedPassword, user.ID)
	if err != nil {
		log.Println("Failed to execute query: " + err.Error())
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

func AddUser(w http.ResponseWriter, r *http.Request) {
	println("add user")
	// Implementation for recording actions
	_, err := middleware.CheckAuthorization(r, models.Admin)
	if err != nil {
		middleware.HandleError(w, err)
		return
	}

	var req models.AppUser
	body, _ := io.ReadAll(r.Body)
	r.Body = io.NopCloser(bytes.NewBuffer(body))
	if err = json.NewDecoder(r.Body).Decode(&req); err != nil {
		// Potential password leak
		log.Println("error: " + err.Error() + " in AddUser. " + string(body))
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}
	hashedPassword, err := middleware.HashPassword(req.Password)
	if err != nil {
		log.Println("Failed to hash password: " + err.Error())
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	_, err = dbPool.Exec(context.Background(), "INSERT INTO users (name, access_level, password) VALUES ($1, $2, $3)", req.Name, req.Access, hashedPassword)
	if err != nil {
		var pgErr *pgconn.PgError
		var errMsg string
		var errCode int
		if errors.As(err, &pgErr) && pgErr.Code == "23505" { // 23505 is the PostgreSQL error code for unique constraint violation
			errMsg = "User already exists"
			errCode = http.StatusConflict
		} else {
			errMsg = "Internal server error"
			errCode = http.StatusInternalServerError
		}
		log.Println("Failed to execute query: " + err.Error())
		http.Error(w, errMsg, errCode)
		return
	}
}
