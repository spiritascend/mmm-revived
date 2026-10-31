package account

import (
	"errors"
	"fmt"

	"mmm-revived/internal/model"
)

func (s *Store) ConfigureTeamSize(size int) error {
	if size < 1 {
		return fmt.Errorf("team_size must be at least 1, got %d", size)
	}
	if s.Rules == nil {
		return nil
	}
	if s.Rules.Constants.FormationSize <= 0 {
		return errors.New("team_size: constant_rules.json has no formation_size")
	}
	s.Rules.TeamSize = size
	return nil
}

func (r *Rules) BattleHeroLimit(id model.BattleID) (int, bool) {
	return r.battleHeroLimit(id, r.TeamSize)
}

func (a *Account) BattleHeroLimit(r *Rules, id model.BattleID) (int, bool) {
	return r.battleHeroLimit(id, a.battleTeamSize(r))
}

func (r *Rules) battleHeroLimit(id model.BattleID, teamSize int) (int, bool) {
	if r == nil {
		return 0, false
	}
	rule := r.BattleSettings[id]
	if teamSize <= 0 || teamSize == rule.Heroes || rule.Heroes != r.Constants.FormationSize || rule.SkipDeploy || r.RogueBattle(id) {
		return rule.Heroes, false
	}
	return teamSize, true
}
