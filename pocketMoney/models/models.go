package models

import (
	"homeApplications/models"
)

type AcknowledgeAction string

const (
	Confirm AcknowledgeAction = "confirm"
	Refute  AcknowledgeAction = "refute"
)

type AcknowledgeRequest struct {
	EntryID int               `json:"id"`
	Action  AcknowledgeAction `json:"action"`
}

type CreateRequest struct {
	UserID int             `json:"userId"`
	Date   models.DateOnly `json:"date"`
	Amount int             `json:"amount"`
}

type PocketMoneyEntry struct {
	ID        int             `json:"id"`
	UserID    int             `json:"user_id"`
	Amount    int             `json:"amount"`
	Date      models.DateOnly `json:"date"`
	Confirmed bool            `json:"confirmed"`
}
