package account

import (
	"maps"
	"math"
	"slices"

	"mmm-revived/internal/model"
)

const MaxBulkCount = math.MaxInt32

func shopPeriod(kind int, now int64) int64 {
	switch kind {
	case 1:
		return LogicDay(now)
	case 2:
		return LogicWeek(now)
	case 3:
		return LogicMonth(now)
	default:
		return 0
	}
}

func addItems(total, items map[model.ItemID]int64) {
	for item, count := range items {
		total[item] += count
	}
}

func (a *Account) exchange(r *Rules, cost, rewards map[model.ItemID]int64) error {
	before := maps.Clone(a.Items)
	if err := a.Spend(cost); err != nil {
		return err
	}
	if err := a.GrantItems(r, rewards); err != nil {
		a.Items = before
		return err
	}
	return nil
}

func (a *Account) BuyEnergy(r *Rules, count int, now int64) error {
	day, bought := LogicDay(now), 0
	if a.Energy.Day == day {
		bought = a.Energy.Bought
	}
	if count <= 0 || count > len(r.Energy)-bought {
		return rejected("daily energy purchase limit reached")
	}
	cost, energy := int64(0), int64(0)
	for index := bought + 1; index <= bought+count; index++ {
		row, known := r.Energy[index]
		if !known || row.Price < 0 || row.Energy < 0 || cost > math.MaxInt64-row.Price || energy > math.MaxInt64-row.Energy {
			return rejected("invalid energy purchase")
		}
		cost, energy = cost+row.Price, energy+row.Energy
	}
	if a.Items[r.Constants.Items.Energy] > math.MaxInt64-energy {
		return rejected("energy overflow")
	}
	if err := a.Spend(map[model.ItemID]int64{r.Constants.Items.Diamond: cost}); err != nil {
		return err
	}
	a.Items[r.Constants.Items.Energy] += energy
	a.Energy.Day, a.Energy.Bought = day, bought+count
	a.RefreshTasks(r)
	return nil
}

func (a *Account) TransferEnergy(r *Rules, count int64) error {
	if count <= 0 || count > a.Energy.Bank || a.Items[r.Constants.Items.Energy] > math.MaxInt64-count {
		return rejected("invalid bank transfer")
	}
	a.Energy.Bank -= count
	a.Items[r.Constants.Items.Energy] += count
	return nil
}

func (a *Account) ConvertYuanbao(r *Rules, count int64) error {
	if count <= 0 || a.Items[r.Constants.Items.Diamond] > math.MaxInt64-count {
		return rejected("invalid currency exchange")
	}
	if err := a.Spend(map[model.ItemID]int64{r.Constants.Items.Yuanbao: count}); err != nil {
		return err
	}
	a.Items[r.Constants.Items.Diamond] += count
	a.RefreshTasks(r)
	return nil
}

func (a *Account) BuyCommodity(r *Rules, id model.ShopID, count int, now int64, guildLevel int) (map[model.ItemID]int64, error) {
	rule, known := r.Shops[id]
	if !known || len(rule.Prices) == 0 || count < 1 || count > MaxBulkCount || a.Level < rule.Level || rule.GuildLevel > guildLevel || rule.Recharge != 0 {
		return nil, rejected("commodity is unavailable")
	}
	calendar := now + r.Journey.CalendarOffset
	if rule.Start > calendar || rule.End > 0 && calendar > rule.End {
		return nil, rejected("commodity sale period is closed")
	}
	if rule.Predecessor != 0 && a.ShopPurchases[rule.Predecessor].Total == 0 {
		return nil, rejected("previous commodity has not been bought")
	}
	if rule.PreTask != 0 && a.Tasks[rule.PreTask].Status < 3 {
		return nil, rejected("commodity requires an unfinished task")
	}
	purchase := a.ShopPurchases[id]
	period := shopPeriod(rule.Period, now)
	windowed := rule.Period == 5 && rule.PeriodHours > 0
	if windowed {
		period, purchase.Count = 0, 0
		if window := a.System.ShopWindows[id]; window.Ends > now {
			purchase.Count = window.Count
		}
	} else if purchase.Period != period {
		purchase.Count = 0
	}
	if rule.Limit > 0 && count > rule.Limit-purchase.Count {
		return nil, rejected("commodity purchase limit reached")
	}
	cost, valid := commodityCost(rule.Prices, purchase.Count, count)
	if !valid {
		return nil, rejected("invalid commodity price")
	}
	rewards, err := rewardItems(r, rule.Reward, int64(count))
	if err != nil {
		return nil, err
	}
	if err = a.exchange(r, map[model.ItemID]int64{rule.Currency: cost}, rewards); err != nil {
		return nil, err
	}
	purchase.Count, purchase.Total, purchase.Period, purchase.PeriodType = purchase.Count+count, purchase.Total+count, period, rule.Period
	if windowed {
		duration := rule.PeriodHours * 3600
		a.System.ShopWindows[id] = model.SystemShopWindow{Count: purchase.Count, Ends: now + duration - (calendar-rule.Start)%duration}
		purchase.PeriodType = 0
	}
	a.ShopPurchases[id] = purchase
	return rewards, nil
}

func commodityCost(prices []int64, bought, count int) (int64, bool) {
	cost, last := int64(0), len(prices)-1
	for index := bought; index < bought+count; index++ {
		price := prices[min(index, last)]
		if price < 0 {
			return 0, false
		}
		if index >= last {
			remaining := int64(bought + count - index)
			if price != 0 && remaining > (math.MaxInt64-cost)/price {
				return 0, false
			}
			return cost + price*remaining, true
		}
		if cost > math.MaxInt64-price {
			return 0, false
		}
		cost += price
	}
	return cost, true
}

func (a *Account) Craft(r *Rules, id model.CraftID, count int, materials map[model.ItemID]int64, up, position int) (map[model.ItemID]int64, error) {
	rule, known := r.Crafts[id]
	if !known || count < 1 || count > MaxBulkCount || rule.Initial == 0 && !slices.Contains(a.CraftRecipes, id) {
		return nil, rejected("craft recipe is locked")
	}
	if up != 0 || position != 0 {
		return nil, rejected("directed equipment crafting is unavailable")
	}
	unit := maps.Clone(rule.Materials)
	if unit == nil {
		unit = map[model.ItemID]int64{}
	}
	optional := int64(0)
	for item, amount := range materials {
		if amount <= 0 || amount > 1000000 {
			return nil, rejected("invalid craft materials")
		}
		if required, known := rule.Materials[item]; known {
			if required != amount {
				return nil, rejected("craft material quantity does not match recipe")
			}
		} else {
			if !slices.Contains(rule.Optional, item) || optional > math.MaxInt64-amount {
				return nil, rejected("craft material does not match recipe")
			}
			unit[item] += amount
			optional += amount
		}
	}
	if optional != rule.OptionalCount {
		return nil, rejected("craft optional materials are incomplete")
	}
	unit[r.Constants.Items.Gold] += rule.Gold
	cost, err := multiplied(unit, int64(count))
	if err != nil {
		return nil, err
	}
	rewards, err := rewardItems(r, rule.Reward, int64(count))
	if err != nil {
		return nil, err
	}
	if err = a.exchange(r, cost, rewards); err != nil {
		return nil, err
	}
	a.RecordJourneyTaskEvent(r, 79, count, 0)
	return rewards, nil
}

func (a *Account) UseItem(r *Rules, id model.ItemID, count int64, selection int) (map[model.ItemID]int64, error) {
	rule, known := r.ItemRules[id]
	if !known || count < 1 || count > MaxBulkCount {
		return nil, rejected("invalid item use")
	}
	var quantities map[model.ItemID]int64
	var err error
	cost, multiplier := map[model.ItemID]int64{id: count}, count
	switch rule.Type {
	case 1008:
		quantities = map[model.ItemID]int64{r.Constants.Items.Energy: rule.Parameter}
	case 1017:
		quantities, err = rewardItems(r, model.RewardID(rule.Parameter), 1)
	case 1038:
		if selection < 0 || selection >= len(rule.Choices) {
			return nil, rejected("gift choice is invalid")
		}
		quantities, err = rewardItems(r, model.RewardID(rule.Choices[selection]), 1)
	case 1044:
		if selection < 0 || selection >= len(rule.Choices) || rule.Secondary < 0 || rule.Secondary > 0 && count > math.MaxInt64/rule.Secondary {
			return nil, rejected("chest choice is invalid")
		}
		quantities, err = rogueChestItems(r, []model.RewardID{model.RewardID(rule.Choices[selection])}, count)
		cost[r.Constants.Items.Energy] += rule.Secondary * count
		multiplier = 1
	case 1024:
		recipe := model.CraftID(rule.Parameter)
		if _, known := r.Crafts[recipe]; !known || slices.Contains(a.CraftRecipes, recipe) {
			return nil, rejected("craft recipe cannot be learned")
		}
		if err = a.Spend(map[model.ItemID]int64{id: 1}); err != nil {
			return nil, err
		}
		a.CraftRecipes = append(slices.Clone(a.CraftRecipes), recipe)
		slices.Sort(a.CraftRecipes)
		return map[model.ItemID]int64{}, nil
	case 1052:
		if selection < 0 || selection >= len(rule.Choices) || len(rule.Choices) != len(rule.Amounts) {
			return nil, rejected("item choice is invalid")
		}
		quantities = map[model.ItemID]int64{model.ItemID(rule.Choices[selection]): rule.Amounts[selection]}
	default:
		return nil, rejected("item cannot be consumed")
	}
	if err != nil {
		return nil, err
	}
	rewards, err := multiplied(quantities, multiplier)
	if err != nil {
		return nil, err
	}
	if err = a.exchange(r, cost, rewards); err != nil {
		return nil, err
	}
	return rewards, nil
}
