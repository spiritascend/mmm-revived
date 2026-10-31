package account

import (
	"time"

	"mmm-revived/internal/model"
)

func (a *Account) Snapshot(c Catalog, r *Rules) model.Avatar {
	if r == nil {
		r = &Rules{}
	}
	now := time.Now().Unix()
	day := LogicDay(now)
	energyBought := 0
	if a.Energy.Day == LogicDay(now) {
		energyBought = a.Energy.Bought
	}
	energyRecover, bankRecover := r.energyIntervals()
	profile := model.Avatar{
		GM:        model.GMComponent{History: a.GM.History},
		UID:       a.UID,
		AccountID: a.Name,
		Username:  a.Name,
		Host:      1,
		CreatedAt: a.CreatedAt,
		LastLogin: a.LastLogin,
		Basic: model.BasicComponent{
			Signature:      a.Profile.Signature,
			Experience:     a.Experience,
			RewardedLevels: a.RewardedLevels,
			Break:          a.TeamBreak,
			TeamName:       a.TeamName,
			Level:          a.Level,
			Gold:           a.Items[r.Constants.Items.Gold],
			Diamond:        a.Items[r.Constants.Items.Diamond],
			Yuanbao:        a.Items[r.Constants.Items.Yuanbao],
			MobileReward:   true,
		},
		Heroes: model.HeroComponent{
			Heroes:          a.Heroes,
			History:         make(map[model.HeroID]model.HeroHistory, len(a.Heroes)),
			Artifacts:       a.Artifacts,
			ArtifactCounts:  map[model.ArtifactID]int{},
			Highlights:      a.Highlights,
			HighlightCounts: map[model.HighlightID]int{},
			Skins:           a.OwnedSkins(c),
			ShowHero:        a.Profile.ShownHeroes,
		},
		Items: model.ItemComponent{Normal: a.Items, Limited: limitedItems(a.LimitedHeroes), Crafts: a.CraftRecipes},
		Lobby: model.LobbyComponent{
			Heroes:    map[int]model.LobbyHero{1: a.Profile.Lobby},
			Guides:    a.Guides,
			HiddenIDs: a.Profile.HiddenIDs,
			RedDots:   a.Profile.RedDots,
		},
		Energy: model.EnergyComponent{
			Energy:      a.Items[r.Constants.Items.Energy],
			Bank:        a.Energy.Bank,
			Purchases:   energyBought,
			RefreshedAt: a.Energy.Refreshed,
			Recover:     energyRecover,
			BankRecover: bankRecover,
		},
		Season: model.SeasonComponent{Level: 1},
		Appearance: model.AppearanceComponent{
			Head:        a.Profile.Head,
			Frame:       a.Profile.Frame,
			Background:  a.Profile.Background,
			Heads:       cosmetics(c, 1022, c.DefaultHead),
			Frames:      cosmetics(c, 1023, c.DefaultFrame),
			Backgrounds: cosmetics(c, 1045, c.DefaultBack),
			Support:     a.Profile.SupportHeroes,
			Tags:        a.Profile.Tags,
			Badge:       a.Profile.Badge,
			BadgeInfo:   map[int]int{},
		},
		Lineups:    model.LineupComponent{Lineups: a.Lineups},
		BattlePass: model.BattlePassComponent{Level: 1},
		Rogue:      a.RogueComponent(false),
		RoguePVE:   a.RogueComponent(true),
		Dreamland:  model.DreamlandComponent{Chapters: a.Chapters, Missions: a.Missions, Passed: a.Passed, Story: a.CurrentStory, Memento: a.CurrentMemento},
		Tasks:      model.TaskComponent{Tasks: a.Tasks},
	}
	profile.Dispatch = model.DispatchComponent{Acceptable: map[model.DispatchID]model.DispatchMission{}, Active: map[model.DispatchID]model.DispatchMission{}}
	if a.Dispatch.Day == day {
		profile.Dispatch.Count, profile.Dispatch.Refreshes = a.Dispatch.Started, a.Dispatch.Refreshes
	}
	for id, mission := range a.Dispatch.Missions {
		if mission.Started == 0 {
			profile.Dispatch.Acceptable[id] = mission
		} else {
			profile.Dispatch.Active[id] = mission
		}
	}
	profile.Shop = model.ShopComponent{Bought: make(map[model.ShopID]int, len(a.ShopPurchases)), Counts: map[model.ShopID]int{}}
	for id, purchase := range a.ShopPurchases {
		profile.Shop.Bought[id] = purchase.Total
		if purchase.Period == shopPeriod(purchase.PeriodType, now) {
			profile.Shop.Counts[id] = purchase.Count
		}
	}
	profile.Activities = model.ActivityComponent{Info: make(map[model.ActivityID]model.ActivityInfo, len(a.Activities)+2), Status: map[model.ActivityID]int{a.RealtimeActivity(r): 1}}
	for id, state := range a.Activities {
		profile.Activities.Info[id] = state
	}
	if id, _, _ := r.FreeResetActivity(); a.ResetActivityOpen(r, now) {
		profile.Activities.Status[id] = 1
		profile.Activities.Info[id] = model.PVPActivity{InitializedAt: a.CreatedAt}
	}
	if a.Retail {
		profile.Appearance.Heads = a.retailCosmetics(c, 1022, c.DefaultHead)
		profile.Appearance.Frames = a.retailCosmetics(c, 1023, c.DefaultFrame)
		profile.Appearance.Backgrounds = a.retailCosmetics(c, 1045, c.DefaultBack)
	}
	a.timedCosmeticSnapshot(c, &profile.Appearance)
	for _, id := range c.Cosmetics[1035] {
		if !a.Retail || a.Items[id] > 0 {
			profile.Appearance.BadgeInfo[int(id)] = 1
		}
	}
	for id, hero := range a.Heroes {
		profile.Heroes.History[id] = model.HeroHistory{MaxLevel: hero.Level}
	}
	for _, artifact := range a.Artifacts {
		profile.Heroes.ArtifactCounts[artifact.ID]++
	}
	for _, highlight := range a.Highlights {
		profile.Heroes.HighlightCounts[highlight.ID]++
	}
	profile.Draw = a.DrawSnapshot()
	a.EconomySnapshot(r, &profile.Draw)
	a.JourneySnapshot(&profile, now)
	a.EventsSnapshot(&profile)
	a.manualSnapshot(&profile)
	a.ApplySystemSnapshot(r, &profile, now)
	profile.PVP = a.NetworkSnapshot(r, now)
	profile.Battle.SupportUses = a.Network.Support.Used
	if a.Network.Activity != 0 {
		profile.Activities.Info[a.Network.Activity] = model.AsyncActivity{Score: a.Network.Points}
		profile.Activities.Status[a.Network.Activity] = 1
	}
	return profile
}

func cosmetics(c Catalog, kind int, fallback model.ItemID) []model.ItemID {
	if len(c.Cosmetics[kind]) > 0 {
		return c.Cosmetics[kind]
	}
	return []model.ItemID{fallback}
}
