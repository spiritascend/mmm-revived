package account

import (
	"maps"
	"slices"

	"mmm-revived/internal/model"
)

func (a *Account) StarHero(r *Rules, id model.HeroID) error {
	hero, ok := a.Heroes[id]
	cost, known := r.HeroStars[hero.Star]
	if !ok || !known || hero.Trial == 1 || hero.Level < r.Constants.Constants.HeroStarUnlockLevel {
		return rejected("hero cannot gain a star")
	}
	next := *a
	next.Items, next.Heroes = maps.Clone(a.Items), maps.Clone(a.Heroes)
	if err := next.Spend(cost); err != nil {
		return err
	}
	hero.Star++
	next.Heroes[id] = hero
	if hero.Star == 3 {
		if err := next.grantStarReward(r, id, 0); err != nil {
			return err
		}
	}
	next.Tasks = maps.Clone(next.Tasks)
	next.RecordJourneyTaskEvent(r, 31, 1, hero.Star)
	next.RefreshTasks(r)
	*a = next
	return nil
}

func (a *Account) grantStarReward(r *Rules, id model.HeroID, depth int) error {
	reward := r.HeroStarReward(id)
	if hero, owned := a.Heroes[id]; !owned || reward == 0 || hero.Trial != 0 || hero.Star < 3 {
		return nil
	}
	items, err := rewardItems(r, reward, 1)
	if err != nil {
		return err
	}
	for item := range items {
		if a.Items[item] > 0 {
			delete(items, item)
		}
	}
	return a.grantItems(r, items, depth+1)
}

func (a *Account) TalentHero(r *Rules, id model.HeroID, slot int) error {
	hero, ok := a.Heroes[id]
	level := hero.Talents[slot] + 1
	rule, known := r.HeroTalent(id, slot, level)
	if !ok || !known || hero.Break < rule.Break || hero.Trial == 1 && level > r.Constants.Constants.TrialHeroMaxTalent {
		return rejected("hero talent cannot be upgraded")
	}
	if err := a.Spend(rule.Cost); err != nil {
		return err
	}
	if hero.Talents == nil {
		hero.Talents = map[int]int{}
	}
	hero.Talents[slot] = level
	a.Heroes[id] = hero
	return nil
}

func (a *Account) AwakeHero(r *Rules, id model.HeroID, awake int) error {
	hero, ok := a.Heroes[id]
	rule, known := r.HeroAwake[awake]
	if !ok || !known || hero.Trial == 1 || awake/10 != int(id) || slices.Contains(hero.AwakeIDs, awake) || len(rule.Cost) != 2 || rule.Cost[1] < 0 {
		return rejected("invalid hero awakening")
	}
	for _, required := range rule.Prerequisites {
		if !slices.Contains(hero.AwakeIDs, required) {
			return rejected("awakening prerequisite is incomplete")
		}
	}
	item, needed := model.ItemID(rule.Cost[0]), rule.Cost[1]
	cost := map[model.ItemID]int64{item: needed}
	if owned := max(a.Items[item], 0); owned < needed {
		if substitute, known := r.Constants.Constants.HeroStarAwakeItems[r.ItemRules[item].Rarity]; known && substitute != item {
			cost[item], cost[substitute] = owned, needed-owned
		}
	}
	if err := a.Spend(cost); err != nil {
		return err
	}
	hero.AwakeIDs = append(hero.AwakeIDs, awake)
	a.Heroes[id] = hero
	return nil
}

func (a *Account) InstallArtifact(r *Rules, heroID model.HeroID, instance model.InstanceID, slot int) error {
	hero, owned := a.Heroes[heroID]
	artifact, exists := a.Artifacts[instance]
	rule, known := r.Artifacts[artifact.ID]
	if !owned || !exists || !known || slot < 1 || slot > 2 {
		return rejected("invalid artifact installation")
	}
	if other, equipped := a.Artifacts[hero.Artifacts[3-slot]]; equipped {
		if other.ID%1000 == artifact.ID%1000 || rule.Refinable && r.Artifacts[other.ID].Refinable {
			return rejected("artifact conflicts with the other slot")
		}
	}
	for _, entry := range a.Heroes {
		maps.DeleteFunc(entry.Artifacts, func(_ int, equipped model.InstanceID) bool { return equipped == instance })
	}
	if hero.Artifacts == nil {
		hero.Artifacts = map[int]model.InstanceID{}
	}
	if previous := hero.Artifacts[slot]; previous != "" && previous != instance {
		a.setArtifactInstall(previous, 0)
	}
	hero.Artifacts[slot] = instance
	artifact.Install = heroID
	a.Artifacts[instance] = artifact
	a.Heroes[heroID] = hero
	return nil
}

func (a *Account) setArtifactInstall(instance model.InstanceID, hero model.HeroID) {
	artifact := a.Artifacts[instance]
	artifact.Install = hero
	a.Artifacts[instance] = artifact
}

func (a *Account) setHighlightInstall(instance model.InstanceID, hero model.HeroID) {
	highlight := a.Highlights[instance]
	highlight.Install = hero
	a.Highlights[instance] = highlight
}

func (a *Account) UninstallArtifact(heroID model.HeroID, instance model.InstanceID) error {
	hero, ok := a.Heroes[heroID]
	if !ok {
		return rejected("hero is not owned")
	}
	for position, equipped := range hero.Artifacts {
		if equipped == instance {
			delete(hero.Artifacts, position)
			a.setArtifactInstall(instance, 0)
			a.Heroes[heroID] = hero
			return nil
		}
	}
	return rejected("artifact is not installed on hero")
}

func (a *Account) InstallHighlight(heroID model.HeroID, instance model.InstanceID) error {
	hero, owned := a.Heroes[heroID]
	if _, exists := a.Highlights[instance]; !owned || !exists {
		return rejected("invalid highlight installation")
	}
	for id, entry := range a.Heroes {
		if id == heroID || !slices.Contains(slices.Collect(maps.Values(entry.Highlights)), instance) {
			continue
		}
		entry.Highlights = maps.Clone(entry.Highlights)
		maps.DeleteFunc(entry.Highlights, func(_ int, equipped model.InstanceID) bool { return equipped == instance })
		entry.Highlight = entry.Highlights[1]
		a.Heroes[id] = entry
	}
	hero.Highlights = maps.Clone(hero.Highlights)
	if hero.Highlights == nil {
		hero.Highlights = map[int]model.InstanceID{}
	}
	if previous := hero.Highlights[1]; previous != "" && previous != instance {
		a.setHighlightInstall(previous, 0)
	}
	maps.DeleteFunc(hero.Highlights, func(_ int, equipped model.InstanceID) bool { return equipped == instance })
	hero.Highlights[1] = instance
	hero.Highlight = instance
	a.setHighlightInstall(instance, heroID)
	a.Heroes[heroID] = hero
	return nil
}

func (a *Account) UninstallHighlight(heroID model.HeroID, slot int) error {
	hero, ok := a.Heroes[heroID]
	if !ok {
		return rejected("hero is not owned")
	}
	hero.Highlights = maps.Clone(hero.Highlights)
	if slot == 0 {
		for _, instance := range hero.Highlights {
			a.setHighlightInstall(instance, 0)
		}
		clear(hero.Highlights)
	} else {
		if instance := hero.Highlights[slot]; instance != "" {
			a.setHighlightInstall(instance, 0)
		}
		delete(hero.Highlights, slot)
	}
	hero.Highlight = hero.Highlights[1]
	a.Heroes[heroID] = hero
	return nil
}

func (a *Account) UpgradeArtifact(r *Rules, id model.InstanceID, count int) error {
	artifact, ok := a.Artifacts[id]
	rule := r.Artifacts[artifact.ID]
	if !ok || count < 1 || count > 100 || artifact.Level+count > min(rule.MaxLevel, a.Level) {
		return rejected("invalid artifact level")
	}
	cost := map[model.ItemID]int64{}
	for level := artifact.Level; level < artifact.Level+count; level++ {
		row, ok := r.ArtifactLevels[rule.Star][level]
		if !ok {
			return rejected("unknown artifact level")
		}
		cost[r.Constants.Items.ArtifactExperience] += row.Experience
		cost[r.Constants.Items.Gold] += row.Money
	}
	if len(r.ArtifactTemplates) > 0 {
		if err := initializeArtifactGrowth(r, &artifact); err != nil {
			return err
		}
		artifact.Growth = cloneGrowth(artifact.Growth)
		artifact.Order = artifact.Ordered().Order
		template := r.ArtifactTemplates[artifact.Template]
		for _, threshold := range rule.SubLevels {
			if threshold <= artifact.Level || threshold > artifact.Level+count {
				continue
			}
			if len(artifact.Growth) < r.Constants.Constants.ArtifactMaximumAttributes[rule.Star] {
				name, value, err := rollAttribute(template, artifact.Growth)
				if err != nil {
					return err
				}
				artifact.Order = append(artifact.Order, name)
				artifact.Growth[name] = []int64{value}
			} else {
				name, err := randomChoice(slices.Sorted(maps.Keys(artifact.Growth)))
				if err != nil {
					return err
				}
				limits := template.Attributes[name]
				value, err := randomRange(limits[0], limits[1])
				if err != nil {
					return err
				}
				artifact.Growth[name] = append(artifact.Growth[name], value)
			}
		}
		artifact.Sub = attributeTotals(artifact.Growth)
	}
	if err := a.Spend(cost); err != nil {
		return err
	}
	artifact.Level += count
	artifact.PendingGrowth, artifact.PendingOrder = map[string][]int64{}, nil
	a.Artifacts[id] = artifact
	a.RecordJourneyTaskEvent(r, 16, 1, 0)
	a.RefreshTasks(r)
	return nil
}

func (a *Account) SetSkin(r *Rules, heroID model.HeroID, skin model.SkinID) error {
	hero, ok := a.Heroes[heroID]
	if !ok || skin != 0 && r.Skins[skin] != heroID {
		return rejected("skin does not belong to hero")
	}
	if r.defaultSkin(skin) {
		skin = 0
	}
	if skin != 0 && !a.ownsSkin(r, skin) {
		return rejected("skin is not owned")
	}
	hero.Skin = skin
	a.Heroes[heroID] = hero
	return nil
}
