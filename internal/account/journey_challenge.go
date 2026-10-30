package account

import (
	"maps"
	"math"
	"math/bits"
	"slices"
	"time"

	"mmm-revived/internal/model"
)

func (a *Account) journeyChallengeRule(r *Rules, id, count int, sweep bool) (JourneyChallengeRule, error) {
	rule, known := r.Journey.Challenges[id]
	record := a.Journey.Challenges[id]
	maximum := 1
	if multipliers := r.Constants.Constants.ChallengeStarMultipliers; record.Stars > 0 && len(multipliers) > 0 {
		maximum = multipliers[min(record.Stars, len(multipliers))-1]
	}
	unlocked := rule.Difficulty == 1 || a.Level >= rule.Level
	if !unlocked && rule.Predecessor > 0 && a.Journey.Challenges[rule.Predecessor].Stars >= 3 {
		unlocked = int64(rule.Level)-int64(a.Level) <= int64(max(0, r.Journey.ChallengeEarlyLevelMargin))
	}
	if !known || rule.Active != 1 || !unlocked || count < 1 || count > maximum || sweep && record.Stars < 3 {
		return rule, rejected("challenge is locked or multiplier unavailable")
	}
	if rule.Energy < 0 || rule.Energy > math.MaxInt64/int64(count) {
		return rule, rejected("invalid challenge energy")
	}
	return rule, nil
}

func (a *Account) BeginJourneyChallenge(r *Rules, id int, battle model.BattleID, count int) error {
	rule, err := a.journeyChallengeRule(r, id, count, false)
	if err != nil {
		return err
	}
	if rule.Battle != battle {
		return rejected("challenge battle does not match")
	}
	if a.doubleMaterialGroup(r, rule.Group) {
		if _, err = a.doubleMaterialChances(r, rule.Group, count, time.Now().Unix()); err != nil {
			return err
		}
	}
	if err = a.Spend(map[model.ItemID]int64{r.Constants.Items.Energy: rule.Energy * int64(count)}); err != nil {
		return err
	}
	a.Journey.ChallengeRun = &model.JourneyChallengeRun{ID: id, Battle: battle, Count: count}
	return nil
}

func (a *Account) energyExperience(energy int64) int64 {
	if !a.Retail {
		return 0
	}
	return max(0, energy)
}

func (a *Account) grantEnergyExperience(r *Rules, energy int64) error {
	if experience := a.energyExperience(energy); experience > 0 {
		return a.GrantItems(r, map[model.ItemID]int64{r.Settings.Items.TeamExperience: experience})
	}
	return nil
}

type ChallengeStarReward struct {
	Star  int
	Items map[model.ItemID]int64
}

func (a *Account) FinishJourneyChallenge(r *Rules, id int, battle model.BattleID, won bool, bitmap int, clearTime int64, kills int) (map[model.ItemID]int64, []ChallengeStarReward, error) {
	run := a.Journey.ChallengeRun
	if run == nil || run.ID != id || run.Battle != battle || bitmap < 0 || bitmap > 7 || clearTime < 0 || clearTime > 86400 || kills < 0 || kills > 1000000 {
		return nil, nil, rejected("challenge result does not match active run")
	}
	rule, known := r.Journey.Challenges[id]
	if !known || rule.Battle != battle {
		return nil, nil, rejected("unknown challenge")
	}
	if !won {
		if err := a.grantEnergyExperience(r, rule.Energy*int64(run.Count)); err != nil {
			return nil, nil, err
		}
		a.Journey.ChallengeRun = nil
		return map[model.ItemID]int64{}, nil, nil
	}
	record := a.Journey.Challenges[id]
	rewards, err := r.challengeRewards(rule.Rewards, int64(run.Count))
	if err != nil {
		return nil, nil, err
	}
	if rewards, err = a.consumeDoubleMaterial(r, rule.Group, run.Count, rewards); err != nil {
		return nil, nil, err
	}
	stars := max(record.Stars, bits.OnesCount(uint(bitmap)))
	extra := map[model.ItemID]int64{}
	var first []ChallengeStarReward
	for index, reward := range rule.StarRewards[:min(stars, len(rule.StarRewards))] {
		if record.StarRewards&(1<<index) != 0 {
			continue
		}
		record.StarRewards |= 1 << index
		items, err := droppedRewardItems(r, []model.RewardID{reward}, 1)
		if err != nil {
			return nil, nil, err
		}
		if len(items) == 0 {
			continue
		}
		first = append(first, ChallengeStarReward{Star: index + 1, Items: items})
		for item, count := range items {
			if !addQuantity(extra, item, count) {
				return nil, nil, rejected("challenge reward overflow")
			}
		}
	}
	if experience := a.energyExperience(rule.Energy * int64(run.Count)); experience > 0 {
		extra[r.Settings.Items.TeamExperience] += experience
	}
	for item, count := range extra {
		if rewards[item] > math.MaxInt64-count {
			return nil, nil, rejected("challenge reward overflow")
		}
		rewards[item] += count
	}
	if err = a.GrantItems(r, rewards); err != nil {
		return nil, nil, err
	}
	record.Stars = stars
	record.Bitmap |= bitmap
	if record.Time == 0 || clearTime > 0 && clearTime < record.Time {
		record.Time = clearTime
	}
	record.Kills = max(record.Kills, kills)
	a.Journey.Challenges[id] = record
	a.Journey.ChallengeRun = nil
	return rewards, first, nil
}

func (a *Account) SweepJourneyChallenge(r *Rules, id, count int) (map[model.ItemID]int64, error) {
	rule, err := a.journeyChallengeRule(r, id, count, true)
	if err != nil {
		return nil, err
	}
	quantities, err := r.challengeRewards(rule.Rewards, int64(count))
	if err != nil {
		return nil, err
	}
	if quantities, err = a.consumeDoubleMaterial(r, rule.Group, count, quantities); err != nil {
		return nil, err
	}
	energy := r.Constants.Items.Energy
	cost := map[model.ItemID]int64{energy: rule.Energy * int64(count), r.Constants.Constants.SweepChallengeItem: int64(count)}
	next := *a
	next.Items = maps.Clone(a.Items)
	if err = next.Spend(cost); err != nil {
		return nil, err
	}
	if experience := a.energyExperience(cost[energy]); experience > 0 {
		quantities[r.Settings.Items.TeamExperience] += experience
	}
	if err = next.GrantItems(r, quantities); err != nil {
		return nil, err
	}
	next.RecordJourneyTaskEvent(r, 2, count, r.challengeTaskParameter(rule.Group))
	*a = next
	return quantities, nil
}

func (a *Account) BeginJourneySpiral(r *Rules, mission int, battle model.BattleID) error {
	rule, known := r.Journey.SpiralMissions[mission]
	if !known || rule.Battle != battle || rule.Predecessor > 0 && a.Journey.Spiral[rule.Predecessor]&1 == 0 {
		return rejected("spiral mission is locked")
	}
	return nil
}

func (a *Account) FinishJourneySpiral(r *Rules, mission int, battle model.BattleID, bitmap int) (map[model.ItemID]int64, error) {
	rule, known := r.Journey.SpiralMissions[mission]
	if !known || rule.Battle != battle || bitmap < 0 || bitmap > 7 {
		return nil, rejected("invalid spiral result")
	}
	old := a.Journey.Spiral[mission]
	var rewards []model.RewardID
	for index, reward := range rule.Rewards {
		if bitmap&(1<<index) != 0 && old&(1<<index) == 0 {
			rewards = append(rewards, reward)
		}
	}
	quantities, err := mergeJourneyRewards(r, rewards)
	if err != nil {
		return nil, err
	}
	if err = a.GrantItems(r, quantities); err != nil {
		return nil, err
	}
	a.Journey.Spiral[mission] = old | bitmap
	for group, row := range r.Journey.SpiralGroups {
		if slices.Contains(row.Missions, mission) {
			if _, exists := a.Journey.SpiralRewards[group]; !exists {
				a.Journey.SpiralRewards[group] = []int{}
			}
		}
	}
	return quantities, nil
}

func (a *Account) ClaimJourneySpiralStars(r *Rules, group, star int) (map[model.ItemID]int64, error) {
	rule, known := r.Journey.SpiralGroups[group]
	reward, rewardKnown := rule.Rewards[star]
	if !known || !rewardKnown || star <= 0 {
		return nil, rejected("spiral group reward is not configured")
	}
	score := 0
	for _, mission := range rule.Missions {
		score += bits.OnesCount(uint(a.Journey.Spiral[mission] & 7))
	}
	if score < star {
		return nil, rejected("spiral star target is incomplete")
	}
	if slices.Contains(a.Journey.SpiralRewards[group], star) {
		return map[model.ItemID]int64{}, nil
	}
	quantities, err := rewardItems(r, reward, 1)
	if err != nil {
		return nil, err
	}
	if err = a.GrantItems(r, quantities); err != nil {
		return nil, err
	}
	a.Journey.SpiralRewards[group] = append(a.Journey.SpiralRewards[group], star)
	return quantities, nil
}
