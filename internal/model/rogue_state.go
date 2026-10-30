package model

type RogueStatus uint8

const (
	RogueIdle RogueStatus = iota
	RogueChooseHero
	RogueShopping
	RogueBattle
	RogueSettle
	RogueAddHero
)

type RoguePVEStatus uint8

const (
	RoguePVEIdle RoguePVEStatus = iota
	RoguePVEChooseHero
	RoguePVEEvent
	RoguePVEBattle
	RoguePVESettle
	RoguePVEPrepare
)

type RogueObject struct {
	ID         int                `json:"id" wire:"id"`
	Level      int                `json:"level" wire:"lv"`
	Index      int                `json:"index" wire:"idx"`
	Relic      InstanceID         `json:"relic" wire:"relic"`
	ExtraLevel int                `json:"extra_level" wire:"extraHeroLv"`
	Discount   int64              `json:"discount" wire:"decCost"`
	Star       int                `json:"star" wire:"star"`
	BreakLevel int                `json:"break_level" wire:"breakLv"`
	Experience int64              `json:"experience" wire:"exp"`
	Talents    map[int]int        `json:"talents" wire:"talents"`
	Awake      []int              `json:"awake" wire:"awakeIds"`
	Artifact   InstanceID         `json:"artifact" wire:"installArti"`
	Highlights map[int]InstanceID `json:"highlights" wire:"installHLight"`
}

type RogueShopEntry struct {
	Object InstanceID `json:"object" wire:"oid"`
	Status int        `json:"status" wire:"st"`
}

type RogueHeroStatus struct {
	Energy    int64 `json:"energy" wire:"energy"`
	HP        int64 `json:"hp" wire:"hp"`
	MaximumHP int64 `json:"maxhp" wire:"maxhp"`
}

type RoguePVEEventInfo struct {
	Token        int64   `json:"token" wire:"token"`
	BuffOptions  []int   `json:"buff_options" wire:"select_buff_opts"`
	ShopOptions  [][]int `json:"shop_options" wire:"rogue_buff_shop_opts"`
	EventOptions [][]int `json:"execute_options" wire:"execute_events_opts"`
}

type RoguePVEBattleInfo struct {
	Token      int64                      `json:"token" wire:"token"`
	Heroes     []HeroID                   `json:"heroes" wire:"heroList"`
	HeroStatus map[HeroID]RogueHeroStatus `json:"hero_status" wire:"hero_stat"`
	Buffs      []int                      `json:"buffs" wire:"gained_rogue_buffs"`
	Event      RoguePVEEventInfo          `json:"event" wire:"event"`
}

type RogueDifficulty struct {
	Wins  int   `json:"wins" wire:"winNum"`
	Score int64 `json:"score" wire:"score"`
}

type RogueHeroRecord struct {
	Hero       HeroID `json:"heroId"`
	Level      int    `json:"heroLv"`
	Relic      int    `json:"relicId,omitempty"`
	RelicLevel int    `json:"relicLv"`
	ExtraLevel int    `json:"extraHeroLv"`
}

type RogueRecord struct {
	Team   []RogueHeroRecord `json:"team"`
	Wins   int               `json:"win"`
	Losses int               `json:"lose"`
	Time   int64             `json:"finishTime"`
}

type RoguePVERecordHero struct {
	ID      HeroID      `json:"id"`
	Level   int         `json:"lv"`
	Star    int         `json:"star"`
	Break   int         `json:"breakLv"`
	Talents map[int]int `json:"talents"`
	Awake   []int       `json:"awakeIds"`
}

type RoguePVERecordTeam struct {
	Heroes []RoguePVERecordHero `json:"hero"`
	Buffs  []int                `json:"rogue_buffs"`
}

type RoguePVERecord struct {
	Team  RoguePVERecordTeam `json:"team"`
	Round int                `json:"round"`
	Time  int64              `json:"finishTime"`
	Level int                `json:"roguePveId"`
}

type RogueRun struct {
	LastDeaths    []HeroID                   `json:"last_deaths"`
	DeathCount    int                        `json:"death_count"`
	RoundTriggers map[InstanceID]int         `json:"round_triggers"`
	Status        RogueStatus                `json:"status"`
	ID            int                        `json:"id"`
	Battles       []BattleID                 `json:"battles"`
	Heroes        map[int]InstanceID         `json:"heroes"`
	Objects       map[InstanceID]RogueObject `json:"objects"`
	Shop          map[int]RogueShopEntry     `json:"shop"`
	HeroPool      []HeroID                   `json:"hero_pool"`
	Banned        []HeroID                   `json:"banned"`
	Coin          int64                      `json:"coin"`
	Life          int                        `json:"life"`
	Index         int                        `json:"index"`
	Wins          int                        `json:"wins"`
	Rerolls       int                        `json:"rerolls"`
	FreeRefresh   int                        `json:"free_refresh"`
	RefreshCount  int                        `json:"refresh_count"`
	SaleCount     int                        `json:"sale_count"`
	MergeCount    int                        `json:"merge_count"`
	NextObject    int64                      `json:"next_object"`
	Started       int64                      `json:"started"`
	Claimed       bool                       `json:"claimed"`
	RankChange    int64                      `json:"rank_change"`
}

type RoguePVERun struct {
	Mana        int64              `json:"mana"`
	Status      RoguePVEStatus     `json:"status"`
	ID          int                `json:"id"`
	Round       int                `json:"round"`
	Battles     []BattleID         `json:"battles"`
	EventGroups []int              `json:"event_groups"`
	EventIndex  int                `json:"event_index"`
	Battle      RoguePVEBattleInfo `json:"battle"`
	Duration    int64              `json:"duration"`
	Started     int64              `json:"started"`
	Claimed     bool               `json:"claimed"`
}

type RogueProgress struct {
	Initialized         bool                    `json:"initialized"`
	Week                int64                   `json:"week"`
	WeeklyScore         int64                   `json:"weekly_score"`
	WeeklyCoin          int64                   `json:"weekly_coin"`
	WeeklyRewards       map[int]int             `json:"weekly_rewards"`
	FirstClaimed        map[int]int             `json:"first_claimed"`
	PVEFirstClaimed     map[int]int             `json:"pve_first_claimed"`
	FirstRewardsDone    bool                    `json:"first_rewards_done"`
	TotalWins           int                     `json:"total_wins"`
	RankScore           int64                   `json:"rank_score"`
	ChestLevel          int                     `json:"chest_level"`
	Run                 RogueRun                `json:"run"`
	Records             []RogueRecord           `json:"records"`
	PVEWeek             int64                   `json:"pve_week"`
	PVEWeeklyScore      int64                   `json:"pve_weekly_score"`
	PVEWeeklyCoin       int64                   `json:"pve_weekly_coin"`
	PVEWeeklyRewards    map[int]int             `json:"pve_weekly_rewards"`
	PVEFirstRewardsDone bool                    `json:"pve_first_rewards_done"`
	MaxDifficulty       int                     `json:"max_difficulty"`
	DifficultyCounted   bool                    `json:"difficulty_counted"`
	Difficulties        map[int]RogueDifficulty `json:"difficulties"`
	PVE                 RoguePVERun             `json:"pve"`
	PVERecords          []RoguePVERecord        `json:"pve_records"`
	FinalWins           int                     `json:"final_wins"`
	FinalWinsCounted    bool                    `json:"final_wins_counted"`
	PVEWins             int                     `json:"pve_wins"`
	PVEWinsCounted      bool                    `json:"pve_wins_counted"`
	RogueID             int                     `json:"-"`
	RankVersion         int                     `json:"-"`
	NewbiePassed        bool                    `json:"-"`
}

type RogueLimitedHero struct {
	ID         HeroID             `wire:"id"`
	Level      int                `wire:"lv"`
	Index      int                `wire:"idx"`
	Star       int                `wire:"star"`
	BreakLevel int                `wire:"breakLv"`
	Talents    map[int]int        `wire:"talents"`
	Awake      []int              `wire:"awakeIds"`
	Artifact   InstanceID         `wire:"installArti"`
	Highlights map[int]InstanceID `wire:"installHighlight"`
	Relic      int                `wire:"relicId"`
	RelicLevel int                `wire:"relicLv"`
}

type RogueEnhancement struct {
	Level int `wire:"rogue_enhance"`
}

type RogueBattleExtra struct {
	SeasonScience   []int                       `wire:"seasonScienceNodeIds"`
	TeamLevel       int                         `wire:"teamLv"`
	EnemyLevel      int                         `wire:"enemy_lv"`
	EnemyTeam       [][]int64                   `wire:"enemy_team"`
	RogueHeroes     map[HeroID]RogueEnhancement `wire:"rogue_hero"`
	RogueBuffs      []int                       `wire:"rogue_buffs"`
	InheritedHeroes map[HeroID]RogueHeroStatus  `wire:"rogue_pve_hero"`
	EnemyBuffs      []int                       `wire:"enemy_buff"`
	Mana            int64                       `wire:"mana"`
	Bonds           map[string]int64            `wire:"bondAttr,omitempty"`
}

type RogueTrigger struct {
	Relic   InstanceID
	Skill   int
	Targets []InstanceID
}
