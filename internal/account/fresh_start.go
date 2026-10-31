package account

import (
	"maps"
	"slices"
	"time"

	"mmm-revived/internal/calendar"
	"mmm-revived/internal/model"
)

const DaySeconds = calendar.DaySeconds

func LogicDay(now int64) int64 { return calendar.LogicDay(now) }

func LogicWeek(now int64) int64 { return calendar.LogicWeek(now) }

func LogicMonth(now int64) int64 { return calendar.LogicMonth(now) }

func NewRetailAccount(uid model.UserID, name string, createdAt int64, r *Rules, catalog Catalog) Account {
	start := r.Defaults.Retail
	a := Account{
		Retail:     true,
		UID:        uid,
		Name:       name,
		TeamName:   name,
		CreatedAt:  createdAt,
		LastLogin:  createdAt,
		Level:      max(1, start.Level),
		Items:      map[model.ItemID]int64{r.Constants.Items.Energy: start.Energy},
		Heroes:     map[model.HeroID]model.Hero{},
		Guides:     map[string]string{},
		Artifacts:  map[model.InstanceID]model.Artifact{},
		Highlights: map[model.InstanceID]model.Highlight{},
		Energy:     model.PlayerEnergy{Initialized: true, Day: LogicDay(createdAt), Refreshed: createdAt},
	}
	for _, id := range start.Heroes {
		template, known := catalog.Heroes[id]
		if !known {
			continue
		}
		a.Heroes[id] = model.Hero{
			ID:         id,
			Level:      DefaultHeroLevel,
			Star:       template.Star,
			Talents:    maps.Clone(template.Talents),
			AwakeIDs:   []int{},
			Artifacts:  map[int]model.InstanceID{},
			Highlights: map[int]model.InstanceID{},
		}
		a.Profile.ShownHeroes = append(a.Profile.ShownHeroes, id)
	}
	if len(a.Profile.ShownHeroes) > 0 {
		a.Profile.Lobby.Hero = a.Profile.ShownHeroes[0]
	}
	a.Profile.Head, a.Profile.Frame, a.Profile.Background = start.Head, start.Frame, start.Background
	return a
}

func (a *Account) BeginRetail(r *Rules) error {
	start := r.Defaults.Retail
	if err := a.GrantItems(r, start.Items); err != nil {
		return err
	}
	if rule, known := r.Journey.NoviceTasks[start.NoviceTask]; known {
		a.Journey.Novice.Task = start.NoviceTask
		a.openNoviceNodes(rule.Nodes)
	}
	return nil
}

func (a *Account) RegenerateEnergy(r *Rules, now int64) bool {
	if !a.Retail || r == nil {
		return false
	}
	start := r.Defaults.Retail
	every, bankEvery := r.energyInterval(start.EnergySeconds), r.energyInterval(start.BankSeconds)
	if every <= 0 {
		return false
	}
	energy, bank, refreshed := a.Items[r.Constants.Items.Energy], a.Energy.Bank, a.Energy.Refreshed
	if a.Economy(r).InfiniteEnergy {
		a.Items[r.Constants.Items.Energy] = max(energy, r.TeamLevels[a.Level].EnergyLimit)
		return energy != a.Items[r.Constants.Items.Energy]
	}
	if a.Energy.Refreshed <= 0 || a.Energy.Refreshed > now {
		a.Energy.Refreshed = now
	}
	limit := r.TeamLevels[a.Level].EnergyLimit
	if a.Items[r.Constants.Items.Energy] < limit {
		gained := min((now-a.Energy.Refreshed)/every, limit-a.Items[r.Constants.Items.Energy])
		a.Items[r.Constants.Items.Energy] += gained
		a.Energy.Refreshed += gained * every
	}
	if a.Items[r.Constants.Items.Energy] >= limit {
		if bankEvery > 0 && a.Energy.Bank < start.BankLimit {
			gained := min((now-a.Energy.Refreshed)/bankEvery, start.BankLimit-a.Energy.Bank)
			a.Energy.Bank += gained
			a.Energy.Refreshed += gained * bankEvery
		}
		if a.Energy.Bank >= start.BankLimit {
			a.Energy.Refreshed = now
		}
	}
	return energy != a.Items[r.Constants.Items.Energy] || bank != a.Energy.Bank || refreshed != a.Energy.Refreshed
}

func (a *Account) levelTeam(r *Rules) {
	from := a.Level
	for {
		current, known := r.TeamLevels[a.Level]
		if _, exists := r.TeamLevels[a.Level+1]; !known || !exists || a.Retail && current.Break > a.TeamBreak {
			break
		}
		if current.Experience <= 0 && current.Break > 0 {
			a.Level++
			continue
		}
		if current.Experience <= 0 || a.Experience < current.Experience {
			break
		}
		a.Experience -= current.Experience
		a.Level++
	}
	if a.Retail {
		for level := from + 1; level <= a.Level; level++ {
			a.Items[r.Constants.Items.Energy] += r.TeamLevels[level].EnergyRecover
		}
	} else {
		a.TeamBreak = TeamBreakAt(r, a.Level)
	}
}

func TeamBreakAt(r *Rules, level int) int {
	result := 0
	for current := 1; current <= level; current++ {
		result = max(result, r.TeamLevels[current].Break)
	}
	return result
}

func LevelEnergyRecovery(r *Rules, from, to int) int64 {
	var total int64
	for level := from + 1; level <= to; level++ {
		total += r.TeamLevels[level].EnergyRecover
	}
	return total
}

func (a *Account) FinishTeamBreak(r *Rules, battle model.BattleID) (map[model.ItemID]int64, bool, error) {
	if !a.Retail || r == nil {
		return nil, false, nil
	}
	tag := r.TeamLevels[a.Level].Break
	rule, known := r.Defaults.Retail.TeamBreaks[tag]
	if !known || tag <= a.TeamBreak || rule.Battle != battle {
		return nil, false, nil
	}
	if rule.Task != 0 && a.Tasks[rule.Task].Status < 3 {
		return nil, false, rejected("team breakthrough is locked")
	}
	items := map[model.ItemID]int64{}
	if rule.Reward != 0 {
		var err error
		if items, err = rewardItems(r, rule.Reward, 1); err != nil {
			return nil, false, err
		}
	}
	a.TeamBreak = tag
	if err := a.GrantItems(r, items); err != nil {
		return nil, false, err
	}
	return items, true, nil
}

func (a *Account) resetRetailDaily(r *Rules) {
	daily := r.Defaults.Retail.Daily
	if daily.PointItem != 0 {
		a.Items[daily.PointItem] = 0
	}
	for _, id := range daily.PointTasks {
		a.Tasks[id] = model.Task{Status: 2}
	}
	selected := map[model.TaskID]bool{}
	if a.Level < daily.NewbieLevel {
		for _, id := range daily.NewbieTasks {
			selected[id] = true
		}
	} else {
		for _, kind := range slices.Sorted(maps.Keys(daily.Counts)) {
			candidates := []RetailDailyTask{}
			for _, task := range daily.Tasks {
				if task.Type == kind && task.Level <= a.Level && task.Weight > 0 {
					candidates = append(candidates, task)
				}
			}
			for range daily.Counts[kind] {
				var total int64
				for _, task := range candidates {
					total += task.Weight
				}
				if total <= 0 {
					break
				}
				roll, err := secureIntN(total)
				if err != nil {
					break
				}
				for index, task := range candidates {
					if roll -= task.Weight; roll < 0 {
						selected[task.Task] = true
						candidates = slices.Delete(candidates, index, index+1)
						break
					}
				}
			}
		}
	}
	for id, rule := range r.Tasks {
		if rule.Daily && !selected[id] {
			a.Tasks[id] = model.Task{Status: 5}
		}
	}
}

func (a *Account) retailCosmetics(c Catalog, kind int, initial model.ItemID) []model.ItemID {
	owned := []model.ItemID{initial}
	for _, id := range c.Cosmetics[kind] {
		if id != initial && a.Items[id] > 0 {
			owned = append(owned, id)
		}
	}
	return owned
}

func (a *Account) OwnsCosmetic(c Catalog, kind int, item model.ItemID) bool {
	return a.ownsPermanentCosmetic(c, kind, item) || a.ownsTimedCosmetic(c, kind, item, time.Now().Unix())
}

func (a *Account) RollDay(r *Rules, now int64) error {
	a.AdvanceCalendar(r, now)
	a.InitializeJourney(r, now)
	a.InitializeEvents(r, now)
	a.InitializeRogue(r, now)
	a.PrepareNetwork(r.Network, now)
	a.RefreshTasks(r)
	return a.PrepareDispatch(r, now)
}
