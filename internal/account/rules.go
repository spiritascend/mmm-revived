package account

import (
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"

	"mmm-revived/internal/json"
	"mmm-revived/internal/model"
)

type Rules struct {
	RetailActivity        RetailActivityRules                             `json:"retail_activity"`
	RetailPVP             RetailPVPRules                                  `json:"retail_pvp"`
	Defaults              GameplayDefaults                                `json:"defaults"`
	GM                    GMRules                                         `json:"gm"`
	System                SystemRules                                     `json:"system"`
	Events                EventRules                                      `json:"events"`
	Draw                  DrawRules                                       `json:"draw"`
	Rogue                 RogueRules                                      `json:"rogue"`
	Journey               JourneyRules                                    `json:"journey"`
	Network               NetworkRules                                    `json:"network"`
	DispatchMissions      map[model.DispatchRuleID]DispatchRule           `json:"dispatch_missions"`
	DispatchMaps          []model.DispatchMapID                           `json:"dispatch_maps"`
	DispatchLimits        map[int]int                                     `json:"dispatch_limits"`
	ArtifactTemplates     map[int]ArtifactTemplate                        `json:"artifact_templates"`
	ArtifactSalvage       map[int]map[int]int64                           `json:"artifact_salvage"`
	HighlightSalvage      map[int]map[int]map[model.ItemID]int64          `json:"highlight_salvage"`
	HighlightAwakeSalvage map[int]map[int]map[model.ItemID]int64          `json:"highlight_awake_salvage"`
	ProficiencyLevels     map[int]int64                                   `json:"proficiency_levels"`
	ProficiencyRewards    map[model.HeroID]map[int]model.RewardID         `json:"proficiency_rewards"`
	ItemRules             map[model.ItemID]ItemRule                       `json:"item_rules"`
	Crafts                map[model.CraftID]CraftRule                     `json:"crafts"`
	Shops                 map[model.ShopID]ShopRule                       `json:"shops"`
	BattleSettings        map[model.BattleID]BattleRule                   `json:"battle_settings"`
	MementoMaps           map[int]map[model.MementoChipID]MementoTileRule `json:"memento_maps"`
	ItemTypes             map[model.ItemID]int                            `json:"item_types"`
	ArtifactLevels        map[int]map[int]LevelRule                       `json:"artifact_levels"`
	Skins                 map[model.SkinID]model.HeroID                   `json:"skins"`
	SkinRarities          map[model.SkinID]int                            `json:"skin_rarities"`
	SkinCollection        SkinCollectionRule                              `json:"skin_collection"`
	Artifacts             map[model.ArtifactID]ArtifactRule               `json:"artifacts"`
	HeroStars             map[int]map[model.ItemID]int64                  `json:"hero_stars"`
	HeroTalents           map[string]TalentRule                           `json:"hero_talents"`
	HeroTalentSets        map[model.HeroID]model.HeroID                   `json:"hero_talent_sets"`
	HeroAwake             map[int]AwakeRule                               `json:"hero_awake"`
	Energy                map[int]EnergyRule                              `json:"energy"`
	Chapters              map[model.ChapterID]ChapterRule                 `json:"chapters"`
	PVPRounds             map[int]PVPRoundRule                            `json:"pvp_rounds"`
	PVPBattleMaps         map[int]model.BattleID                          `json:"pvp_battle_maps"`
	HeroTraits            map[model.HeroID]HeroTraits                     `json:"hero_traits"`
	PVP                   map[model.ActivityID]PVPRule                    `json:"pvp"`
	Missions              map[model.MissionID]MissionRule                 `json:"missions"`
	Nodes                 map[model.NodeID]NodeRule                       `json:"nodes"`
	Levels                map[int]LevelRule                               `json:"levels"`
	Breaks                map[int]map[int]map[model.ItemID]int64          `json:"breaks"`
	HeroBreakSets         map[model.HeroID]int                            `json:"hero_break_sets"`
	Rewards               map[model.RewardID][][2]int64                   `json:"rewards"`
	DropDetails           map[model.RewardID]map[model.ItemID]int         `json:"drop_details"`
	DropTierChances       map[int]int64                                   `json:"drop_tier_chances"`
	Tasks                 map[model.TaskID]TaskRule                       `json:"tasks"`
	BattleModes           map[model.BattleID]int                          `json:"battle_modes"`
	TeamLevels            map[int]TeamLevelRule                           `json:"team_levels"`
	HighlightLevels       map[string]LevelRule                            `json:"highlight_levels"`
	HighlightBreaks       map[int]map[int]map[model.ItemID]int64          `json:"highlight_breaks"`
	HighlightTemplates    map[model.HighlightID]HighlightTemplate         `json:"highlight_templates"`
	Bonds                 map[int]BondRule                                `json:"bonds"`
	ExchangeHeroes        []model.HeroID                                  `json:"exchange_heroes"`
	Manual                ManualRules                                     `json:"manual"`
	Constants             ConstantRules                                   `json:"-"`
	Settings              ServerSettings                                  `json:"-"`
	Economy               Economy                                         `json:"-"`
	AllEvents             EventWindows                                    `json:"-"`
	TeamSize              int                                             `json:"-"`
	EnergyRegen           float64                                         `json:"-"`
	RewardMultiplier      int                                             `json:"-"`
	AllCosmetics          bool                                            `json:"-"`
}

type HeroTraits struct {
	Career int `json:"career"`
	Camp   int `json:"camp"`
	Star   int `json:"star"`
}

type PVPRoundRule struct {
	NewHero         int   `json:"new_hero"`
	NewStar         int   `json:"new_hero_star"`
	NewPool         int   `json:"new_hero_random_num"`
	Upgrades        int   `json:"up_hero_chance"`
	UpgradePool     int   `json:"up_hero_random_num"`
	MaximumUpgrades int   `json:"uptomax_hero_chance"`
	MaximumPool     int   `json:"uptomax_hero_random_num"`
	AIStars         []int `json:"ai_star"`
}

type PVPRule struct {
	Heroes        []model.HeroID `json:"hero"`
	InitialHeroes int            `json:"init_hero_num"`
	InitialStar   int            `json:"init_hero_star"`
	Battles       []int          `json:"battle"`
	Environments  []int          `json:"environment"`
	Total         int            `json:"total"`
	WinReward     model.RewardID `json:"win_point"`
	LossReward    model.RewardID `json:"lose_point"`
	PointTasks    []model.TaskID `json:"point_task"`
	WinTasks      []model.TaskID `json:"max_win_task"`
	DayRecover    int            `json:"day_recover"`
}

type TeamLevelRule struct {
	HeroLimit     int            `json:"hero_limit"`
	Reward        model.RewardID `json:"reward"`
	Break         int            `json:"break"`
	Experience    int64          `json:"experience"`
	EnergyLimit   int64          `json:"energy_limit"`
	EnergyRecover int64          `json:"energy_recover"`
}

type ChapterRule struct {
	Missions []model.MissionID      `json:"missions"`
	Rewards  map[int]model.RewardID `json:"rewards"`
}
type EnergyRule struct {
	Energy int64 `json:"energy"`
	Price  int64 `json:"price"`
}
type ArtifactTemplate struct {
	Attributes map[string][2]int64 `json:"attributes"`
	Weights    map[string]int64    `json:"weights"`
}

type ArtifactRule struct {
	Refinable  bool           `json:"refinable"`
	SubLevels  []int          `json:"sub_levels"`
	Star       int            `json:"star"`
	Templates  []int          `json:"template"`
	Position   int            `json:"position"`
	MaxLevel   int            `json:"max_level"`
	Pools      []model.ItemID `json:"pool"`
	Unique     bool           `json:"unique"`
	Unreleased bool           `json:"unreleased"`
	Item       model.ItemID   `json:"item"`
}
type TalentRule struct {
	Break int                    `json:"break"`
	Cost  map[model.ItemID]int64 `json:"cost"`
}
type AwakeRule struct {
	Prerequisites []int   `json:"prerequisites"`
	Cost          []int64 `json:"cost"`
}
type HighlightTemplate struct {
	AwakeItem  model.ItemID `json:"awake_item"`
	Rarity     int          `json:"rarity"`
	BreakSet   int          `json:"break_set"`
	Classified bool         `json:"classified"`
	Unreleased bool         `json:"unreleased"`
	Item       model.ItemID `json:"item"`
}

type MissionRule struct {
	MaximumSteps    int               `json:"max_steps"`
	DeploymentLevel int               `json:"deployment_level"`
	Switches        map[int]int       `json:"switches"`
	StarConditions  [][2]int          `json:"star_conditions"`
	BossPenalty     int               `json:"boss_penalty"`
	Memento         bool              `json:"memento"`
	Chapter         model.ChapterID   `json:"chapter"`
	FirstNode       model.NodeID      `json:"first_node"`
	Energy          int64             `json:"energy"`
	Level           int               `json:"level"`
	Prerequisites   []model.MissionID `json:"prerequisites"`
	Reward          model.RewardID    `json:"reward"`
}

type NodeRule struct {
	Map    int            `json:"map"`
	Next   model.NodeID   `json:"next"`
	Type   int            `json:"type"`
	Battle model.BattleID `json:"battle"`
}

type LevelRule struct {
	Break      int   `json:"break_level"`
	Experience int64 `json:"experience"`
	Money      int64 `json:"money"`
}

type TaskRule struct {
	AutoClaim    int            `json:"auto_claim"`
	Unreferenced bool           `json:"unreferenced"`
	Daily        bool           `json:"daily"`
	Scope        TaskScope      `json:"scope"`
	Lists        map[int][]int  `json:"param_lists"`
	Texts        map[int]string `json:"param_texts"`
	Achievement  int            `json:"achievement"`
	Points       int64          `json:"points"`
	Targets      []int          `json:"targets"`

	Target int            `json:"target"`
	Type   int            `json:"complete_type"`
	Params []int          `json:"param"`
	Reward model.RewardID `json:"reward"`
}

func LoadRules(path string) (*Rules, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	r := &Rules{}
	if err = json.Unmarshal(data, r); err != nil {
		return nil, err
	}
	dir := filepath.Dir(path)
	for _, extension := range [...]struct {
		name   string
		target any
	}{
		{"gameplay_defaults.json", &r.Defaults}, {"gm_rules.json", &r.GM}, {"system_rules.json", &r.System}, {"progression_events.json", &r.Events}, {"draw_rules.json", &r.Draw}, {"rogue_rules.json", &r.Rogue}, {"journey_rules.json", &r.Journey}, {"network_rules.json", &r.Network}, {"retail_pvp_rules.json", &r.RetailPVP}, {"retail_activity_rules.json", &r.RetailActivity}, {"constant_rules.json", &r.Constants}, {"server_settings.json", &r.Settings},
	} {
		data, err := os.ReadFile(filepath.Join(dir, extension.name))
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if err = json.Unmarshal(data, extension.target); err != nil {
			return nil, fmt.Errorf("%s: %w", extension.name, err)
		}
	}
	if r.Journey.Activities == nil {
		r.Journey.Activities = map[model.ActivityID]JourneyActivityRule{}
	}
	if r.Rewards == nil {
		r.Rewards = map[model.RewardID][][2]int64{}
	}
	for id, rows := range r.RetailActivity.Rewards {
		if _, exists := r.Rewards[id]; !exists {
			r.Rewards[id] = rows
		}
	}
	if r.ItemRules == nil {
		r.ItemRules = map[model.ItemID]ItemRule{}
	}
	for id, row := range r.RetailActivity.Items {
		if _, exists := r.ItemRules[id]; !exists {
			r.ItemRules[id] = row
		}
	}
	for id, row := range r.RetailActivity.Activities {
		if _, exists := r.Journey.Activities[id]; !exists {
			r.Journey.Activities[id] = row
		}
	}
	return r, nil
}

type MementoTileRule struct {
	Type      model.MementoEvent    `json:"type"`
	Initial   bool                  `json:"initial"`
	Next      []model.MementoChipID `json:"next"`
	Moon      int                   `json:"moon"`
	Camp      int                   `json:"camp"`
	Parameter int64                 `json:"param"`
	Targets   []model.MementoChipID `json:"targets"`
	Team      int                   `json:"team"`
}

type BattleRule struct {
	Heroes         int                               `json:"heroes"`
	FixedHeroes    []model.HeroID                    `json:"fixed_heroes"`
	TrialHeroes    []model.HeroID                    `json:"trial_heroes"`
	TemplateHeroes map[model.HeroID]TemplateHeroRule `json:"template_heroes"`
	SkipDeploy     bool                              `json:"skip_deploy"`
	Cards          []int                             `json:"cards"`
	CardCount      int                               `json:"card_count"`
}

type TemplateArtifact struct {
	ID       model.ArtifactID `json:"id"`
	Level    int              `json:"level"`
	Template int              `json:"template"`
	Sub      map[string]int64 `json:"sub"`
}

type TemplateHighlight struct {
	ID    model.HighlightID `json:"id"`
	Level int               `json:"level"`
	Awake int               `json:"awake"`
}

type TemplateHeroState struct {
	Level     int                `json:"level"`
	Break     int                `json:"break"`
	Star      int                `json:"star"`
	Talents   map[int]int        `json:"talents"`
	Awake     []int              `json:"awake"`
	Artifacts []TemplateArtifact `json:"artifacts"`
	Highlight *TemplateHighlight `json:"highlight"`
}

type TemplateHeroRule struct {
	Level   int                       `json:"level"`
	Dynamic bool                      `json:"dynamic"`
	State   TemplateHeroState         `json:"state"`
	Levels  map[int]TemplateHeroState `json:"levels"`
}

func (t TemplateHeroRule) At(teamLevel int) TemplateHeroState {
	if !t.Dynamic || len(t.Levels) == 0 {
		return t.State
	}
	return t.Levels[min(max(t.Level+teamLevel, 1), slices.Max(slices.Collect(maps.Keys(t.Levels))))]
}

type ItemRule struct {
	Type      int                    `json:"type"`
	Rarity    int                    `json:"rarity"`
	Parameter int64                  `json:"parameter"`
	Secondary int64                  `json:"secondary"`
	Refund    map[model.ItemID]int64 `json:"refund"`
	Choices   []int64                `json:"choices"`
	Amounts   []int64                `json:"amounts"`
}

type CraftRule struct {
	Type             string                 `json:"type"`
	Initial          int                    `json:"initial"`
	Materials        map[model.ItemID]int64 `json:"materials"`
	Optional         []model.ItemID         `json:"optional"`
	OptionalCount    int64                  `json:"optional_count"`
	Gold             int64                  `json:"gold"`
	Reward           model.RewardID         `json:"reward"`
	BannedHighlights []int                  `json:"banned_highlights"`
}

type ShopRule struct {
	Currency    model.ItemID   `json:"currency"`
	Prices      []int64        `json:"prices"`
	Reward      model.RewardID `json:"reward"`
	Level       int            `json:"level"`
	Limit       int            `json:"limit"`
	Period      int            `json:"period"`
	Start       int64          `json:"start"`
	End         int64          `json:"end"`
	GuildLevel  int            `json:"guild_level"`
	Recharge    int            `json:"recharge"`
	Predecessor model.ShopID   `json:"pre_id"`
	PreTask     model.TaskID   `json:"pre_task"`
	PeriodHours int64          `json:"period_time"`
}

type DispatchRule struct {
	Heroes       int            `json:"heroes"`
	Camps        []int          `json:"camps"`
	HeroLevel    int            `json:"hero_level"`
	TeamLevel    int            `json:"team_level"`
	MaximumLevel int            `json:"maximum_level"`
	Reward       model.RewardID `json:"reward"`
	ExtraReward  model.RewardID `json:"extra_reward"`
}
