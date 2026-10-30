package account

import (
	"maps"
	"slices"
	"strconv"
	"time"

	"mmm-revived/internal/json"
	"mmm-revived/internal/model"
)

func appendRecent[T any](records []T, record T, limit int) []T {
	records = append(records, record)
	if len(records) > limit {
		records = records[len(records)-limit:]
	}
	return records
}

func weightedIndex(weights []int, total int) (int, error) {
	roll, err := secureIntN(int64(total))
	if err != nil {
		return 0, err
	}
	for index, weight := range weights {
		roll -= int64(weight)
		if roll < 0 {
			return index, nil
		}
	}
	return -1, nil
}

func (a *Account) InitializeRogue(r *Rules, now int64) {
	p := &a.RogueProgress
	week := LogicWeek(now)
	if p.FirstClaimed == nil {
		p.FirstClaimed = map[int]int{}
	}
	if p.PVEFirstClaimed == nil {
		p.PVEFirstClaimed = map[int]int{}
	}
	if !p.Initialized {
		p.Initialized, p.ChestLevel = true, 1
	}
	if r != nil {
		p.RogueID, p.RankVersion, p.NewbiePassed = r.RogueRunID(now), r.Rogue.RankVersion, r.Rogue.NewbiePassed
	}
	if p.WeeklyRewards == nil || p.Week != week {
		p.Week, p.WeeklyScore, p.WeeklyCoin, p.WeeklyRewards = week, 0, 0, map[int]int{}
	}
	if p.PVEWeeklyRewards == nil || p.PVEWeek != week {
		p.PVEWeek, p.PVEWeeklyScore, p.PVEWeeklyCoin, p.PVEWeeklyRewards = week, 0, 0, map[int]int{}
	}
	if p.Difficulties == nil {
		p.Difficulties = map[int]model.RogueDifficulty{}
	}
	if !p.DifficultyCounted && r != nil {
		p.MaxDifficulty = 0
		for id, info := range p.Difficulties {
			if info.Wins > 0 {
				p.MaxDifficulty = max(p.MaxDifficulty, r.Rogue.PVE[id].Difficulty)
			}
		}
		p.DifficultyCounted = true
	}
	if p.Records == nil {
		p.Records = []model.RogueRecord{}
	}
	if !p.FinalWinsCounted && r != nil {
		for _, record := range p.Records {
			if record.Wins >= r.Rogue.MaximumWins {
				p.FinalWins++
			}
		}
		p.FinalWinsCounted = true
	}
	if p.PVERecords == nil {
		p.PVERecords = []model.RoguePVERecord{}
	}
	if !p.PVEWinsCounted {
		for _, record := range p.PVERecords {
			p.PVEWins += record.Round
		}
		if p.PVE.Status != model.RoguePVEIdle && p.PVE.Status != model.RoguePVESettle {
			p.PVEWins += p.PVE.Round
		}
		p.PVEWinsCounted = true
	}
}

func (a *Account) RogueComponent(pve bool) model.RogueComponent {
	p := &a.RogueProgress
	run := &p.Run
	result := model.RogueComponent{Status: int(run.Status), Battles: run.Battles, WeeklyCoin: p.WeeklyCoin, WeeklyScore: p.WeeklyScore, WeeklyRewards: p.WeeklyRewards, Heroes: run.Heroes, Shop: run.Shop, Objects: run.Objects, AddHeroPool: run.HeroPool, Banned: run.Banned, RogueID: p.RogueID, Version: p.RankVersion, PVEVersion: p.RankVersion, Index: run.Index, Life: run.Life, ChestLevel: max(1, p.ChestLevel), TotalWins: p.TotalWins, FinalWins: p.FinalWins, RankScore: p.RankScore, Difficulty: p.Difficulties, MaxDifficulty: p.MaxDifficulty, Coin: run.Coin, FreeRefresh: run.FreeRefresh, RefreshCount: run.RefreshCount, SaleCount: run.SaleCount, MergeCount: run.MergeCount, LastDeaths: run.LastDeaths, Records: rogueHistory(p.Records), PVERecords: rogueHistory(p.PVERecords)}
	if run.Status != model.RogueIdle && run.ID != 0 {
		result.RogueID = run.ID
	}
	if p.NewbiePassed {
		result.TutorialPassed = 1
	}
	if p.FirstRewardsDone {
		result.AllFirstRewards = 1
	} else {
		result.WeeklyRewards = p.FirstClaimed
	}
	if pve {
		v := &p.PVE
		result.Status, result.Battles, result.PVEBattle, result.PVEID, result.Round, result.TotalTime = int(v.Status), v.Battles, v.Battle, v.ID, v.Round, v.Duration
		result.WeeklyCoin, result.WeeklyScore, result.WeeklyRewards, result.AllFirstRewards = p.PVEWeeklyCoin, p.PVEWeeklyScore, p.PVEWeeklyRewards, 0
		if p.PVEFirstRewardsDone {
			result.AllFirstRewards = 1
		} else {
			result.WeeklyRewards = p.PVEFirstClaimed
		}
		result.EventIndex = v.EventIndex
		if v.Round < len(v.EventGroups) {
			result.EventGroup = v.EventGroups[v.Round]
		}
	}
	return result
}

func rogueObject(run *model.RogueRun, prefix string, value model.RogueObject) model.InstanceID {
	run.NextObject++
	id := model.InstanceID(prefix + strconv.FormatInt(run.NextObject, 10))
	run.Objects[id] = value
	return id
}

func rogueHero(r *Rules, id model.HeroID, level, index int) model.RogueObject {
	return model.RogueObject{ID: int(id), Level: level, Index: index, Star: r.Rogue.HeroStar, Talents: map[int]int{1: 1, 2: 1, 3: 1, 4: 1, 5: 1}, Awake: []int{}, Highlights: map[int]model.InstanceID{}}
}

func rogueHeroCandidates(r *Rules, run *model.RogueRun, wins int) []model.HeroID {
	used := make(map[int]bool, len(run.Heroes))
	for _, oid := range run.Heroes {
		used[run.Objects[oid].ID] = true
	}
	var candidates []model.HeroID
	for _, hero := range rogueHeroPool(r, run.ID) {
		if !used[int(hero)] && r.Rogue.HeroUnlockWins[hero] <= wins {
			candidates = append(candidates, hero)
		}
	}
	return candidates
}

func rogueHeroPool(r *Rules, id int) []model.HeroID {
	var heroes []model.HeroID
	for _, pool := range r.Rogue.Runs[id].HeroPool {
		heroes = append(heroes, pool...)
	}
	slices.Sort(heroes)
	return slices.Compact(heroes)
}

func (a *Account) InitRogue(r *Rules, now int64) error {
	a.InitializeRogue(r, now)
	if !a.rogueUnlocked(r, rogueSystem, now) || a.RogueProgress.Run.Status != model.RogueIdle {
		return rejected("rogue run is active or locked")
	}
	if _, open := r.RogueAlternation(rogueKind, now); !open {
		return rejected("rogue is closed")
	}
	id := a.RogueProgress.RogueID
	rule, known := r.Rogue.Runs[id]
	if !known || len(rule.HeroPool) == 0 {
		return rejected("rogue rules unavailable")
	}
	run := model.RogueRun{Status: model.RogueChooseHero, ID: id, Life: r.Rogue.Life, Heroes: map[int]model.InstanceID{}, Objects: map[model.InstanceID]model.RogueObject{}, Shop: map[int]model.RogueShopEntry{}, HeroPool: []model.HeroID{}, Banned: []model.HeroID{}, Started: now}
	heroes, err := sample(rogueHeroCandidates(r, &run, a.RogueProgress.TotalWins), max(1, r.Rogue.HeroNum))
	if err != nil {
		return err
	}
	if len(heroes) == 0 {
		return rejected("rogue has no heroes")
	}
	for index, hero := range heroes {
		run.Heroes[index] = rogueObject(&run, rogueHeroPrefix, rogueHero(r, hero, max(1, r.Rogue.HeroLevel), index))
	}
	for _, value := range rule.BattleSequence {
		var id model.BattleID
		if value == "b" {
			id, err = randomChoice(rule.BattlePool)
		} else {
			var number int64
			number, err = strconv.ParseInt(value, 10, 32)
			id = model.BattleID(number)
		}
		if err != nil {
			return err
		}
		if !r.RogueRunBattle(id) {
			return rejected("rogue battle is unavailable")
		}
		run.Battles = append(run.Battles, id)
	}
	if len(run.Battles) == 0 {
		return rejected("rogue has no battles")
	}
	a.RogueProgress.Run = run
	return nil
}

func (a *Account) RechooseRogueHero(r *Rules, index int) error {
	run := &a.RogueProgress.Run
	oid, exists := run.Heroes[index]
	if run.Status != model.RogueChooseHero || !exists || run.Rerolls >= r.Rogue.RechooseLimit {
		return rejected("hero cannot be reselected")
	}
	hero, err := randomChoice(rogueHeroCandidates(r, run, a.RogueProgress.TotalWins))
	if err != nil {
		return err
	}
	old := run.Objects[oid]
	old.ID = int(hero)
	run.Objects[oid] = old
	run.Rerolls++
	return nil
}

func (a *Account) rogueShop(r *Rules, replace bool) error {
	run := &a.RogueProgress.Run
	rule := r.Rogue.Runs[run.ID]
	if run.Index >= len(rule.Shops) {
		return rejected("rogue shop round is unavailable")
	}
	shop, known := r.Rogue.Shops[rule.Shops[run.Index]]
	if !known {
		return rejected("rogue shop is unavailable")
	}
	var pool, weights []int
	total := 0
	for _, id := range rule.RelicPool {
		relic := r.Rogue.Relics[id]
		if relic.Tier > 0 && relic.Tier <= len(shop.Weights) && shop.Weights[relic.Tier-1] > 0 {
			weight := shop.Weights[relic.Tier-1]
			pool = append(pool, id)
			weights = append(weights, weight)
			total += weight
		}
	}
	for i := range shop.Relics {
		old, exists := run.Shop[i]
		if exists && old.Status == 2 {
			continue
		}
		if exists && old.Status != 3 {
			delete(run.Objects, old.Object)
		}
		if len(pool) == 0 {
			return rejected("rogue relic pool is empty")
		}
		index, err := weightedIndex(weights, total)
		if err != nil {
			return err
		}
		oid := rogueObject(run, rogueRelicPrefix, model.RogueObject{ID: pool[max(index, 0)], Level: 1, Awake: []int{}, Highlights: map[int]model.InstanceID{}, Talents: map[int]int{}})
		run.Shop[i] = model.RogueShopEntry{Object: oid, Status: 1}
	}
	return nil
}

func (a *Account) StartRogue(r *Rules) error {
	run := &a.RogueProgress.Run
	if run.Status != model.RogueChooseHero || len(run.Heroes) == 0 {
		return rejected("rogue is not ready to start")
	}
	run.Status = model.RogueShopping
	run.Coin = r.Rogue.Shops[r.Rogue.Runs[run.ID].Shops[0]].Coin
	if err := a.rogueShop(r, false); err != nil {
		return err
	}
	return a.rogueSkills(r, roguePrepare, model.RogueObject{}, -1, false)
}

func (a *Account) SelectRogueHero(r *Rules, index int) error {
	run := &a.RogueProgress.Run
	if run.Status == model.RogueChooseHero {
		return a.RechooseRogueHero(r, index)
	}
	if run.Status != model.RogueAddHero || index < 0 || index >= len(run.HeroPool) {
		return rejected("invalid rogue hero choice")
	}
	id := run.HeroPool[index]
	slot := len(run.Heroes)
	for _, oid := range run.Heroes {
		if run.Objects[oid].ID == int(id) {
			return rejected("hero already selected")
		}
	}
	level := 1
	if shop := r.Rogue.Shops[r.Rogue.Runs[run.ID].Shops[run.Index]]; len(shop.AddHero) > 1 {
		level = shop.AddHero[1]
	}
	run.Heroes[slot] = rogueObject(run, rogueHeroPrefix, rogueHero(r, id, level, slot))
	run.HeroPool = []model.HeroID{}
	run.Status = model.RogueShopping
	return nil
}

func (a *Account) RefreshRogueShop(r *Rules) error {
	run := &a.RogueProgress.Run
	if run.Status != model.RogueShopping {
		return rejected("rogue shop is closed")
	}
	if run.FreeRefresh > 0 {
		run.FreeRefresh--
	} else {
		if run.Coin < r.Rogue.RefreshPrice {
			return rejected("insufficient rogue coins")
		}
		run.Coin -= r.Rogue.RefreshPrice
	}
	run.RefreshCount++
	a.RecordJourneyTaskEvent(r, 84, 1, 0)
	if err := a.rogueShop(r, true); err != nil {
		return err
	}
	return a.rogueSkills(r, rogueRefresh, model.RogueObject{}, -1, false)
}

func rogueMerge(base, extra model.RogueObject) (model.RogueObject, error) {
	if base.ID != extra.ID || base.Level != extra.Level || base.Level >= 3 {
		return base, rejected("relics cannot merge")
	}
	base.Level++
	base.ExtraLevel += extra.ExtraLevel
	return base, nil
}

func (a *Account) BuyRogueRelic(r *Rules, shopIndex, heroIndex int) (bool, error) {
	run := &a.RogueProgress.Run
	entry, exists := run.Shop[shopIndex]
	heroID, owned := run.Heroes[heroIndex]
	if run.Status != model.RogueShopping || !exists || entry.Status == 3 || !owned {
		return false, rejected("invalid rogue relic purchase")
	}
	relic, known := run.Objects[entry.Object]
	if !known || relic.Level < 1 || relic.Level > len(r.Rogue.RelicPrices) {
		return false, rejected("unknown rogue relic")
	}
	price := max(0, r.Rogue.RelicPrices[relic.Level-1]-relic.Discount)
	if run.Coin < price {
		return false, rejected("insufficient rogue coins")
	}
	hero := run.Objects[heroID]
	up := hero.Relic != ""
	if up {
		old, exists := run.Objects[hero.Relic]
		if !exists {
			return false, rejected("equipped rogue relic is missing")
		}
		merged, err := rogueMerge(old, relic)
		if err != nil {
			return false, err
		}
		run.Objects[hero.Relic] = merged
		delete(run.Objects, entry.Object)
	} else {
		hero.Relic = entry.Object
		run.Objects[heroID] = hero
	}
	run.Coin -= price
	a.RecordJourneyTaskEvent(r, 83, 1, 0)
	run.Shop[shopIndex] = model.RogueShopEntry{Status: 3}
	source := run.Objects[hero.Relic]
	if err := a.rogueSkills(r, rogueBuy, source, heroIndex, true); err != nil {
		return false, err
	}
	if up {
		run.MergeCount++
		a.RecordJourneyTaskEvent(r, 85, 1, 0)
		if err := a.rogueSkills(r, rogueMerged, source, heroIndex, true); err != nil {
			return false, err
		}
	}
	return up, nil
}

func (a *Account) SaleRogueRelic(r *Rules, index int) error {
	run := &a.RogueProgress.Run
	oid, exists := run.Heroes[index]
	hero := run.Objects[oid]
	if run.Status != model.RogueShopping || !exists || hero.Relic == "" {
		return rejected("hero has no relic to sell")
	}
	source := run.Objects[hero.Relic]
	delete(run.Objects, hero.Relic)
	hero.Relic = ""
	run.Objects[oid] = hero
	run.Coin += r.Rogue.SalePrice
	run.SaleCount++
	a.RecordJourneyTaskEvent(r, 83, 1, 0)
	return a.rogueSkills(r, rogueSale, source, index, true)
}

func (a *Account) MergeRogueShop(r *Rules, baseIndex, extraIndex int) error {
	run := &a.RogueProgress.Run
	base, baseOK := run.Shop[baseIndex]
	extra, extraOK := run.Shop[extraIndex]
	if run.Status != model.RogueShopping || baseIndex == extraIndex || !baseOK || !extraOK || base.Status == 3 || extra.Status == 3 {
		return rejected("invalid shop relic merge")
	}
	combined, err := rogueMerge(run.Objects[base.Object], run.Objects[extra.Object])
	if err != nil {
		return err
	}
	run.Objects[base.Object] = combined
	delete(run.Objects, extra.Object)
	run.Shop[extraIndex] = model.RogueShopEntry{Status: 3}
	run.MergeCount++
	a.RecordJourneyTaskEvent(r, 85, 1, 0)
	return a.rogueSkills(r, rogueMerged, combined, -1, false)
}

func (a *Account) SwitchRogueRelics(r *Rules, baseIndex, extraIndex int) (bool, error) {
	run := &a.RogueProgress.Run
	baseID, baseOK := run.Heroes[baseIndex]
	extraID, extraOK := run.Heroes[extraIndex]
	if run.Status != model.RogueShopping || baseIndex == extraIndex || !baseOK || !extraOK {
		return false, rejected("invalid relic transfer")
	}
	base, extra := run.Objects[baseID], run.Objects[extraID]
	if extra.Relic == "" {
		return false, rejected("source hero has no relic")
	}
	up := false
	baseRelic, extraRelic := run.Objects[base.Relic], run.Objects[extra.Relic]
	if base.Relic != "" && baseRelic.ID == extraRelic.ID && baseRelic.Level == extraRelic.Level && baseRelic.Level < 3 {
		combined, err := rogueMerge(baseRelic, extraRelic)
		if err != nil {
			return false, err
		}
		run.Objects[base.Relic] = combined
		delete(run.Objects, extra.Relic)
		extra.Relic = ""
		run.MergeCount++
		up = true
	} else {
		base.Relic, extra.Relic = extra.Relic, base.Relic
	}
	run.Objects[baseID], run.Objects[extraID] = base, extra
	if up {
		if err := a.rogueSkills(r, rogueMerged, run.Objects[base.Relic], baseIndex, true); err != nil {
			return false, err
		}
	}
	return up, nil
}

func (a *Account) LockRogueRelic(index int) (int, error) {
	run := &a.RogueProgress.Run
	entry, exists := run.Shop[index]
	if run.Status != model.RogueShopping || !exists || entry.Status == 3 {
		return 0, rejected("relic cannot be locked")
	}
	entry.Status = 3 - entry.Status
	run.Shop[index] = entry
	return entry.Status, nil
}

func (a *Account) SwitchRogueHeroes(first, second int) error {
	run := &a.RogueProgress.Run
	one, ok1 := run.Heroes[first]
	two, ok2 := run.Heroes[second]
	if run.Status != model.RogueShopping || !ok1 || !ok2 || first == second {
		return rejected("invalid rogue hero swap")
	}
	h1, h2 := run.Objects[one], run.Objects[two]
	h1.Index, h2.Index = second, first
	run.Objects[one], run.Objects[two] = h1, h2
	run.Heroes[first], run.Heroes[second] = two, one
	return nil
}

func (a *Account) FinishRogueShopping(r *Rules) error {
	run := &a.RogueProgress.Run
	if run.Status != model.RogueShopping || len(run.Heroes) == 0 || run.Index >= len(run.Battles) {
		return rejected("rogue is not ready for battle")
	}
	if err := a.rogueSkills(r, rogueFinish, model.RogueObject{}, -1, false); err != nil {
		return err
	}
	run.Status = model.RogueBattle
	return nil
}

func (a *Account) SettleRogue(r *Rules) error {
	run := &a.RogueProgress.Run
	if run.Status == model.RogueIdle {
		return rejected("rogue has not started")
	}
	if run.Status != model.RogueSettle && run.Status != model.RogueChooseHero {
		a.RogueProgress.Records = appendRecent(a.RogueProgress.Records, a.rogueRecord(), r.Rogue.RecordLimit)
		if rogueRunOver(r, run) {
			if run.Wins >= r.Rogue.MaximumWins {
				a.RogueProgress.FinalWins++
			}
			a.RecordJourneyRogueFinish(r, 1, run.Wins)
		}
	}
	run.Status = model.RogueSettle
	return nil
}

func rogueRunOver(r *Rules, run *model.RogueRun) bool {
	return run.Life <= 0 || run.Index >= len(run.Battles) || run.Wins >= r.Rogue.MaximumWins
}

func (a *Account) rogueLimitedReward(r *Rules, pve bool, values map[model.ItemID]int64, now int64) error {
	progress := &a.RogueProgress
	coin, weekly, kind := r.Rogue.Currency, &progress.WeeklyCoin, rogueKind
	if pve {
		coin, weekly, kind = r.Rogue.PVECurrency, &progress.PVEWeeklyCoin, roguePVEKind
	}
	alternation, _ := r.RogueAlternation(kind, now)
	if amount, exists := values[coin]; exists {
		if amount = min(amount, max(0, alternation.CoinLimit-*weekly)); amount > 0 {
			values[coin] = amount
		} else {
			delete(values, coin)
		}
	}
	if err := a.GrantItems(r, values); err != nil {
		return err
	}
	*weekly += values[coin]
	return nil
}

func (a *Account) ClaimRogue(r *Rules, pve bool, now int64) (map[model.ItemID]int64, error) {
	a.InitializeRogue(r, now)
	p := &a.RogueProgress
	reward, count := r.Rogue.WinReward, p.Run.Wins
	if pve {
		if p.PVE.Status != model.RoguePVESettle || p.PVE.Claimed {
			return nil, rejected("rogue pve is not ready to claim")
		}
		rule := r.Rogue.PVE[p.PVE.ID]
		if rule.Difficulty < 1 || rule.Difficulty > len(r.Rogue.PVEWinRewards) {
			return nil, rejected("invalid rogue pve reward")
		}
		reward, count = r.Rogue.PVEWinRewards[rule.Difficulty-1], p.PVE.Round
	} else if p.Run.Status != model.RogueSettle || p.Run.Claimed {
		return nil, rejected("rogue is not ready to claim")
	}
	values := map[model.ItemID]int64{}
	if count > 0 {
		var err error
		if values, err = rewardItems(r, reward, int64(count)); err != nil {
			return nil, err
		}
	}
	if err := a.rogueLimitedReward(r, pve, values, now); err != nil {
		return nil, err
	}
	if pve {
		p.PVE = model.RoguePVERun{}
	} else {
		p.Run.Claimed = true
		p.Run.Status = model.RogueIdle
	}
	return values, nil
}

func (a *Account) ClaimRogueWeekly(r *Rules, pve bool, now int64) (map[model.ItemID]int64, error) {
	a.InitializeRogue(r, now)
	p := &a.RogueProgress
	score, claimed, done := p.WeeklyScore, p.WeeklyRewards, &p.FirstRewardsDone
	kind := rogueKind
	if pve {
		kind = roguePVEKind
	}
	alternation, open := r.RogueAlternation(kind, now)
	if !open {
		return nil, rejected("rogue is closed")
	}
	rewards := r.Rogue.FirstRewards
	if p.FirstRewardsDone {
		rewards = alternation.Rewards
	} else {
		claimed = p.FirstClaimed
	}
	if pve {
		score, claimed, done = p.PVEWeeklyScore, p.PVEWeeklyRewards, &p.PVEFirstRewardsDone
		rewards = r.Rogue.PVEFirstRewards
		if *done {
			rewards = alternation.Rewards
		} else {
			claimed = p.PVEFirstClaimed
		}
	}
	values := map[model.ItemID]int64{}
	var thresholds []int
	for threshold, reward := range rewards {
		if int64(threshold) > score || claimed[threshold] != 0 {
			continue
		}
		items, err := rewardItems(r, reward, 1)
		if err != nil {
			return nil, err
		}
		addItems(values, items)
		thresholds = append(thresholds, threshold)
	}
	if len(thresholds) == 0 {
		return nil, rejected("no rogue weekly rewards available")
	}
	if err := a.rogueLimitedReward(r, pve, values, now); err != nil {
		return nil, err
	}
	for _, threshold := range thresholds {
		claimed[threshold] = 1
	}
	if len(claimed) == len(rewards) {
		*done = true
		if pve {
			p.PVEWeeklyRewards = maps.Clone(claimed)
		} else {
			p.WeeklyRewards = maps.Clone(claimed)
		}
	}
	return values, nil
}

func (a *Account) OpenRogueChest(r *Rules, count, index int) (map[model.ItemID]int64, error) {
	p := &a.RogueProgress
	run := r.Rogue.Runs[p.RogueID]
	if count < 1 || count > 9999 || p.ChestLevel < 1 || p.ChestLevel > len(run.Chests) {
		return nil, rejected("invalid rogue chest")
	}
	rule := r.Rogue.Chests[run.Chests[p.ChestLevel-1]]
	if a.Level < rule.Level || index < -1 || index >= len(rule.Rewards) {
		return nil, rejected("rogue chest reward unavailable")
	}
	var values map[model.ItemID]int64
	var err error
	if index >= 0 {
		values, err = rogueChestItems(r, rule.Rewards[index:index+1], int64(count))
	} else {
		picks := make([]model.RewardID, count)
		for i := range picks {
			if picks[i], err = randomChoice(rule.Rewards); err != nil {
				return nil, err
			}
		}
		values, err = rogueChestItems(r, picks, 1)
	}
	if err != nil {
		return nil, err
	}
	cost, err := multiplied(rule.Cost, int64(count))
	if err != nil {
		return nil, err
	}
	cost[r.Constants.Items.Energy] += rule.Energy * int64(count)
	if err = a.Spend(cost); err != nil {
		return nil, err
	}
	if err = a.GrantItems(r, values); err != nil {
		return nil, err
	}
	return values, nil
}

const rogueChestChancePer = 1000000

func rogueChestItems(r *Rules, ids []model.RewardID, count int64) (map[model.ItemID]int64, error) {
	result := map[model.ItemID]int64{}
	for _, id := range ids {
		drop, known := r.Rogue.ChestDrops[id]
		if !known {
			items, err := droppedRewardItems(r, []model.RewardID{id}, count)
			if err != nil {
				return nil, err
			}
			addItems(result, items)
			continue
		}
		items, err := rewardItems(r, id, count)
		if err != nil {
			return nil, err
		}
		guaranteed, err := multiplied(drop.Guaranteed, count)
		if err != nil {
			return nil, err
		}
		for item, amount := range guaranteed {
			if !addQuantity(items, item, amount) {
				return nil, rejected("reward overflow")
			}
		}
		extras := slices.Sorted(maps.Keys(drop.Extra))
		for range count {
			roll, err := secureIntN(rogueChestChancePer)
			if err != nil {
				return nil, err
			}
			for _, item := range extras {
				if roll -= drop.Extra[item]; roll < 0 {
					items[item]++
					break
				}
			}
		}
		for item, amount := range items {
			if amount > 0 && !addQuantity(result, item, amount) {
				return nil, rejected("reward overflow")
			}
		}
	}
	return result, nil
}

func (a *Account) ValidateRogueBattle(r *Rules, id model.BattleID) error {
	p := &a.RogueProgress
	run, v := &p.Run, &p.PVE
	switch {
	case r.RogueRunBattle(id):
		if run.Status == model.RogueBattle && !rogueRunOver(r, run) && run.Battles[run.Index] == id {
			return nil
		}
	case r.RoguePVEBattle(id):
		if (v.Status == model.RoguePVEPrepare || v.Status == model.RoguePVEBattle) && v.Round < len(v.Battles) && v.Battles[v.Round] == id {
			return nil
		}
	case r.RogueChestBattle(id):
		chests := r.Rogue.Runs[p.RogueID].Chests
		if run.Status == model.RogueIdle && p.ChestLevel >= 1 && p.ChestLevel < len(chests) {
			if rule := r.Rogue.Chests[chests[p.ChestLevel]]; rule.Battle == id && a.Level >= rule.Level {
				return nil
			}
		}
	}
	return rejected("battle does not match the active rogue run")
}

func (a *Account) FinishRogueBattle(r *Rules, id model.BattleID, win bool, duration int64) error {
	if err := a.ValidateRogueBattle(r, id); err != nil {
		return err
	}
	p := &a.RogueProgress
	switch {
	case r.RogueRunBattle(id):
		return a.finishRogueRunBattle(r, win)
	case r.RoguePVEBattle(id):
		return a.finishRoguePVEBattle(r, win, duration)
	}
	if win {
		p.ChestLevel++
	}
	return nil
}

func (a *Account) finishRogueRunBattle(r *Rules, win bool) error {
	p := &a.RogueProgress
	run := &p.Run
	run.Index++
	previous := p.RankScore
	if win {
		run.Wins++
		p.TotalWins++
		p.WeeklyScore += r.Rogue.Score
		p.RankScore += r.Rogue.RankWinScore
	} else {
		run.Life--
		p.RankScore = max(0, p.RankScore-r.Rogue.RankLossScore)
	}
	run.RankChange += p.RankScore - previous
	if rogueRunOver(r, run) {
		run.Battles = run.Battles[:min(run.Index, len(run.Battles))]
		return nil
	}
	shop := r.Rogue.Shops[r.Rogue.Runs[run.ID].Shops[run.Index]]
	run.Coin += shop.Coin
	run.Status = model.RogueShopping
	if err := a.rogueShop(r, false); err != nil {
		return err
	}
	run.RoundTriggers = map[model.InstanceID]int{}
	if err := a.rogueSkills(r, roguePrepare, model.RogueObject{}, -1, false); err != nil {
		return err
	}
	if len(shop.AddHero) > 0 {
		candidates := rogueHeroCandidates(r, run, p.TotalWins)
		options, err := sample(candidates, min(shop.AddHero[0], len(candidates)))
		if err != nil {
			return err
		}
		run.HeroPool = options
		run.Status = model.RogueAddHero
	}
	return nil
}

func (a *Account) finishRoguePVEBattle(r *Rules, win bool, duration int64) error {
	p := &a.RogueProgress
	v := &p.PVE
	v.Duration += max(0, duration)
	if !win {
		v.Status = model.RoguePVESettle
		p.PVERecords = appendRecent(p.PVERecords, a.roguePVERecord(), r.Rogue.RecordLimit)
		a.RecordJourneyRogueFinish(r, 2, v.Round)
		return nil
	}
	v.Round++
	p.PVEWins++
	p.PVEWeeklyScore += r.Rogue.PVEScore
	if v.Round >= len(v.Battles) {
		v.Status = model.RoguePVESettle
		info := p.Difficulties[v.ID]
		info.Wins++
		info.Score = max(info.Score, r.Rogue.MaxTimeScore-v.Duration)
		p.Difficulties[v.ID] = info
		p.MaxDifficulty = max(p.MaxDifficulty, r.Rogue.PVE[v.ID].Difficulty)
		p.PVERecords = appendRecent(p.PVERecords, a.roguePVERecord(), r.Rogue.RecordLimit)
		a.RecordJourneyRogueFinish(r, 2, v.Round)
		return nil
	}
	v.EventIndex = 0
	v.Status = model.RoguePVEEvent
	return a.prepareRogueEvent(r)
}

func rogueHistory[T any](records []T) []string {
	result := make([]string, 0, len(records))
	for _, record := range slices.Backward(records) {
		if data, err := json.Marshal(record); err == nil {
			result = append(result, string(data))
		}
	}
	return result
}

func (a *Account) rogueRecord() model.RogueRecord {
	run := &a.RogueProgress.Run
	result := model.RogueRecord{Wins: run.Wins, Losses: run.Index - run.Wins, Time: time.Now().Unix(), Team: make([]model.RogueHeroRecord, 0, len(run.Heroes))}
	for index := range len(run.Heroes) {
		hero := run.Objects[run.Heroes[index]]
		relic := run.Objects[hero.Relic]
		result.Team = append(result.Team, model.RogueHeroRecord{Hero: model.HeroID(hero.ID), Level: hero.Level, Relic: relic.ID, RelicLevel: relic.Level, ExtraLevel: relic.ExtraLevel})
	}
	return result
}

func (a *Account) roguePVERecord() model.RoguePVERecord {
	run := &a.RogueProgress.PVE
	result := model.RoguePVERecord{Round: run.Round, Time: time.Now().Unix(), Level: run.ID, Team: model.RoguePVERecordTeam{Heroes: make([]model.RoguePVERecordHero, 0, len(run.Battle.Heroes)), Buffs: slices.Clone(run.Battle.Buffs)}}
	for _, id := range run.Battle.Heroes {
		hero := a.Heroes[id]
		result.Team.Heroes = append(result.Team.Heroes, model.RoguePVERecordHero{ID: id, Level: hero.Level, Star: hero.Star, Break: hero.Break, Talents: maps.Clone(hero.Talents), Awake: slices.Clone(hero.AwakeIDs)})
	}
	return result
}
