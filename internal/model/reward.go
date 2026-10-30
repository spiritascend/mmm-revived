package model

type ItemReward struct {
	Item    ItemID
	Count   int64
	Details RewardDetails
}

func (r ItemReward) WireValue() any { return []any{r.Item, r.Count, r.Details} }

type RewardDetails struct {
	Conversion     map[ItemID]int64 `wire:"trans,omitempty"`
	New            bool             `wire:"isNew"`
	Instance       InstanceID       `wire:"uid,omitempty"`
	Unique         InstanceID       `wire:"uniId,omitempty"`
	ID             int32            `wire:"id,omitempty"`
	Level          int              `wire:"lv,omitempty"`
	Template       int              `wire:"temp,omitempty"`
	SubAttributes  map[string]int64 `wire:"subAttr,omitempty"`
	Task           TaskID           `wire:"taskId,omitempty"`
	TrialConverted int              `wire:"trail2official,omitempty"`
	ChallengeStar  int              `wire:"challengeStarShow,omitempty"`
}
