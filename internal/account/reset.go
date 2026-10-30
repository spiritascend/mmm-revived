package account

import (
	"maps"
	"strconv"

	"mmm-revived/internal/model"
)

func talentRuleKey(id model.HeroID, slot, level int) string {
	return strconv.FormatInt(int64(id), 10) + "&" + strconv.Itoa(slot) + "&" + strconv.Itoa(level)
}

func (a *Account) HeroResetItems(r *Rules, id model.HeroID) (map[model.ItemID]int64, error) {
	hero, owned := a.Heroes[id]
	if !owned {
		return nil, rejected("hero is not owned")
	}
	items := map[model.ItemID]int64{r.Constants.Items.HeroExperience: hero.Experience}
	for level := 1; level < hero.Level; level++ {
		row, known := r.Levels[level]
		if !known {
			return nil, rejected("unknown hero level")
		}
		items[r.Constants.Items.HeroExperience] += row.Experience
		items[r.Constants.Items.Gold] += row.Money
	}
	breaks := r.Breaks[r.HeroBreakSets[id]]
	for tier := 1; tier <= hero.Break; tier++ {
		row, known := breaks[tier]
		if !known {
			return nil, rejected("unknown hero breakthrough")
		}
		addItems(items, row)
	}
	traits, known := r.HeroTraits[id]
	if !known {
		return nil, rejected("unknown hero")
	}
	for star := traits.Star; star < hero.Star; star++ {
		cost, known := r.HeroStars[star]
		if !known {
			return nil, rejected("unknown hero star")
		}
		addItems(items, cost)
	}
	for slot, maximum := range hero.Talents {
		for level := r.baseTalentLevel(id, slot) + 1; level <= maximum; level++ {
			row, known := r.HeroTalent(id, slot, level)
			if !known {
				return nil, rejected("unknown hero talent")
			}
			addItems(items, row.Cost)
		}
	}
	awakened, err := awakeRefund(r, hero.AwakeIDs)
	if err != nil {
		return nil, err
	}
	addItems(items, awakened)
	return items, nil
}

func (r *Rules) baseTalentLevel(id model.HeroID, slot int) int {
	if rule, known := r.HeroTalent(id, slot, 1); known && rule.Break == 0 && len(rule.Cost) == 0 {
		return 1
	}
	return 0
}

func awakeRefund(r *Rules, awakened []int) (map[model.ItemID]int64, error) {
	items := map[model.ItemID]int64{}
	for _, awake := range awakened {
		row, known := r.HeroAwake[awake]
		if !known || len(row.Cost) != 2 {
			return nil, rejected("unknown hero awakening")
		}
		items[model.ItemID(row.Cost[0])] += row.Cost[1]
	}
	return items, nil
}

func (a *Account) HighlightResetItems(r *Rules, id model.InstanceID) (map[model.ItemID]int64, error) {
	entry, owned := a.Highlights[id]
	template, known := r.HighlightTemplates[entry.ID]
	if !owned || !known {
		return nil, rejected("highlight is not owned")
	}
	items := map[model.ItemID]int64{r.Constants.Items.Gold: entry.Experience}
	for level := 1; level < entry.Level; level++ {
		row, known := r.HighlightLevels[highlightLevelKey(template.Rarity, level)]
		if !known {
			return nil, rejected("unknown highlight level")
		}
		items[r.Constants.Items.Gold] += row.Money
	}
	breaks := r.HighlightBreaks[template.BreakSet]
	for tier := 1; tier <= entry.Break; tier++ {
		row, known := breaks[tier]
		if !known {
			return nil, rejected("unknown highlight breakthrough")
		}
		addItems(items, row)
	}
	return items, nil
}

func (a *Account) ResetHeroAwake(r *Rules, id model.HeroID, now int64) (map[model.ItemID]int64, error) {
	hero, owned := a.Heroes[id]
	if !owned || len(hero.AwakeIDs) == 0 {
		return nil, rejected("hero has no awakening to reset")
	}
	items, err := awakeRefund(r, hero.AwakeIDs)
	if err != nil {
		return nil, err
	}
	next := *a
	next.Items = maps.Clone(a.Items)
	if !a.ResetActivityOpen(r, now) {
		if err := next.Spend(r.Constants.Constants.AwakeResetCost); err != nil {
			return nil, err
		}
	}
	if err := next.GrantItems(r, items); err != nil {
		return nil, err
	}
	hero.AwakeIDs = []int{}
	next.Heroes[id] = hero
	*a = next
	return items, nil
}

func (a *Account) AddProficiency(r *Rules, heroes []model.HeroID, experience int64) {
	if r == nil || experience <= 0 || experience > 1000000 {
		return
	}
	seen := make(map[model.HeroID]bool, len(heroes))
	for _, id := range heroes {
		hero, owned := a.Heroes[id]
		if !owned || seen[id] || hero.Trial != 0 || hero.ProficiencyLevel >= len(r.ProficiencyLevels) {
			continue
		}
		seen[id] = true
		hero.ProficiencyXP += experience
		for hero.ProficiencyLevel < len(r.ProficiencyLevels) {
			required := r.ProficiencyLevels[hero.ProficiencyLevel+1]
			if required <= 0 || hero.ProficiencyXP < required {
				break
			}
			hero.ProficiencyXP -= required
			hero.ProficiencyLevel++
		}
		a.Heroes[id] = hero
	}
}

func (a *Account) FreeResetHero(r *Rules, id model.HeroID) (map[model.ItemID]int64, error) {
	items, err := a.HeroResetItems(r, id)
	if err != nil {
		return nil, err
	}
	if err = a.GrantItems(r, items); err != nil {
		return nil, err
	}
	hero := a.Heroes[id]
	hero.Level, hero.Break, hero.Experience = 1, 0, 0
	hero.Star = min(hero.Star, r.HeroTraits[id].Star)
	talents := make(map[int]int, len(hero.Talents))
	for slot := range hero.Talents {
		if base := r.baseTalentLevel(id, slot); base > 0 {
			talents[slot] = base
		}
	}
	hero.Talents, hero.AwakeIDs = talents, []int{}
	a.Heroes[id] = hero
	return items, nil
}

func (a *Account) FreeResetHighlight(r *Rules, id model.InstanceID) (map[model.ItemID]int64, error) {
	items, err := a.HighlightResetItems(r, id)
	if err != nil {
		return nil, err
	}
	if err = a.GrantItems(r, items); err != nil {
		return nil, err
	}
	entry := a.Highlights[id]
	entry.Level, entry.Break, entry.Experience = 1, 0, 0
	a.Highlights[id] = entry
	return items, nil
}

const freeResetActivityType = 4

func (r *Rules) FreeResetActivity() (model.ActivityID, JourneyActivityRule, bool) {
	return r.ActivityOfType(freeResetActivityType)
}

func (a *Account) ResetActivityOpen(r *Rules, now int64) bool {
	_, rule, known := r.FreeResetActivity()
	return known && a.Level >= rule.Level && now >= a.CreatedAt && LogicDay(now)-LogicDay(a.CreatedAt) <= int64(rule.Duration)
}
