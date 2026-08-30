package model

import "time"

const SchemaVersion = 1

type Evidence struct {
	Source     string    `json:"source"`
	ObservedAt time.Time `json:"observed_at"`
	Freshness  string    `json:"freshness"`
}

type SaveEvidence struct {
	Counter        uint32     `json:"counter"`
	LastSeenAt     *time.Time `json:"last_save_seen_at,omitempty"`
	PlayTime       string     `json:"play_time"`
	FileModifiedAt *time.Time `json:"file_modified_at,omitempty"`
	Source         string     `json:"source"`
}

type Location struct {
	MapGroup int    `json:"map_group"`
	MapNum   int    `json:"map_num"`
	MapID    string `json:"map_id"`
	Name     string `json:"name"`
	X        int    `json:"x"`
	Y        int    `json:"y"`
}

type Pokemon struct {
	Slot      int      `json:"slot"`
	SpeciesID uint16   `json:"species_id"`
	Species   string   `json:"species"`
	Nickname  string   `json:"nickname"`
	Level     uint8    `json:"level"`
	HP        uint16   `json:"hp"`
	MaxHP     uint16   `json:"max_hp"`
	MoveIDs   []uint16 `json:"move_ids"`
	Moves     []string `json:"moves"`
	IsEgg     bool     `json:"is_egg"`
}

type Item struct {
	ID       uint16 `json:"id"`
	Name     string `json:"name"`
	Quantity uint16 `json:"quantity"`
}

type Objective struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Destination string   `json:"destination,omitempty"`
	Evidence    []string `json:"evidence"`
	Confidence  string   `json:"confidence"`
}

type Snapshot struct {
	SchemaVersion int          `json:"schema_version"`
	GameCode      string       `json:"game_code"`
	ROMCRC32      string       `json:"rom_crc32"`
	PlayerName    string       `json:"player_name"`
	Location      Location     `json:"location"`
	Party         []Pokemon    `json:"party"`
	Badges        []string     `json:"badges"`
	KeyItems      []Item       `json:"key_items"`
	Milestones    []string     `json:"milestones"`
	Next          Objective    `json:"next"`
	Save          SaveEvidence `json:"save"`
	Evidence      Evidence     `json:"evidence"`
}

type Check struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Detail string `json:"detail,omitempty"`
	Fix    string `json:"fix,omitempty"`
}

type Doctor struct {
	SchemaVersion int           `json:"schema_version"`
	Status        string        `json:"status"`
	Version       string        `json:"version"`
	Target        string        `json:"target"`
	Checks        []Check       `json:"checks"`
	GameCode      string        `json:"game_code,omitempty"`
	ROMCRC32      string        `json:"rom_crc32,omitempty"`
	ObservedAt    *time.Time    `json:"observed_at,omitempty"`
	Save          *SaveEvidence `json:"save,omitempty"`
}
