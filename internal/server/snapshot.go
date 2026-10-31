package server

import (
	"maps"
	"slices"

	"mmm-revived/internal/account"
	"mmm-revived/internal/model"
)

func (s *Service) clientSnapshot(a *account.Account) model.Avatar {
	profile := a.Snapshot(s.Store.Catalog, s.Store.Rules)
	s.Store.Rules.EventWindowSnapshot(&profile.Activities)
	a.DoubleMaterialSnapshot(s.Store.Rules, &profile.Challenges)
	profile.Achievement = a.AchievementSnapshot(s.Store.Rules)
	sets := s.catalogSets()
	items, heroes, skins, artifacts, highlights := sets.items, sets.heroes, sets.skins, sets.artifacts, sets.highlights
	profile.Items.Normal = clientEntries(profile.Items.Normal, func(id model.ItemID, _ int64) bool { return clientHas(items, id) })
	profile.Items.Limited = clientEntries(profile.Items.Limited, func(id model.ItemID, _ model.LimitedItem) bool { return clientHas(items, id) })
	profile.Heroes.Heroes = clientEntries(profile.Heroes.Heroes, func(id model.HeroID, _ model.Hero) bool { return clientHas(heroes, id) })
	profile.Heroes.History = clientEntries(profile.Heroes.History, func(id model.HeroID, _ model.HeroHistory) bool { return clientHas(heroes, id) })
	profile.Heroes.Artifacts = clientEntries(profile.Heroes.Artifacts, func(_ model.InstanceID, value model.Artifact) bool { return clientHas(artifacts, value.ID) })
	profile.Heroes.Highlights = clientEntries(profile.Heroes.Highlights, func(_ model.InstanceID, value model.Highlight) bool { return clientHas(highlights, value.ID) })
	profile.Heroes.Skins = clientEntries(profile.Heroes.Skins, func(id model.SkinID, _ int) bool { return clientHas(skins, id) })
	profile.Heroes.SkinCollection = a.SkinCollectionLevel(s.Store.Rules)
	profile.Heroes.ArtifactCounts = make(map[model.ArtifactID]int, len(profile.Heroes.Artifacts))
	profile.Heroes.HighlightCounts = make(map[model.HighlightID]int, len(profile.Heroes.Highlights))
	for id, value := range profile.Heroes.Artifacts {
		if _, visible := profile.Heroes.Heroes[value.Install]; !visible {
			value.Install = 0
		}
		profile.Heroes.Artifacts[id] = value.ForClient(id, a.CreatedAt)
		profile.Heroes.ArtifactCounts[value.ID]++
	}
	for id, value := range profile.Heroes.Highlights {
		if _, visible := profile.Heroes.Heroes[value.Install]; !visible {
			value.Install = 0
		}
		profile.Heroes.Highlights[id] = value.ForClient(id, a.CreatedAt)
		profile.Heroes.HighlightCounts[value.ID]++
	}
	a.MergeCollectionCounts(&profile.Heroes)
	for id, hero := range profile.Heroes.Heroes {
		if !clientHas(skins, hero.Skin) {
			hero.Skin = 0
		}
		hero.Artifacts = clientEntries(hero.Artifacts, func(_ int, instance model.InstanceID) bool {
			_, visible := profile.Heroes.Artifacts[instance]
			return instance == "" || visible
		})
		hero.Highlights = clientEntries(hero.Highlights, func(_ int, instance model.InstanceID) bool {
			_, visible := profile.Heroes.Highlights[instance]
			return instance == "" || visible
		})
		hero.Highlight = hero.Highlights[1]
		profile.Heroes.Heroes[id] = hero
	}
	slotHeroes := func(ids []model.HeroID) []model.HeroID {
		result := slices.Clone(ids)
		for index, id := range result {
			if _, visible := profile.Heroes.Heroes[id]; id != 0 && !visible {
				result[index] = 0
			}
		}
		return result
	}
	profile.Heroes.ShowHero = slotHeroes(profile.Heroes.ShowHero)
	profile.Appearance.Support = slices.DeleteFunc(slices.Clone(profile.Appearance.Support), func(id model.HeroID) bool {
		_, visible := profile.Heroes.Heroes[id]
		return !visible
	})
	lineups := make(map[int]model.Lineup, model.LineupSlots)
	for slot := 1; slot <= model.LineupSlots; slot++ {
		lineup := profile.Lineups.Lineups[slot]
		if lineup.Type == 0 {
			lineup.Type = model.DefaultLineupType
		}
		lineup.Heroes = slotHeroes(lineup.Heroes)
		lineups[slot] = lineup
	}
	profile.Lineups.Lineups = lineups
	profile.Lobby.Heroes = maps.Clone(profile.Lobby.Heroes)
	var candidates []model.HeroID
	for slot, hero := range profile.Lobby.Heroes {
		if _, visible := profile.Heroes.Heroes[hero.Hero]; !visible {
			if candidates == nil {
				candidates = slices.Sorted(maps.Keys(profile.Heroes.Heroes))
			}
			if len(candidates) == 0 {
				delete(profile.Lobby.Heroes, slot)
				continue
			}
			hero.Hero, hero.Skin = candidates[0], 0
		}
		if !clientHas(skins, hero.Skin) {
			hero.Skin = 0
		}
		profile.Lobby.Heroes[slot] = hero
	}
	if len(sets.activities) > 0 {
		activities := sets.activities
		profile.Activities.Info = clientEntries(profile.Activities.Info, func(id model.ActivityID, _ model.ActivityInfo) bool { return clientHas(activities, id) })
		profile.Activities.Status = clientEntries(profile.Activities.Status, func(id model.ActivityID, _ int) bool { return clientHas(activities, id) })
	}
	return profile
}

type catalogSets struct {
	items      map[model.ItemID]struct{}
	heroes     map[model.HeroID]struct{}
	skins      map[model.SkinID]struct{}
	artifacts  map[model.ArtifactID]struct{}
	highlights map[model.HighlightID]struct{}
	activities map[model.ActivityID]struct{}
}

func newCatalogSets(catalog account.Catalog) catalogSets {
	sets := catalogSets{
		items:      clientIDs(catalog.Items),
		heroes:     make(map[model.HeroID]struct{}, len(catalog.Heroes)),
		skins:      clientIDs(catalog.Skins),
		artifacts:  clientIDs(catalog.Artifacts),
		highlights: clientIDs(catalog.Highlights),
		activities: clientIDs(catalog.Activities),
	}
	for id := range catalog.Heroes {
		sets.heroes[id] = struct{}{}
	}
	return sets
}

func clientIDs[T comparable](ids []T) map[T]struct{} {
	result := make(map[T]struct{}, len(ids))
	for _, id := range ids {
		result[id] = struct{}{}
	}
	return result
}

func clientHas[T comparable](ids map[T]struct{}, id T) bool {
	if len(ids) == 0 {
		return true
	}
	_, found := ids[id]
	return found
}

func clientEntries[K comparable, V any](entries map[K]V, include func(K, V) bool) map[K]V {
	if entries == nil {
		return nil
	}
	result := make(map[K]V, len(entries))
	for key, value := range entries {
		if include(key, value) {
			result[key] = value
		}
	}
	return result
}
