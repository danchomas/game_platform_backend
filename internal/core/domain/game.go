package domain

import (
	"encoding/json"
	"time"
)

type Game struct {
	ID		string		`json:"id"`
	OwnerID		string		`json:"owner_id"`
	OwnerName	string		`json:"owner_name"`
	Title		string		`json:"title"`
	Genre		string		`json:"genre"`
	Audience	string		`json:"audience"`
	Prompt		string		`json:"prompt"`
	HTML		string		`json:"html,omitempty"`
	Version		int		`json:"version"`
	Published	bool		`json:"published"`
	Plays		int64		`json:"plays"`
	Status		string		`json:"status"`
	StatusDetail	string		`json:"status_detail,omitempty"`
	Spec		json.RawMessage	`json:"spec,omitempty"`
	CreatedAt	time.Time	`json:"created_at"`
	UpdatedAt	time.Time	`json:"updated_at"`
}

type Message struct {
	ID		int64		`json:"id"`
	Role		string		`json:"role"`
	Content		string		`json:"content"`
	Version		*int		`json:"version,omitempty"`
	CreatedAt	time.Time	`json:"created_at"`
}

type VersionInfo struct {
	Version		int		`json:"version"`
	Instruction	string		`json:"instruction"`
	CreatedAt	time.Time	`json:"created_at"`
}

type VersionInput struct {
	GameID		string
	UserText	string
	Instruction	string
	Reply		string
	HTML		string
	Title		string
	Genre		string
	Audience	string
}
