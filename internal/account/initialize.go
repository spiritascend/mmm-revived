package account

import (
	"maps"
	"slices"
	"strconv"

	"mmm-revived/internal/model"
)

func (a *Account) Initialize(c Catalog, r *Rules) {
	a.InitializeDraw()
	a.InitializeSystem()
	a.InitializeGM()
	a.AdvanceCalendar(r, a.LastLogin)
	a.InitializeEvents(r, a.LastLogin)
	a.InitializeRogue(r, a.LastLogin)
	if a.Dispatch.Missions == nil {
		a.Dispatch.Missions = map[model.DispatchID]model.DispatchMission{}
	}
	if a.ShopPurchases == nil {
		a.ShopPurchases = map[model.ShopID]model.ShopPurchase{}
	}
	if !a.Energy.Initialized {
		a.Energy = model.PlayerEnergy{Initialized: true, Bank: DefaultEnergy, Day: LogicDay(a.LastLogin), Refreshed: a.LastLogin}
	}
	if a.CraftRecipes == nil && r != nil {
		a.CraftRecipes = []model.CraftID{}
		for id, rule := range r.Crafts {
			if rule.Initial != 0 {
				a.CraftRecipes = append(a.CraftRecipes, id)
			}
		}
		slices.Sort(a.CraftRecipes)
	}
	if a.Profile.Head == 0 {
		a.Profile.Head = c.DefaultHead
	}
	if a.Profile.Frame == 0 {
		a.Profile.Frame = c.DefaultFrame
	}
	if a.Profile.Background == 0 {
		a.Profile.Background = c.DefaultBack
	}
	if a.Profile.ShownHeroes == nil && r != nil {
		for _, id := range r.Defaults.Retail.Heroes {
			if _, ok := a.Heroes[id]; ok {
				a.Profile.ShownHeroes = append(a.Profile.ShownHeroes, id)
			}
		}
	}
	if a.Profile.RedDots == nil {
		a.Profile.RedDots = map[string]int64{}
	}
	if a.Profile.Lobby.Hero == 0 && len(a.Profile.ShownHeroes) > 0 {
		a.Profile.Lobby.Hero = a.Profile.ShownHeroes[0]
	}
	if len(a.Profile.Tags) == 0 {
		a.Profile.Tags = model.HonorTags{"story", "heroes", "skins", "createTime"}
	}
	if a.Lineups == nil {
		a.Lineups = map[int]model.Lineup{}
	}
	if a.Chapters == nil {
		a.Chapters = map[model.ChapterID]model.ChapterProgress{}
	}
	if a.Activities == nil {
		a.Activities = map[model.ActivityID]model.PVPActivity{}
	}
	if a.Missions == nil {
		a.Missions = map[model.MissionID]model.Mission{}
	}
	if a.Passed == nil {
		a.Passed = map[model.MissionID]model.MissionProgress{}
	}
	if a.Tasks == nil {
		a.Tasks = map[model.TaskID]model.Task{}
	}
	if a.Guides == nil {
		a.Guides = map[string]string{}
	}
	if a.Highlights == nil {
		a.Highlights = map[model.InstanceID]model.Highlight{}
		for i, id := range c.Highlights {
			a.Highlights[model.InstanceID(strconv.Itoa(i+1))] = model.Highlight{ID: id, Level: 1}
		}
	}
	if a.Artifacts == nil {
		a.Artifacts = map[model.InstanceID]model.Artifact{}
		for i, id := range c.Artifacts {
			a.Artifacts[model.InstanceID(strconv.Itoa(i+1))] = model.Artifact{ID: id, Level: 1}
		}
	}
	if a.NextInstance == 0 {
		observe := func(id model.InstanceID) {
			if value, err := strconv.ParseInt(string(id), 10, 64); err == nil {
				a.NextInstance = max(a.NextInstance, value)
			}
		}
		for id := range a.Artifacts {
			observe(id)
		}
		for id := range a.Highlights {
			observe(id)
		}
	}
	if r != nil {
		if mission, ok := a.Missions[a.CurrentStory]; ok && finishedReward(r, a.CurrentStory, mission) {
			a.CurrentStory = 0
		}
		if mission, ok := a.Missions[a.CurrentMemento]; ok && finishedReward(r, a.CurrentMemento, mission) {
			a.CurrentMemento = 0
		}
		if mission, ok := a.Missions[a.CurrentMission]; ok && finishedReward(r, a.CurrentMission, mission) {
			a.CurrentMission = 0
		}
		if a.CurrentMission != 0 {
			if r.Missions[a.CurrentMission].Memento {
				if a.CurrentMemento == 0 {
					a.CurrentMemento = a.CurrentMission
				}
			} else if a.CurrentStory == 0 {
				a.CurrentStory = a.CurrentMission
			}
		}
		for id, mission := range a.Missions {
			if mission.Visited == nil {
				mission.Visited = map[model.MementoChipID]int{}
			}
			if mission.LokiRecord == nil {
				mission.LokiRecord = map[model.MementoChipID]int{}
			}
			if mission.TeamSteps == nil {
				mission.TeamSteps = map[int]int{}
			}
			if node := r.Nodes[mission.Node.ID]; len(mission.Visited) == 0 && mission.Step == 0 && node.Type == 5 {
				initializeMemento(&mission, r.Missions[id], r.MementoMaps[node.Map])
			}
			a.Missions[id] = mission
		}
		a.removeUnreleasedGear(r)
		for id, artifact := range a.Artifacts {
			if templates := r.Artifacts[artifact.ID].Templates; artifact.Template == 0 && len(templates) > 0 {
				artifact.Template = templates[0]
			}
			if artifact.Growth == nil {
				artifact.Growth = growthFromSub(artifact.Sub)
			}
			if artifact.PendingGrowth == nil {
				artifact.PendingGrowth = map[string][]int64{}
			}
			if artifact.Sub == nil {
				artifact.Sub = map[string]int64{}
			}
			if seeded := artifact; len(artifact.Growth) == 0 && len(r.ArtifactTemplates) > 0 && initializeArtifactGrowth(r, &seeded) == nil {
				artifact = seeded
			}
			a.Artifacts[id] = artifact
		}
		if !a.Retail {
			a.TeamBreak = TeamBreakAt(r, a.Level)
		}
		a.normalizeSkins(r)
		if a.Retail {
			for _, id := range slices.Sorted(maps.Keys(a.Heroes)) {
				_ = a.grantStarReward(r, id, 0)
			}
		}
		for id, rule := range r.Tasks {
			if _, ok := a.Tasks[id]; !ok {
				a.Tasks[id] = model.Task{Status: 2}
			}
			if rule.Type == 3 {
				a.UpdateTask(id, a.Level, rule.Target)
			}
		}
		a.RefreshTasks(r)
	}
	a.InitializeJourney(r, a.LastLogin)
	a.RefreshTasks(r)
	if r != nil {
		a.PrepareNetwork(r.Network, a.LastLogin)
	}
}

func (a *Account) UpdateTask(id model.TaskID, value, target int) {
	task := a.Tasks[id]
	if task.Status >= 4 {
		return
	}
	task.Value = max(task.Value, value)
	if target > 0 && task.Value >= target {
		task.Status = 3
	}
	a.Tasks[id] = task
}
