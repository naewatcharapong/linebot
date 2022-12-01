package model

import (
	"time"
)

type User struct {
	Id        string       `json:"userId,omitempty"`
	CreatedAt time.Time `json:"timestamp"`
}
