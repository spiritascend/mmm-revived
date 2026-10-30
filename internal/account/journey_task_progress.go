package account

import (
	"math"
	"math/bits"
	"slices"
	"time"

	"mmm-revived/internal/model"
)

type TaskScope string

const (
	TaskGeneral     TaskScope = ""
	TaskGuild       TaskScope = "guild"
	TaskHyperflow   TaskScope = "hyperflow"
	TaskLimitedPass TaskScope = "limited_pass"
	TaskBattlePass  TaskScope = "battle_pass"
)

func (a *Account) journeyTaskEnabled(r *Rules, id model.TaskID, rule TaskRule, now int64) bool {
	if a.Retail && rule.Unreferenced {
		return false
	}
	switch rule.Scope {
	case TaskGeneral:
		return true
	case TaskGuild:
		return a.Journey.GuildMember
	case TaskBattlePass:
		pass, err := a.passRule(r, now)
		return err == nil && (slices.Contains(pass.Tasks, id) || slices.Contains(pass.WeeklyTasks, id))
	case TaskHyperflow, TaskLimitedPass:
		for activityID, state := range a.Journey.Activities {
			activity := r.Journey.Activities[activityID]
			if !state.Open || !a.journeyActivityAvailable(activity, now, r.Journey.CalendarOffset) {
				continue
			}
			if rule.Scope == TaskHyperflow && activity.Type == 22 && slices.Contains(r.Journey.Hyperflows[activity.Parameter].Tasks, id) {
				return true
			}
			if rule.Scope == TaskLimitedPass && activity.Type == 20 {
				pass := r.Journey.LimitedPasses[state.Pass.ID]
				if slices.Contains(pass.Tasks, id) || slices.Contains(pass.WeeklyTasks, id) {
					return true
				}
			}
		}
	}
	return false
}

func taskNumber(rule TaskRule, index, otherwise int) int {
	if len(rule.Params) > index {
		return rule.Params[index]
	}
	return otherwise
}

func (a *Account) taskCumulativeValue(r *Rules, rule TaskRule) int64 {
	switch rule.Type {
	case 19:
		var result int64
		for _, pool := range a.Draw.Pools {
			result += int64(pool.Count)
		}
		return result
	case 21, 43:
		return a.Journey.SpentItems[r.Constants.Items.Energy]
	case 42:
		return a.Journey.DailyClaims
	case 73:
		return a.Journey.SpentItems[model.ItemID(taskNumber(rule, 0, 1))]
	}
	return 0
}

func (a *Account) refreshJourneyTaskValues(r *Rules) {
	a.refreshJourneyTaskValuesAt(r, time.Now().Unix())
}

func (a *Account) refreshJourneyTaskValuesAt(r *Rules, now int64) {
	if r == nil {
		return
	}
	day, week := LogicDay(now), LogicWeek(now)
	for id, rule := range r.Tasks {
		if !a.journeyTaskEnabled(r, id, rule, now) {
			continue
		}
		var value int64
		switch rule.Type {
		case 1:
			value = int64(a.Journey.LoginDays)
			if rule.Scope == TaskGuild {
				value = int64(a.Journey.GuildLoginDays)
			}
			if rule.Daily || rule.Scope == TaskHyperflow {
				value = min(value, 1)
			}
		case 3:
			value = int64(a.Level)
		case 6:
			value = a.Items[model.ItemID(taskNumber(rule, 0, 0))]
		case 4:
			minimum := taskNumber(rule, 0, 0)
			for _, hero := range a.Heroes {
				if hero.Level >= minimum {
					value++
				}
			}
		case 5, 11:
			value = int64(a.Heroes[model.HeroID(taskNumber(rule, 0, 0))].Level)
		case 8:
			minimum := taskNumber(rule, 0, 1)
			for _, hero := range a.Heroes {
				if hero.Break >= minimum {
					value++
				}
			}
		case 9:
			for _, mission := range rule.Lists[1] {
				if taskNumber(rule, 0, 1) == 2 && a.Journey.Challenges[mission].Stars > 0 || taskNumber(rule, 0, 1) != 2 && a.Passed[model.MissionID(mission)].Stars > 0 {
					value++
				}
			}
		case 12:
			for _, task := range rule.Lists[0] {
				if a.Tasks[model.TaskID(task)].Status >= 4 {
					value++
				}
			}
		case 14:
			value = int64(len(a.Highlights))
		case 19, 21, 42, 43, 73:
			value = max(int64(0), a.taskCumulativeValue(r, rule)-a.Journey.TaskBaselines[id])
			if rule.Type == 21 || rule.Type == 43 || rule.Type == 73 {
				item := r.Constants.Items.Energy
				if rule.Type == 73 {
					item = model.ItemID(taskNumber(rule, 0, 1))
				}
				if rule.Daily || rule.Scope == TaskHyperflow {
					value = 0
					if a.Journey.SpentDay == day {
						value = a.Journey.DailySpent[item]
					}
				}
				if rule.Scope == TaskBattlePass && slices.Contains(r.Journey.Passes[a.Journey.Pass.ID].WeeklyTasks, id) {
					value = 0
					if a.Journey.SpentWeek == week {
						value = a.Journey.WeeklySpent[item]
					}
				}
			}
		case 22:
			minimum := taskNumber(rule, 0, 0)
			for _, highlight := range a.Highlights {
				if highlight.Level >= minimum {
					value++
				}
			}
		case 23:
			if a.journeyChallengeCleared(r, rule.Lists[2], taskNumber(rule, 1, 1)) {
				value = 1
			}
		case 25, 26:
			minimum := taskNumber(rule, 0, 0)
			for _, hero := range a.Heroes {
				for _, instance := range hero.Artifacts {
					artifact, known := a.Artifacts[instance]
					if !known {
						continue
					}
					if rule.Type == 25 && artifact.Level >= minimum || rule.Type == 26 && r.Artifacts[artifact.ID].Star >= minimum {
						value++
					}
				}
			}
		case 28:
			for _, activity := range a.Journey.Activities {
				for _, node := range activity.BattleNodes {
					value += int64(bits.OnesCount(uint(node.Stars & 7)))
				}
			}
		case 32:
			minimum := taskNumber(rule, 0, 1)
			for _, hero := range a.Heroes {
				for _, level := range hero.Talents {
					if level >= minimum {
						value++
					}
				}
			}
		case 37:
			for _, hero := range a.Heroes {
				for _, level := range hero.Talents {
					value += int64(max(0, level-1))
				}
			}
		case 38:
			value = int64(len(a.Artifacts))
		case 39:
			value = int64(a.TeamBreak)
		case 40:
			switch taskNumber(rule, 0, 0) {
			case 1:
				value = int64(a.RogueProgress.TotalWins)
			case 2:
				value = int64(a.RogueProgress.PVEWins)
			default:
				value = int64(a.RogueProgress.TotalWins + a.RogueProgress.PVEWins)
			}
		case 50:
			chapter := model.ChapterID(taskNumber(rule, 0, 0))
			for mission, progress := range a.Passed {
				if r.Missions[mission].Chapter == chapter {
					value += int64(bits.OnesCount(uint(progress.Stars & 7)))
				}
			}
		case 51:
			value = int64(len(a.Heroes))
		case 60:
			kind := taskNumber(rule, 0, 0)
			for item, itemKind := range r.ItemTypes {
				if itemKind == kind && a.Items[item] > 0 {
					value++
				}
			}
		case 66:
			if _, known := a.Heroes[model.HeroID(taskNumber(rule, 0, 0))]; known {
				value = 1
			}
		case 70:
			value = 1
			for _, hero := range rule.Params {
				if _, known := a.Heroes[model.HeroID(hero)]; !known {
					value = 0
					break
				}
			}
		case 87:
			if a.Journey.SharedInvite {
				value = 1
			}
		case 92:
			if a.TeamName != "" && a.TeamName != a.Name {
				value = 1
			}
		case 95:
			minimum := taskNumber(rule, 0, 1)
			for _, hero := range a.Heroes {
				for _, instance := range hero.Highlights {
					if highlight, known := a.Highlights[instance]; known && highlight.Level >= minimum {
						value++
					}
				}
			}
		case 110:
			mission := taskNumber(rule, 0, 0)
			for _, activity := range a.Journey.Activities {
				if activity.Missions[mission].Finished == 1 {
					value = 1
					break
				}
			}
		case 111:
			game := taskNumber(rule, 0, 0)
			for _, activity := range a.Journey.Activities {
				if activity.MiniGames[game] == 1 {
					value = 1
					break
				}
			}
		case 114:
			if a.Passed[model.MissionID(taskNumber(rule, 0, 0))].Stars > 0 {
				value = 1
			}
		case 115:
			target := taskNumber(rule, 0, 0)
			for activityID, activity := range a.Journey.Activities {
				if int(activityID) == target {
					for _, node := range activity.BattleNodes {
						value += int64(bits.OnesCount(uint(node.Stars & 7)))
					}
					break
				}
			}
		case 10, 53:
			if pvpRuleTask(r, id) {
				continue
			}
			for _, activity := range a.Activities {
				if rule.Type == 10 {
					value = max(value, int64(max(activity.Wins, activity.MaxWins)))
				} else {
					value += max(0, activity.Points)
				}
			}
		case 27, 59, 83, 84, 85:
			if rule.Achievement == 0 {
				continue
			}
			value = a.journeyTaskBackfill(rule)
		case 52:
			for _, mission := range rule.Lists[0] {
				value = max(value, a.Events.Boss.Best[mission])
			}
		case 61, 62, 93:
			value = a.journeyDrawValue(r, rule)
		case 64:
			value = int64(a.Events.Season.Level)
		case 80:
			minimum := taskNumber(rule, 0, 5)
			for _, hero := range a.Heroes {
				if hero.Trial == 0 && hero.ProficiencyLevel >= minimum {
					value++
				}
			}
		case 81:
			if a.Profile.Signature != "" {
				value = 1
			}
		case 109:
			for _, count := range a.System.Products {
				value += int64(count)
			}
		case 82:
			value = a.rogueLevelValue(taskNumber(rule, 0, 1))
		case 88:
			value = a.rogueFavorValue(rule.Targets)
		case 130:
			if !a.journeyConditionTaskOpen(r, id, now) {
				continue
			}
			for _, activity := range a.Journey.Activities {
				if activity.Open && activity.RetailPVP != nil {
					value = max(value, activity.RetailPVP.Point)
				}
			}
		default:
			continue
		}
		a.UpdateTask(id, int(min(value, math.MaxInt32)), rule.Target)
	}
	a.recordAchievementTimes(r, now)
}

func (a *Account) journeyChallengeCleared(r *Rules, battles []int, stars int) bool {
	for _, battle := range battles {
		for challenge, entry := range r.Journey.Challenges {
			if entry.Battle == model.BattleID(battle) && a.Journey.Challenges[challenge].Stars >= stars {
				return true
			}
		}
	}
	return false
}

func (a *Account) RecordJourneyTaskEvent(r *Rules, kind, amount, parameter int) {
	if r == nil || amount <= 0 {
		return
	}
	now := time.Now().Unix()
	for id, rule := range r.Tasks {
		if rule.Type != kind || !a.journeyTaskEnabled(r, id, rule, now) || journeyActivityTaskType(kind) && !a.journeyConditionTaskOpen(r, id, now) {
			continue
		}
		if filter := rule.Lists[0]; (kind == 2 || kind == 7) && len(filter) > 0 && !slices.Contains(filter, 0) && !slices.Contains(filter, parameter) {
			continue
		}
		if kind == 31 && (len(rule.Params) < 2 || rule.Params[1] != parameter) || kind == 59 && id == r.Constants.Constants.DailyShareTask {
			continue
		}
		task := a.Tasks[id]
		if task.Status >= 4 {
			continue
		}
		a.UpdateTask(id, int(min(math.MaxInt32, int64(task.Value)+int64(amount))), rule.Target)
	}
}

func (a *Account) RecordJourneyBattleStatistics(r *Rules, ultimates, cards int64) {
	if ultimates > 0 && ultimates <= 100000 {
		a.RecordJourneyTaskEvent(r, 68, int(ultimates), 0)
	}
	if cards > 0 && cards <= 100000 {
		a.RecordJourneyTaskEvent(r, 71, int(cards), 0)
	}
}

func (r *Rules) challengeTaskParameter(group int) int {
	for kind, groups := range r.Journey.ChallengeTypes {
		if slices.Contains(groups, group) {
			return r.Settings.BattleTasks.ChallengeTypes[kind]
		}
	}
	return 0
}

func (r *Rules) BattleTaskParameter(mission model.MissionID, challenge int) int {
	switch {
	case r == nil:
		return 0
	case challenge > 0:
		return r.challengeTaskParameter(r.Journey.Challenges[challenge].Group)
	case mission != 0:
		return r.Settings.BattleTasks.ChapterTags[r.GM.Chapters[r.Missions[mission].Chapter]]
	}
	return 0
}

func (a *Account) RecordJourneyBattleTasks(r *Rules, parameter int, won bool, kills int, manaSpent int64, assist bool) {
	if won {
		a.RecordJourneyTaskEvent(r, 2, 1, parameter)
	}
	if kills > 0 && kills <= 1000000 {
		a.RecordJourneyTaskEvent(r, 74, kills, 0)
	}
	if manaSpent > 0 && manaSpent <= 1000000 {
		a.RecordJourneyTaskEvent(r, 72, int(manaSpent), 0)
	}
	if won && assist {
		a.RecordJourneyTaskEvent(r, 69, 1, 0)
	}
}
