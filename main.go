package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"homeApplications/health"
	"homeApplications/middleware"
	"homeApplications/models"
	"homeApplications/music"
	"homeApplications/pocketMoney"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
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
	// Create a cancellable context for lifecycle management
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Connect to DB with retries (will keep retrying until context canceled)
	var err error
	dbPool, err = connectWithRetry(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatal(fmt.Fprintf(os.Stderr, "Unable to connect to database: %v\n", err))
	}
	defer dbPool.Close()

	// Start a background DB monitor that keeps an in-memory readiness flag updated.
	middleware.SetDBReady(true)
	go monitorDB(ctx, dbPool)

	middleware.SetDBConnection(dbPool)
	pocketMoney.SetDBConnection(dbPool)

	mux := http.NewServeMux()
	// Unprotected health endpoint (reports DB readiness separately)
	mux.HandleFunc("/health", health.HealthCheck)
	// Wrap DB-backed routes with RequireDB so clients receive 503 while DB is down
	mux.Handle("/login", middleware.RequireDB(http.HandlerFunc(Login)))
	mux.Handle("/users", middleware.RequireDB(http.HandlerFunc(GetUsers)))
	mux.Handle("/user", middleware.RequireDB(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			AddUser(w, r)
		case http.MethodPatch:
			ChangePassword(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})))
	mux.Handle("/pocketMoney/addAction", middleware.RequireDB(http.HandlerFunc(pocketMoney.CreateAction)))
	mux.Handle("/pocketMoney/acknowledgeAction", middleware.RequireDB(http.HandlerFunc(pocketMoney.AcknowledgeAction)))
	mux.Handle("/pocketMoney/", middleware.RequireDB(http.HandlerFunc(pocketMoney.GetActions)))
	// Audio streaming is file-based and does not require DB
	mux.HandleFunc("/audio/", music.StreamMusic)
	mux.Handle("/songs/", middleware.RequireDB(http.HandlerFunc(music.FetchSongTitles)))
	srv := &http.Server{Addr: ":8080", Handler: middleware.CorsMiddleware(middleware.JSONMiddleware(mux))}

	// Start server
	go func() {
		log.Println("Server is starting on port 8080...")
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("ListenAndServe(): %v", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutdown signal received, shutting down server...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("Server Shutdown Failed: %v", err)
	}

	// Cancel background tasks and close DB pool (deferred above will run)
	cancel()
}

// connectWithRetry attempts to create a pgxpool.Pool, retrying with exponential backoff until success
func connectWithRetry(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	backoff := 500 * time.Millisecond
	for attempt := 0; ; attempt++ {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		pool, err := pgxpool.New(ctx, dsn)
		if err == nil {
			// quick health check
			if err = pingDB(ctx, pool); err == nil {
				log.Printf("Connected to DB on attempt %d", attempt+1)
				return pool, nil
			}
			pool.Close()
		}

		log.Printf("DB connect attempt %d failed: %v. Retrying in %s...", attempt+1, err, backoff)
		select {
		case <-time.After(backoff):
			backoff *= 2
			if backoff > 30*time.Second {
				backoff = 30 * time.Second
			}
			continue
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

// pingDB performs a lightweight query to verify connectivity
func pingDB(ctx context.Context, pool *pgxpool.Pool) error {
	if pool == nil {
		return fmt.Errorf("nil pool")
	}
	// Use Exec with a short timeout taken from ctx
	cctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	_, err := pool.Exec(cctx, "SELECT 1")
	return err
}

// monitorDB periodically checks DB connectivity and updates the in-memory readiness flag.
// It intentionally avoids noisy logging; it simply sets the flag so middleware and health can react.
func monitorDB(ctx context.Context, pool *pgxpool.Pool) {
	interval := 10 * time.Second
	if v := os.Getenv("DB_MONITOR_INTERVAL_SECONDS"); v != "" {
		if secs, err := time.ParseDuration(v + "s"); err == nil {
			interval = secs
		}
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	wasUp := true
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := pingDB(ctx, pool); err != nil {
				if wasUp {
					wasUp = false
					middleware.SetDBReady(false)
				}
			} else {
				if !wasUp {
					wasUp = true
					middleware.SetDBReady(true)
				}
			}
		}
	}
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
	rows, err := dbPool.Query(r.Context(), "SELECT id, name, access_level FROM users")
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
	if err = json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Println("error: " + err.Error() + " in ChangePassword. " + user.Name)
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

	_, err = dbPool.Exec(r.Context(), "UPDATE users SET password=$1 WHERE id=$2", hashedPassword, user.ID)
	if err != nil {
		log.Println("Failed to execute query: " + err.Error())
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

func AddUser(w http.ResponseWriter, r *http.Request) {
	// Implementation for recording actions
	_, err := middleware.CheckAuthorization(r, models.Admin)
	if err != nil {
		middleware.HandleError(w, err)
		return
	}

	var req models.AppUser
	if err = json.NewDecoder(r.Body).Decode(&req); err != nil {
		// Potential password leak
		log.Println("error: " + err.Error() + " in AddUser.")
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}
	fmt.Printf("Add user '%s'\n", req.Name)
	hashedPassword, err := middleware.HashPassword(req.Password)
	if err != nil {
		log.Println("Failed to hash password: " + err.Error())
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	_, err = dbPool.Exec(r.Context(), "INSERT INTO users (name, access_level, password) VALUES ($1, $2, $3)", req.Name, req.Access, hashedPassword)
	if err != nil {
		var errMsg string
		var errCode int
		if pgErr, ok := errors.AsType[*pgconn.PgError](err); ok && pgErr.Code == "23505" { // 23505 is the PostgreSQL error code for unique constraint violation
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
