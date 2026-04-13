package health

import (
	"encoding/json"
	"net/http"

	"homeApplications/middleware"
	"log"
)

type HealthStatus struct {
	Status string `json:"status"`
}

func HealthCheck(w http.ResponseWriter, r *http.Request) {
	status := HealthStatus{Status: "OK"}
	if !middleware.IsDBReady() {
		status.Status = "DB_UNAVAILABLE"
		w.WriteHeader(http.StatusServiceUnavailable)
	}
	if err := json.NewEncoder(w).Encode(status); err != nil {
		log.Println("HealthCheck: failed to encode response:", err)
	}
}
