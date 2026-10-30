package account

import (
	"crypto/rand"
	"maps"
	"math"
	"math/big"
	"slices"
	"strconv"
	"time"

	"mmm-revived/internal/model"
)

func secureIntN(n int64) (int64, error) {
	value, err := rand.Int(rand.Reader, big.NewInt(n))
	if err != nil {
		return 0, err
	}
	return value.Int64(), nil
}

func randomChoice[T any](values []T) (T, error) {
	var zero T
	if len(values) == 0 {
		return zero, rejected("reward pool is empty")
	}
	index, err := secureIntN(int64(len(values)))
	if err != nil {
		return zero, err
	}
	return values[index], nil
}

func (a *Account) instanceID() model.InstanceID {
	for {
		a.NextInstance++
		id := model.InstanceID(strconv.FormatInt(a.NextInstance, 10))
		if _, exists := a.Artifacts[id]; exists {
			continue
		}
		if _, exists := a.Highlights[id]; !exists {
			return id
		}
	}
}

func (a *Account) grantRefund(r *Rules, rule ItemRule, count int64, depth int) error {
	refund, err := multiplied(rule.Refund, count)
	if err != nil {
		return err
	}
	return a.grantItems(r, refund, depth+1)
}

func (a *Account) grantItems(r *Rules, quantities map[model.ItemID]int64, depth int) error {
	if depth > 4 {
		return rejected("reward recursion limit exceeded")
	}
	for item, amount := range quantities {
		if item <= 0 || amount < 0 || a.Items[item] < 0 || a.Items[item] > math.MaxInt64-amount {
			return rejected("invalid reward quantity")
		}
		rule := r.ItemRules[item]
		if (rule.Type == 1013 || rule.Type == 1014 || rule.Type == 1015 || rule.Type == 1021 || rule.Type == 1029 || rule.Type == 1036 || rule.Type == 1053) && amount > 1000 {
			return rejected("too many equipment rewards")
		}
	}
	for item, amount := range quantities {
		if amount == 0 {
			continue
		}
		rule := r.ItemRules[item]
		switch rule.Type {
		case 1002:
			hero := model.HeroID(rule.Parameter)
			if current, owned := a.Heroes[hero]; owned {
				if current.Trial != 0 {
					current.Trial = 0
					a.Heroes[hero] = current
					amount--
					if err := a.grantStarReward(r, hero, depth); err != nil {
						return err
					}
				}
				if err := a.grantRefund(r, rule, amount, depth); err != nil {
					return err
				}
				continue
			}
			traits, known := r.HeroTraits[hero]
			if !known {
				return rejected("unknown rewarded hero")
			}
			star := traits.Star
			if star == 0 {
				star = 3
			}
			a.Heroes[hero] = model.Hero{ID: hero, Level: 1, Star: star, Talents: map[int]int{1: 1, 2: 1, 3: 1, 4: 1, 5: 1}, AwakeIDs: []int{}, Artifacts: map[int]model.InstanceID{}, Highlights: map[int]model.InstanceID{}}
			if err := a.grantStarReward(r, hero, depth); err != nil {
				return err
			}
			if amount > 1 {
				if err := a.grantRefund(r, rule, amount-1, depth); err != nil {
					return err
				}
			}
		case 1013, 1014:
			candidates := []model.ArtifactID{}
			exact := model.ArtifactID(int64(rule.Rarity)*1000 + rule.Secondary)
			if artifact, known := r.Artifacts[exact]; rule.Secondary != 0 && known && !artifact.Unreleased {
				candidates = append(candidates, exact)
			} else {
				for id, artifact := range r.Artifacts {
					if artifact.Star == rule.Rarity && !artifact.Unreleased {
						candidates = append(candidates, id)
					}
				}
				slices.Sort(candidates)
			}
			if len(candidates) == 0 {
				return rejected("artifact reward has no valid template")
			}
			for range amount {
				id, err := randomChoice(candidates)
				if err != nil {
					return err
				}
				if err = a.createArtifact(r, id); err != nil {
					return err
				}
			}
		case 1029:
			candidates := []model.ArtifactID{}
			for id, artifact := range r.Artifacts {
				if slices.Contains(artifact.Pools, item) && !artifact.Unreleased {
					candidates = append(candidates, id)
				}
			}
			slices.Sort(candidates)
			for range amount {
				pool := candidates
				if available := slices.DeleteFunc(slices.Clone(candidates), func(id model.ArtifactID) bool { return r.Artifacts[id].Unique && a.ownsArtifact(id) }); len(available) > 0 {
					pool = available
				}
				id, err := randomChoice(pool)
				if err != nil {
					return err
				}
				if err = a.createArtifact(r, id); err != nil {
					return err
				}
			}
		case 1036:
			var classified, candidates []model.HighlightID
			for id, template := range r.HighlightTemplates {
				if int64(template.Rarity) != rule.Parameter || template.Unreleased {
					continue
				}
				candidates = append(candidates, id)
				if template.Classified {
					classified = append(classified, id)
				}
			}
			if len(classified) > 0 {
				candidates = classified
			}
			slices.Sort(candidates)
			for range amount {
				id, err := randomChoice(candidates)
				if err != nil {
					return err
				}
				a.Highlights[a.instanceID()] = model.Highlight{ID: id, Level: 1, Obtained: time.Now().Unix()}
			}
		case 1021, 1053:
			for range amount {
				hero, err := a.randomExchangeHero(r, item, rule.Type == 1053)
				if err != nil {
					return err
				}
				if err = a.grantItems(r, map[model.ItemID]int64{model.ItemID(hero): 1}, depth+1); err != nil {
					return err
				}
			}
		case 1015:
			id := model.HighlightID(rule.Parameter)
			if template, known := r.HighlightTemplates[id]; !known || template.Unreleased {
				return rejected("unknown highlight reward")
			}
			for range amount {
				a.Highlights[a.instanceID()] = model.Highlight{ID: id, Level: 1, Obtained: time.Now().Unix()}
			}
		case limitedPassItemType:
			if !a.activateLimitedPass(r, model.ActivityID(rule.Parameter)) {
				a.Items[item] += amount
			}
		case timedItemType:
			if !a.grantTimedCosmetic(r, rule, amount, time.Now().Unix()) {
				a.Items[item] += amount
			}
		case 1032:
			if !a.Retail {
				a.Items[item] += amount
				continue
			}
			if !a.ownsSkin(r, model.SkinID(rule.Parameter)) {
				a.Items[item] = 1
				amount--
			}
			if amount > 0 {
				if err := a.grantRefund(r, rule, amount, depth); err != nil {
					return err
				}
			}
		default:
			a.Items[item] += amount
		}
	}
	return nil
}

func (a *Account) createArtifact(r *Rules, id model.ArtifactID) error {
	template, err := randomChoice(r.Artifacts[id].Templates)
	if err != nil {
		return err
	}
	artifact := model.Artifact{ID: id, Level: 1, Template: template, PendingGrowth: map[string][]int64{}, Obtained: time.Now().Unix()}
	if len(r.ArtifactTemplates) > 0 {
		if err := initializeArtifactGrowth(r, &artifact); err != nil {
			return err
		}
	}
	a.Artifacts[a.instanceID()] = artifact
	return nil
}

func (a *Account) ownsArtifact(id model.ArtifactID) bool {
	for _, artifact := range a.Artifacts {
		if artifact.ID == id {
			return true
		}
	}
	return false
}

func (a *Account) randomExchangeHero(r *Rules, item model.ItemID, unique bool) (model.HeroID, error) {
	gained := a.LimitedHeroes[item]
	pool := r.ExchangeHeroes
	if unique {
		if remaining := slices.DeleteFunc(slices.Clone(pool), func(id model.HeroID) bool { return slices.Contains(gained, id) }); len(remaining) > 0 {
			pool = remaining
		}
	}
	hero, err := randomChoice(pool)
	if err != nil {
		return 0, err
	}
	if rule := r.ItemRules[model.ItemID(hero)]; rule.Type != 1002 || model.HeroID(rule.Parameter) != hero {
		return 0, rejected("random hero has no hero item")
	}
	if unique && !slices.Contains(gained, hero) {
		if a.LimitedHeroes == nil {
			a.LimitedHeroes = map[model.ItemID][]model.HeroID{}
		}
		a.LimitedHeroes[item] = append(slices.Clip(gained), hero)
	}
	return hero, nil
}

func limitedItems(gained map[model.ItemID][]model.HeroID) map[model.ItemID]model.LimitedItem {
	result := make(map[model.ItemID]model.LimitedItem, len(gained))
	for item, heroes := range gained {
		result[item] = model.LimitedItem{HeroGain: heroes}
	}
	return result
}

func multiplied(values map[model.ItemID]int64, count int64) (map[model.ItemID]int64, error) {
	if count < 0 {
		return nil, rejected("negative quantity")
	}
	result := make(map[model.ItemID]int64, len(values))
	for id, amount := range values {
		if amount < 0 || amount != 0 && count > math.MaxInt64/amount {
			return nil, rejected("quantity overflow")
		}
		result[id] = amount * count
	}
	return result, nil
}

func (a *Account) GrantItems(r *Rules, quantities map[model.ItemID]int64) error {
	if r == nil {
		r = &Rules{}
	}
	if a.Experience > math.MaxInt64-quantities[r.Settings.Items.TeamExperience] {
		return rejected("experience overflow")
	}
	next := *a
	next.Items, next.Heroes, next.Artifacts, next.Highlights, next.Tasks = maps.Clone(a.Items), maps.Clone(a.Heroes), maps.Clone(a.Artifacts), maps.Clone(a.Highlights), maps.Clone(a.Tasks)
	next.LimitedHeroes, next.TimedCosmetics = maps.Clone(a.LimitedHeroes), maps.Clone(a.TimedCosmetics)
	if next.Artifacts == nil {
		next.Artifacts = map[model.InstanceID]model.Artifact{}
	}
	if next.Highlights == nil {
		next.Highlights = map[model.InstanceID]model.Highlight{}
	}
	if err := next.grantItems(r, quantities, 0); err != nil {
		return err
	}
	next.Experience += quantities[r.Settings.Items.TeamExperience]
	next.levelTeam(r)
	now := time.Now().Unix()
	if err := next.AddSeasonExperience(r, quantities[r.Settings.Items.SeasonExperience]); err != nil {
		return err
	}
	if err := next.AddJourneyPassExperience(r, quantities[r.Settings.Items.PassExperience], false, now); err != nil {
		return err
	}
	if err := next.AddLimitedJourneyPassExperience(r, quantities[r.Settings.Items.LimitedPassExperience], now); err != nil {
		return err
	}
	if err := next.AddJourneyHyperPoints(r, quantities[r.Settings.Items.HyperflowPoints], now); err != nil {
		return err
	}
	if quantities[r.Settings.Items.PassActivation] > 0 {
		if err := next.ActivateJourneyPass(r, 1, now); err != nil {
			return err
		}
	}
	if quantities[r.Settings.Items.PremiumPassActivation] > 0 {
		if err := next.ActivateJourneyPass(r, 3, now); err != nil {
			return err
		}
	}
	for range quantities[r.Settings.Items.MonthlyPass] {
		if err := next.ActivateMonthlyPass(r, now); err != nil {
			return err
		}
	}
	next.RefreshTasks(r)
	*a = next
	return nil
}

func rewardItems(r *Rules, id model.RewardID, count int64) (map[model.ItemID]int64, error) {
	rows, known := r.Rewards[id]
	if !known {
		return nil, rejected("unknown reward")
	}
	quantities := make(map[model.ItemID]int64, len(rows))
	for _, row := range rows {
		if row[0] <= 0 || row[0] > math.MaxInt32 || row[1] < 0 || !addQuantity(quantities, model.ItemID(row[0]), row[1]) {
			return nil, rejected("invalid reward quantity")
		}
	}
	return multiplied(quantities, count)
}

const dropChancePer = 10000

func droppedRewardItems(r *Rules, ids []model.RewardID, count int64) (map[model.ItemID]int64, error) {
	result := map[model.ItemID]int64{}
	for _, id := range ids {
		if id == 0 {
			continue
		}
		quantities, err := rewardItems(r, id, count)
		if err != nil {
			return nil, err
		}
		for _, row := range r.Rewards[id] {
			if row[1] != 0 {
				continue
			}
			item := model.ItemID(row[0])
			chance := r.DropTierChances[r.DropDetails[id][item]]
			for range count {
				roll, err := secureIntN(dropChancePer)
				if err != nil {
					return nil, err
				}
				if roll < chance {
					quantities[item]++
				}
			}
		}
		for item, amount := range quantities {
			if amount > 0 && !addQuantity(result, item, amount) {
				return nil, rejected("reward overflow")
			}
		}
	}
	return result, nil
}

func RewardQuantities(r *Rules, ids ...model.RewardID) map[model.ItemID]int64 {
	quantities := map[model.ItemID]int64{}
	if r == nil {
		return quantities
	}
	for _, id := range ids {
		items, err := rewardItems(r, id, 1)
		if err != nil {
			continue
		}
		for item, amount := range items {
			addQuantity(quantities, item, amount)
		}
	}
	return quantities
}
