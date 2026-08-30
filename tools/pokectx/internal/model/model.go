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
	Slot       int      `json:"slot"`
	SpeciesID  uint16   `json:"species_id"`
	Species    string   `json:"species"`
	Nickname   string   `json:"nickname"`
	Level      uint8    `json:"level"`
	HP         uint16   `json:"hp"`
	MaxHP      uint16   `json:"max_hp"`
	Experience uint32   `json:"experience"`
	HeldItemID uint16   `json:"held_item_id,omitempty"`
	HeldItem   string   `json:"held_item,omitempty"`
	MoveIDs    []uint16 `json:"move_ids"`
	Moves      []string `json:"moves"`
	MovePP     []int    `json:"move_pp"`
	IsEgg      bool     `json:"is_egg"`
	Status     string   `json:"status"`
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

type BattlePokemon struct {
	Battler    int            `json:"battler"`
	Side       string         `json:"side"`
	SpeciesID  uint16         `json:"species_id"`
	Species    string         `json:"species"`
	Level      uint8          `json:"level"`
	HP         uint16         `json:"hp"`
	MaxHP      uint16         `json:"max_hp"`
	Types      []string       `json:"types"`
	AbilityID  uint8          `json:"ability_id"`
	Ability    string         `json:"ability"`
	MoveIDs    []uint16       `json:"move_ids"`
	Moves      []string       `json:"moves"`
	MovePP     []int          `json:"move_pp"`
	Status     string         `json:"status"`
	StatStages map[string]int `json:"stat_stages"`
}

type Battle struct {
	Kind          string          `json:"kind"`
	CanAttemptRun bool            `json:"can_attempt_run"`
	Pokemon       []BattlePokemon `json:"pokemon"`
}

type StoryState struct {
	BirchLab                 uint16 `json:"birch_lab"`
	PetalburgGym             uint16 `json:"petalburg_gym"`
	RustboroCity             uint16 `json:"rustboro_city"`
	SlateportCity            uint16 `json:"slateport_city"`
	MossdeepSpaceCenter      uint16 `json:"mossdeep_space_center"`
	SootopolisWallace        uint16 `json:"sootopolis_wallace"`
	RescuedBirch             bool   `json:"rescued_birch"`
	AdventureStarted         bool   `json:"adventure_started"`
	DefeatedRivalRoute103    bool   `json:"defeated_rival_route_103"`
	RecoveredDevonGoods      bool   `json:"recovered_devon_goods"`
	ReturnedDevonGoods       bool   `json:"returned_devon_goods"`
	ReceivedPokeNav          bool   `json:"received_pokenav"`
	DeliveredDevonGoods      bool   `json:"delivered_devon_goods"`
	DeliveredStevenLetter    bool   `json:"delivered_steven_letter"`
	ReceivedSurf             bool   `json:"received_surf"`
	ReceivedDive             bool   `json:"received_dive"`
	AquaEscapedInSubmarine   bool   `json:"aqua_escaped_in_submarine"`
	DefeatedMagmaSpaceCenter bool   `json:"defeated_magma_space_center"`
	KyogreEscaped            bool   `json:"kyogre_escaped"`
	WallaceWentToSkyPillar   bool   `json:"wallace_went_to_sky_pillar"`
	MasterBallCollected      bool   `json:"master_ball_collected"`
	RayquazaEncountered      bool   `json:"rayquaza_encountered"`
}

type Snapshot struct {
	SchemaVersion int          `json:"schema_version"`
	GameCode      string       `json:"game_code"`
	ROMCRC32      string       `json:"rom_crc32"`
	PlayerName    string       `json:"player_name"`
	Money         uint32       `json:"money"`
	Location      Location     `json:"location"`
	Team          []Pokemon    `json:"team"`
	Items         []Item       `json:"items"`
	PokeBalls     []Item       `json:"poke_balls"`
	TMsAndHMs     []Item       `json:"tms_and_hms"`
	Berries       []Item       `json:"berries"`
	Badges        []string     `json:"badges"`
	KeyItems      []Item       `json:"key_items"`
	RepelSteps    uint16       `json:"repel_steps"`
	Battle        *Battle      `json:"battle,omitempty"`
	Story         StoryState   `json:"story"`
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
