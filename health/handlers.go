package health

import (
	"encoding/json"
	"net/http"
)

type HealthStatus struct {
	Status string `json:"status"`
}

func HealthCheck(w http.ResponseWriter, r *http.Request) {
	status := HealthStatus{Status: "OK"}
	json.NewEncoder(w).Encode(status)
}
