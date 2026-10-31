package account

import (
	"maps"
	"slices"

	"mmm-revived/internal/model"
)

type DrawRules struct {
	CalendarOffset int64                                     `json:"calendar_offset"`
	Pools          map[model.DrawPoolID]DrawPoolRule         `json:"pools"`
	Sets           map[model.DrawSetID]DrawSetRule           `json:"sets"`
	Strategies     map[model.DrawStrategyID]DrawStrategyRule `json:"strategies"`
}

type DrawPoolRule struct {
	Ticket         model.ItemID           `json:"ticket"`
	Price          int64                  `json:"price"`
	Level          int                    `json:"level"`
	Start          int64                  `json:"start"`
	End            int64                  `json:"end"`
	Sets           []model.DrawSetID      `json:"sets"`
	Strategies     []model.DrawStrategyID `json:"strategies"`
	WeightStrategy model.DrawStrategyID   `json:"weight_strategy"`
	Point          model.ItemID           `json:"point"`
}

type DrawSetRule struct {
	Items       []model.ItemID `json:"items"`
	Up          []model.ItemID `json:"up"`
	Weight      int64          `json:"weight"`
	SoftWeights map[int]int64  `json:"soft_weights"`
}

type DrawStrategyRule struct {
	Counter        int               `json:"counter"`
	Number         int               `json:"number"`
	Sets           []model.DrawSetID `json:"sets"`
	Prizes         []int             `json:"prizes"`
	TriggerSets    []model.DrawSetID `json:"trigger_sets"`
	TriggerPrizes  []int             `json:"trigger_prizes"`
	AdvancePrizes  []int             `json:"advance_prizes"`
	AdvanceNumber  int               `json:"advance_number"`
	AdvanceTrigger []int             `json:"advance_trigger"`
	Created        int64             `json:"created"`
}

func (a *Account) InitializeDraw() {
	if a.Draw.Pools == nil {
		a.Draw.Pools = map[model.DrawPoolID]model.DrawPoolState{}
	}
	if a.Draw.Counters == nil {
		a.Draw.Counters = map[int]model.DrawCounter{}
	}
	if a.Draw.History == nil {
		a.Draw.History = map[int64]model.DrawRecord{}
	}
}

func (a *Account) DrawSnapshot() model.DrawComponent {
	records := make([]model.DrawRecord, 0, len(a.Draw.History))
	for _, id := range slices.Sorted(maps.Keys(a.Draw.History)) {
		records = append(records, a.Draw.History[id])
	}
	return model.DrawComponent{Pools: a.Draw.Pools, Records: records, SharedPity: a.Draw.Counters}
}

func (r DrawRules) weightedSet(pool DrawPoolRule, pity int) (model.DrawSetID, error) {
	weights := make([]int64, len(pool.Sets))
	var total int64
	for i, id := range pool.Sets {
		set, exists := r.Sets[id]
		if !exists || len(set.Items) == 0 {
			return 0, rejected("draw prize set is unavailable")
		}
		weight := set.Weight
		for _, threshold := range slices.Sorted(maps.Keys(set.SoftWeights)) {
			if pity >= threshold {
				if adjusted, ok := set.SoftWeights[threshold]; ok {
					weight = adjusted
				}
			}
		}
		if weight < 0 || weight > 1000000 {
			return 0, rejected("invalid draw weights")
		}
		weights[i] = weight
		total += weight
	}
	if total <= 0 {
		return 0, rejected("draw pool is empty")
	}
	roll, err := randomRange(0, total-1)
	if err != nil {
		return 0, err
	}
	for i, weight := range weights {
		roll -= weight
		if roll < 0 {
			return pool.Sets[i], nil
		}
	}
	return 0, rejected("draw weights do not resolve")
}

func (r DrawRules) resolvePrize(prize int) (model.DrawSetID, model.ItemID, error) {
	setID, index := model.DrawSetID(prize/100), prize%100-1
	set, known := r.Sets[setID]
	if !known || index < 0 || index >= len(set.Items) {
		return 0, 0, rejected("draw guarantee references missing prize")
	}
	return setID, set.Items[index], nil
}

func (a *Account) DrawOne(r *Rules, id model.DrawPoolID, now int64) (model.ItemID, error) {
	a.InitializeDraw()
	pool, known := r.Draw.Pools[id]
	if !known || a.Level < pool.Level || !a.drawPoolOpen(r, id, pool, now) {
		return 0, rejected("draw pool is not open")
	}
	state := a.Draw.Pools[id]
	state.Satisfied = maps.Clone(state.Satisfied)
	if state.Satisfied == nil {
		state.Satisfied = map[model.DrawStrategyID]bool{}
	}
	pity := a.Draw.Counters[r.Draw.Strategies[pool.WeightStrategy].Counter].Number
	setID, err := r.Draw.weightedSet(pool, pity)
	if err != nil {
		return 0, err
	}
	var item model.ItemID
	var guarantee model.DrawStrategyID
	for _, strategyID := range pool.Strategies {
		strategy, exists := r.Draw.Strategies[strategyID]
		if !exists || strategy.Number <= 0 {
			return 0, rejected("draw strategy is unavailable")
		}
		count := state.Count
		if strategy.Counter != 0 {
			count = a.Draw.Counters[strategy.Counter].Number
		}
		if strategy.Created != 0 && a.CreatedAt > strategy.Created || strategy.Counter != 0 && len(strategy.Prizes) > 0 && a.Draw.Counters[strategy.Counter].Claimed {
			continue
		}
		if count+1 >= strategy.Number && (strategy.Counter != 0 || !state.Satisfied[strategyID]) {
			guarantee = strategyID
		}
	}
	if guarantee != 0 {
		strategy := r.Draw.Strategies[guarantee]
		if len(strategy.Prizes) > 0 {
			prizes := slices.DeleteFunc(slices.Clone(strategy.Prizes), func(prize int) bool {
				_, candidate, err := r.Draw.resolvePrize(prize)
				return err == nil && a.heroAwakened(r, candidate)
			})
			if len(prizes) == 0 {
				prizes = strategy.Prizes
			}
			prize, err := randomChoice(prizes)
			if err != nil {
				return 0, err
			}
			setID, item, err = r.Draw.resolvePrize(prize)
			if err != nil {
				return 0, err
			}
		} else {
			setID, err = randomChoice(strategy.Sets)
			if err != nil {
				return 0, err
			}
		}
	}
	set := r.Draw.Sets[setID]
	if item == 0 {
		choices := set.Items
		for _, strategyID := range pool.Strategies {
			strategy := r.Draw.Strategies[strategyID]
			if slices.Contains(strategy.Sets, setID) && strategy.AdvanceNumber > 0 && a.Draw.Counters[strategy.Counter].Advance+1 >= strategy.AdvanceNumber && len(set.Up) > 0 {
				choices = set.Up
			}
		}
		if fresh := slices.DeleteFunc(slices.Clone(choices), func(candidate model.ItemID) bool { return a.heroAwakened(r, candidate) }); len(fresh) > 0 {
			choices = fresh
		}
		item, err = randomChoice(choices)
		if err != nil {
			return 0, err
		}
	}
	if _, known := r.ItemRules[item]; !known {
		return 0, rejected("draw prize item is unavailable")
	}
	cost := map[model.ItemID]int64{}
	if a.Items[pool.Ticket] > 0 {
		cost[pool.Ticket] = 1
	} else {
		cost[r.Constants.Items.Diamond] = pool.Price
	}
	next := *a
	next.Items = maps.Clone(a.Items)
	if err = next.Spend(cost); err != nil {
		return 0, err
	}
	if a.Economy(r).FreeDraws {
		for currency, count := range cost {
			next.Items[currency] += count
		}
	}
	quantities := map[model.ItemID]int64{item: 1}
	if pool.Point > 0 {
		quantities[pool.Point]++
	}
	if err = next.GrantItems(r, quantities); err != nil {
		return 0, err
	}
	next.Draw.Pools = maps.Clone(a.Draw.Pools)
	next.Draw.Counters = maps.Clone(a.Draw.Counters)
	next.Draw.History = maps.Clone(a.Draw.History)
	updated := map[int]bool{}
	for _, strategyID := range pool.Strategies {
		strategy := r.Draw.Strategies[strategyID]
		trigger := slices.Contains(strategy.TriggerSets, setID) || slices.ContainsFunc(strategy.TriggerPrizes, func(prize int) bool {
			_, candidate, err := r.Draw.resolvePrize(prize)
			return err == nil && candidate == item
		})
		if strategy.Counter == 0 {
			if trigger || guarantee == strategyID {
				state.Satisfied[strategyID] = true
			}
			continue
		}
		if updated[strategy.Counter] {
			continue
		}
		updated[strategy.Counter] = true
		counter := next.Draw.Counters[strategy.Counter]
		counter.Number++
		if trigger {
			counter.Number = 0
		}
		if len(strategy.Prizes) > 0 && (trigger || guarantee == strategyID) {
			counter.Claimed = true
		}
		if strategy.AdvanceNumber > 0 && slices.Contains(strategy.Sets, setID) {
			counter.Advance++
			if slices.Contains(set.Up, item) {
				counter.Advance = 0
			}
		}
		next.Draw.Counters[strategy.Counter] = counter
	}
	state.Count++
	next.Draw.Pools[id] = state
	next.Draw.NextRecord++
	next.Draw.History[next.Draw.NextRecord] = model.DrawRecord{Pool: id, Item: item, Time: now}
	if len(next.Draw.History) > 500 {
		ids := slices.Sorted(maps.Keys(next.Draw.History))
		for _, id := range ids[:len(ids)-500] {
			delete(next.Draw.History, id)
		}
	}
	next.RefreshTasks(r)
	*a = next
	return item, nil
}

func (a *Account) heroAwakened(r *Rules, item model.ItemID) bool {
	rule := r.ItemRules[item]
	id := model.HeroID(rule.Parameter)
	hero, owned := a.Heroes[id]
	if rule.Type != 1002 || !owned || hero.Trial != 0 {
		return false
	}
	needed, known := map[model.ItemID]int64{}, false
	for awake, cost := range r.HeroAwake {
		if awake/10 != int(id) || len(cost.Cost) != 2 {
			continue
		}
		known = true
		if !slices.Contains(hero.AwakeIDs, awake) {
			needed[model.ItemID(cost.Cost[0])] += cost.Cost[1]
		}
	}
	for cost, count := range needed {
		if count > a.Items[cost] {
			return false
		}
	}
	return known
}
