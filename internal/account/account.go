package account

import "mmm-revived/internal/model"

type Account struct {
	Retail         bool                                      `json:"retail"`
	GM             model.GMState                             `json:"gm"`
	System         model.SystemState                         `json:"system"`
	Events         model.EventProgress                       `json:"events"`
	Draw           model.DrawState                           `json:"draw"`
	RogueProgress  model.RogueProgress                       `json:"rogue_progress"`
	Journey        model.JourneyState                        `json:"journey"`
	Network        model.NetworkState                        `json:"network"`
	Dispatch       model.DispatchState                       `json:"dispatch"`
	Energy         model.PlayerEnergy                        `json:"energy_state"`
	ShopPurchases  map[model.ShopID]model.ShopPurchase       `json:"shop_purchases"`
	CraftRecipes   []model.CraftID                           `json:"craft_recipes"`
	NextInstance   int64                                     `json:"next_instance"`
	ArtifactCopies int                                       `json:"artifact_copies"`
	CurrentStory   model.MissionID                           `json:"current_story"`
	CurrentMemento model.MissionID                           `json:"current_memento"`
	Profile        model.ProfileCustomization                `json:"profile"`
	HelperSettings HelperSettings                            `json:"helper_settings,omitempty"`
	Lineups        map[int]model.Lineup                      `json:"lineups"`
	Experience     int64                                     `json:"experience"`
	RewardedLevels []int                                     `json:"rewarded_levels"`
	Chapters       map[model.ChapterID]model.ChapterProgress `json:"chapters"`
	Activities     map[model.ActivityID]model.PVPActivity    `json:"activities"`
	TeamBreak      int                                       `json:"team_break"`
	UID            model.UserID                              `json:"uid"`
	Name           string                                    `json:"name"`
	TeamName       string                                    `json:"team_name"`
	CreatedAt      int64                                     `json:"created_at"`
	LastLogin      int64                                     `json:"last_login"`
	PreviousLogin  int64                                     `json:"-"`
	Level          int                                       `json:"level"`
	Items          map[model.ItemID]int64                    `json:"items"`
	Heroes         map[model.HeroID]model.Hero               `json:"heroes"`
	Guides         map[string]string                         `json:"guides"`
	Missions       map[model.MissionID]model.Mission         `json:"missions"`
	Passed         map[model.MissionID]model.MissionProgress `json:"passed"`
	CurrentMission model.MissionID                           `json:"current_mission"`
	Tasks          map[model.TaskID]model.Task               `json:"tasks"`
	Highlights     map[model.InstanceID]model.Highlight      `json:"highlights"`
	Artifacts      map[model.InstanceID]model.Artifact       `json:"artifacts"`
	LimitedHeroes  map[model.ItemID][]model.HeroID           `json:"limited_heroes"`
	DayScheme      int                                       `json:"day_scheme"`
	Manual         model.ManualState                         `json:"manual"`
	Calendar       CalendarState                             `json:"calendar"`
	TimedCosmetics map[model.ItemID]int64                    `json:"timed_cosmetics,omitempty"`
	RogueTriggers  []model.RogueTrigger                      `json:"-"`
}
