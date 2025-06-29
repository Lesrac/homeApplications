package pocketMoney

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"homeApplications/middleware"
	"homeApplications/models"
	pocketMoneyModels "homeApplications/pocketMoney/models"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

var (
	dbPool *pgxpool.Pool
)

func SetDBConnection(pool *pgxpool.Pool) {
	dbPool = pool
}

func CreateAction(w http.ResponseWriter, r *http.Request) {
	middleware.EnableCors(&w)
	_, err := middleware.CheckAuthorization(r, models.Admin)
	if err != nil {
		middleware.HandleError(w, err)
		return
	}

	var req pocketMoneyModels.CreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Println(err.Error())
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	var newID int
	err = dbPool.QueryRow(context.Background(), "INSERT INTO pocket_money (receiver_user_id, amount, specific_date) VALUES ($1, $2, $3) RETURNING id",
		req.UserID, req.Amount, req.Date.Format("2006-01-02")).Scan(&newID)
	if err != nil {
		var pgErr *pgconn.PgError
		var errMsg string
		var errCode int
		if errors.As(err, &pgErr) && pgErr.Code == "23505" { // 23505 is the PostgreSQL error code for unique constraint violation
			errMsg = "Entry for the given date already exists"
			errCode = http.StatusConflict
		} else {
			errMsg = "Internal server error"
			errCode = http.StatusInternalServerError
		}
		log.Println("Failed to execute query: " + err.Error())
		http.Error(w, errMsg, errCode)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{"status": "success", "id": newID})
}

func AcknowledgeAction(w http.ResponseWriter, r *http.Request) {
	middleware.EnableCors(&w)
	user, err := middleware.CheckAuthorization(r, models.User)
	if err != nil {
		middleware.HandleError(w, err)
		return
	}

	// Implementation for recording actions
	var req pocketMoneyModels.AcknowledgeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Println(err.Error())
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	log.Println("action acknowledged:", req)

	switch req.Action {
	case pocketMoneyModels.Confirm:
		_, err = dbPool.Exec(context.Background(), "UPDATE pocket_money SET confirmed = TRUE WHERE receiver_user_id=$1 AND id =$2", user.ID, req.EntryID)
	//	actions = append(actions, models.Action{UserID: req.EntryID, Action: string(pocketMoneyModels.Confirm), Timestamp: req.Date})
	case pocketMoneyModels.Refute:
		_, err = dbPool.Exec(context.Background(), "UPDATE pocket_money SET confirmed = FALSE WHERE receiver_user_id=$1 AND id =$2", user.ID, req.EntryID)
	default:
		http.Error(w, "Invalid action", http.StatusBadRequest)
		return
	}
	if err != nil {
		log.Println(err.Error())
		http.Error(w, "DB error", http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}

func GetActions(w http.ResponseWriter, r *http.Request) {
	middleware.EnableCors(&w)
	appUser, err := middleware.AuthenticateUser(r)
	if err != nil {
		middleware.HandleError(w, err)
		return
	}

	userIDStr := strings.TrimPrefix(r.URL.Path, "/pocketMoney/")
	if userIDStr == "" {
		http.Error(w, "User ID is required", http.StatusBadRequest)
		return
	}
	userID, err := strconv.Atoi(userIDStr)
	if err != nil {
		http.Error(w, "Invalid User ID", http.StatusBadRequest)
		return
	}
	if !(appUser.Access == models.Admin || appUser.ID == userID) {
		http.Error(w, "Unauthorized access", http.StatusForbidden)
		return
	}

	rows, err := dbPool.Query(context.Background(), "SELECT * FROM pocket_money WHERE receiver_user_id=$1", userID)
	if err != nil {
		log.Println("Failed to execute query: " + err.Error())
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var pocketMoneyActions []pocketMoneyModels.PocketMoneyEntry
	for rows.Next() {
		var specificDate time.Time
		var pocketMoneyAction pocketMoneyModels.PocketMoneyEntry
		if err := rows.Scan(&pocketMoneyAction.ID, &pocketMoneyAction.UserID, &pocketMoneyAction.Amount, &specificDate, &pocketMoneyAction.Confirmed); err != nil {
			log.Println("Failed to scan row: " + err.Error())
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		pocketMoneyAction.Date = models.DateOnly{Time: specificDate}
		pocketMoneyActions = append(pocketMoneyActions, pocketMoneyAction)
	}

	if len(pocketMoneyActions) == 0 {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("[]"))
		return
	}
	// Implementation for fetching actions
	json.NewEncoder(w).Encode(pocketMoneyActions)
}
