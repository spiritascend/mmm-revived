package account

import (
	"fmt"
	"math"

	"mmm-revived/internal/model"
)

func (s *Store) ConfigureRewardMultiplier(multiplier int) error {
	if multiplier < 1 {
		return fmt.Errorf("reward_multiplier must be at least 1, got %d", multiplier)
	}
	if s.Rules != nil {
		s.Rules.RewardMultiplier = multiplier
	}
	return nil
}

func (r *Rules) challengeRewards(ids []model.RewardID, count int64) (map[model.ItemID]int64, error) {
	multiplier := max(1, int64(r.RewardMultiplier))
	if count > math.MaxInt64/multiplier {
		return nil, rejected("challenge reward overflow")
	}
	items, err := droppedRewardItems(r, ids, count*multiplier)
	if err != nil || multiplier == 1 {
		return items, err
	}
	experience := r.Settings.Items
	for _, item := range [...]model.ItemID{experience.TeamExperience, experience.SeasonExperience, experience.PassExperience, experience.LimitedPassExperience} {
		if amount, ok := items[item]; ok {
			items[item] = amount / multiplier
		}
	}
	return items, nil
}
