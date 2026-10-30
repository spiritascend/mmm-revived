package account

import (
	"maps"
	"slices"
	"strconv"

	"mmm-revived/internal/calendar"
	"mmm-revived/internal/model"
)

type RogueRules struct {
	Runs            map[int]RogueRunRule              `json:"runs"`
	Shops           map[int]RogueShopRule             `json:"shops"`
	Relics          map[int]RogueRelicRule            `json:"relics"`
	Chests          map[int]RogueChestRule            `json:"chests"`
	ChestDrops      map[model.RewardID]RogueChestDrop `json:"chest_drops"`
	HeroUnlockWins  map[model.HeroID]int              `json:"hero_unlock_wins"`
	HeroNum         int                               `json:"hero_num"`
	PVE             map[int]RoguePVERule              `json:"pve"`
	Events          map[int]RogueEventRule            `json:"events"`
	EventGroups     map[int][]int                     `json:"event_groups"`
	Buffs           map[int]RogueBuffRule             `json:"buffs"`
	FirstRewards    map[int]model.RewardID            `json:"first_rewards"`
	WeekRewards     map[int]model.RewardID            `json:"week_rewards"`
	PVEFirstRewards map[int]model.RewardID            `json:"pve_first_rewards"`
	PVEWeekRewards  map[int]model.RewardID            `json:"pve_week_rewards"`
	Alternations    map[int64]RogueAlternationRule    `json:"alternations"`
	Unlock          map[string]RogueUnlockRule        `json:"unlock"`
	Score           int64                             `json:"score"`
	PVEScore        int64                             `json:"pve_score"`
	RecordLimit     int                               `json:"record_limit"`
	HeroStar        int                               `json:"hero_star"`
	MaxHeroLevel    int                               `json:"max_hero_level"`
	MaxTimeScore    int64                             `json:"max_time_score"`
	RankWinScore    int64                             `json:"rank_win_score"`
	RankLossScore   int64                             `json:"rank_loss_score"`
	RankVersion     int                               `json:"rank_version"`
	NewbiePassed    bool                              `json:"newbie_passed"`
	Currency        model.ItemID                      `json:"currency"`
	PVECurrency     model.ItemID                      `json:"pve_currency"`
	WinReward       model.RewardID                    `json:"win_reward"`
	PVEWinRewards   []model.RewardID                  `json:"pve_win_rewards"`
	RelicPrices     []int64                           `json:"relic_prices"`
	RefreshPrice    int64                             `json:"refresh_price"`
	SalePrice       int64                             `json:"sale_price"`
	Life            int                               `json:"life"`
	HeroLevel       int                               `json:"hero_level"`
	RechooseLimit   int                               `json:"rechoose_limit"`
	MaximumWins     int                               `json:"max_wins"`
}

type RogueRunRule struct {
	BattleSequence []string         `json:"battle_seq"`
	BattlePool     []model.BattleID `json:"battle_pool"`
	HeroPool       [][]model.HeroID `json:"hero_pool"`
	RelicPool      []int            `json:"relic_pool"`
	Shops          []int            `json:"shop_lv_seq"`
	Chests         []int            `json:"chest"`
	Levels         []int            `json:"battle_lv_seq"`
}

type RogueShopRule struct {
	Relics  int   `json:"relic_num"`
	Coin    int64 `json:"relic_coin"`
	AddHero []int `json:"add_hero"`
	Weights []int `json:"weights"`
}

type RogueRelicRule struct {
	Tier       int     `json:"tier"`
	Type       int     `json:"type"`
	HeroLevels []int   `json:"heroLvUp"`
	Skills     [][]int `json:"skill"`
	ShopSkills [][]int `json:"shopSkill"`
}

type RogueChestRule struct {
	Energy  int64                  `json:"energy_cost"`
	Cost    map[model.ItemID]int64 `json:"coin_cost"`
	Level   int                    `json:"limitLv"`
	Battle  model.BattleID         `json:"unlock_battle"`
	Rewards []model.RewardID       `json:"reward"`
}

type RogueChestDrop struct {
	Guaranteed map[model.ItemID]int64 `json:"guaranteed"`
	Extra      map[model.ItemID]int64 `json:"extra"`
}

type RoguePVERule struct {
	Battles        [][]model.BattleID `json:"battle_seq"`
	Levels         []int              `json:"battle_lv_seq"`
	EventGroups    [][]int            `json:"evt_group_seq"`
	EventPool      []int              `json:"evt_group_pool"`
	Buffs          []int              `json:"buff_pool"`
	EnemyBuffs     [][]int            `json:"enemy_buff_seq"`
	Level          int                `json:"unlock_lv"`
	Difficulty     int                `json:"difficulty"`
	OpeningTime    int                `json:"opening_time"`
	Positions      [][]string         `json:"posList"`
	PositionSwitch []int              `json:"posSwitch"`
}

type RogueEventRule struct {
	Action1 int   `json:"action1"`
	Action2 int   `json:"action2"`
	Action3 int   `json:"action3"`
	Param1  []int `json:"param1"`
	Param2  []int `json:"param2"`
	Param3  []int `json:"param3"`
}

type RogueBuffRule struct {
	Tier  int   `json:"tier"`
	Cost  int64 `json:"cost"`
	Buffs []int `json:"buffid"`
}

type RogueAlternationRule struct {
	Type      string                 `json:"type"`
	Start     int64                  `json:"start"`
	CoinLimit int64                  `json:"coin_limit"`
	Levels    []int                  `json:"levels"`
	Rewards   map[int]model.RewardID `json:"rewards"`
}

type RogueUnlockRule struct {
	Level      int `json:"level"`
	LoginDay   int `json:"login_day"`
	LoginLevel int `json:"login_level"`
}

const (
	rogueKind    = "rogue"
	roguePVEKind = "rogue_pve"
	rogueSystem  = "roguelike"
	pveSystem    = "roguelikePve"
)

func (r *Rules) RogueAlternation(kind string, now int64) (RogueAlternationRule, bool) {
	moment := calendar.ContentTime(now)
	var result RogueAlternationRule
	found := false
	for _, rule := range r.Rogue.Alternations {
		if rule.Type == kind && rule.Start <= moment && (!found || rule.Start > result.Start) {
			result, found = rule, true
		}
	}
	return result, found
}

func (r *Rules) RogueRunID(now int64) int {
	if alternation, open := r.RogueAlternation(rogueKind, now); open && len(alternation.Levels) > 0 {
		return alternation.Levels[0]
	}
	if len(r.Rogue.Runs) == 0 {
		return 0
	}
	return slices.Min(slices.Collect(maps.Keys(r.Rogue.Runs)))
}

func (r *Rules) RoguePVEOptions(now int64) map[int]int {
	alternation, open := r.RogueAlternation(roguePVEKind, now)
	if !open {
		return nil
	}
	levels := map[int]map[int]int{}
	for _, id := range alternation.Levels {
		rule, known := r.Rogue.PVE[id]
		if !known {
			continue
		}
		if levels[rule.Difficulty] == nil {
			levels[rule.Difficulty] = map[int]int{}
		}
		levels[rule.Difficulty][rule.OpeningTime] = id
	}
	moment := calendar.ContentTime(now)
	day := floorDays(moment - calendar.RefreshSeconds)
	current := (day-((day+3)%7+7)%7)*calendar.DaySeconds + calendar.RefreshSeconds
	week := int64(1)
	if current > alternation.Start {
		diff := current - alternation.Start
		week = diff/calendar.WeekSeconds + 1
		if diff%calendar.WeekSeconds != 0 {
			week++
		}
	}
	result := map[int]int{}
	for difficulty := 1; levels[difficulty] != nil; difficulty++ {
		list := levels[difficulty]
		count := int64(0)
		for list[int(count)+1] != 0 {
			count++
		}
		if count == 0 {
			break
		}
		index := week
		if index > count {
			if index %= count; index == 0 {
				index = count
			}
		}
		result[difficulty] = list[int(index)]
	}
	return result
}

func (r *Rules) rogueRunBattles() map[model.BattleID]bool {
	result := map[model.BattleID]bool{}
	for _, run := range r.Rogue.Runs {
		for _, id := range run.BattlePool {
			result[id] = true
		}
		for _, value := range run.BattleSequence {
			if id, err := strconv.Atoi(value); err == nil {
				result[model.BattleID(id)] = true
			}
		}
	}
	return result
}

func (r *Rules) RogueRunBattle(id model.BattleID) bool {
	return r != nil && r.rogueRunBattles()[id]
}

func (r *Rules) RoguePVEBattle(id model.BattleID) bool {
	if r == nil {
		return false
	}
	for _, rule := range r.Rogue.PVE {
		for _, pool := range rule.Battles {
			if slices.Contains(pool, id) {
				return true
			}
		}
	}
	return false
}

func (r *Rules) RogueChestBattle(id model.BattleID) bool {
	if r == nil || id == 0 {
		return false
	}
	for _, chest := range r.Rogue.Chests {
		if chest.Battle == id {
			return true
		}
	}
	return false
}

func (r *Rules) RogueBattle(id model.BattleID) bool {
	return r.RogueRunBattle(id) || r.RoguePVEBattle(id) || r.RogueChestBattle(id)
}

func (a *Account) rogueUnlocked(r *Rules, system string, now int64) bool {
	rule, known := r.Rogue.Unlock[system]
	if !known {
		return false
	}
	return a.Level >= rule.Level || rule.LoginDay > 0 && rule.LoginLevel > 0 && a.Level >= rule.LoginLevel && LogicDay(now)-LogicDay(a.CreatedAt)+1 >= int64(rule.LoginDay)
}
