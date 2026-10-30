package model

type DrawSetID int32
type DrawStrategyID int32

type DrawPoolState struct {
	Count     int                     `json:"count" wire:"count"`
	TrialTime int64                   `json:"trial_time" wire:"trailTime"`
	Satisfied map[DrawStrategyID]bool `json:"satisfied" wire:"satisfied"`
}

type DrawCounter struct {
	Number  int  `json:"number" wire:"num"`
	Advance int  `json:"advance" wire:"advance"`
	Claimed bool `json:"claimed,omitempty" wire:"-"`
}

type DrawRecord struct {
	Pool DrawPoolID `json:"pool" wire:"drawId"`
	Item ItemID     `json:"item" wire:"itemId"`
	Time int64      `json:"time" wire:"t"`
}

type DrawState struct {
	Pools      map[DrawPoolID]DrawPoolState `json:"pools"`
	Counters   map[int]DrawCounter          `json:"counters"`
	History    map[int64]DrawRecord         `json:"history"`
	NextRecord int64                        `json:"next_record"`
}
