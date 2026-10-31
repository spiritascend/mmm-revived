package profile

import (
	"fmt"
	"math/bits"
	"slices"
	"strconv"
	"time"

	"mmm-revived/internal/account"
	"mmm-revived/internal/model"
	"mmm-revived/internal/server"
	"mmm-revived/internal/wire"
)

func (s service) nameCards(p *server.Player, r NameCards) error {
	includeHeroes := slices.Contains(r.Keys, "show") || slices.Contains(r.Keys, "supportHeroes")
	profiles, err := s.Store.PublicProfiles(r.Players, includeHeroes)
	if err != nil {
		return err
	}
	badges := make(map[int]int, len(s.Store.Catalog.Cosmetics[1035]))
	for _, id := range s.Store.Catalog.Cosmetics[1035] {
		badges[int(id)] = 1
	}
	cards := make([]map[string]string, 0, len(r.Players))
	for _, id := range r.Players {
		if id == model.HelperUID {
			card, err := helperNameCard(r.Keys, honorScores(s.Store.Rules))
			if err != nil {
				return err
			}
			cards = append(cards, card)
			continue
		}
		entry, found := profiles[id]
		if !found {
			cards = append(cards, map[string]string{"deleted": "1"})
			continue
		}
		a := entry.Account
		profile := a.Profile
		online, offline := entry.Online, entry.Offline
		if s.Online(id) != nil {
			online = time.Now().Unix()
		} else if online >= offline {
			offline = max(a.LastLogin, online+1)
		}
		card := model.NameCard{Name: a.TeamName, Head: decimal(profile.Head), Frame: decimal(profile.Frame), Level: decimal(a.Level), Deleted: "0", Signature: profile.Signature, Story: decimal(storyMission(s.Store.Rules.StoryMissions(), a.Passed)), Offline: decimal(offline), Online: decimal(online), SeasonLevel: decimal(a.Events.Season.Level), PVP: decimal(pvpScore(s.Store.Rules, &a)), Heroes: decimal(entry.Heroes), Badge: decimal(profile.Badge), Background: decimal(profile.Background), Host: "1", Points: "0", Created: decimal(a.CreatedAt), Skins: decimal(len(a.OwnedSkins(s.Store.Catalog))), Server: "Local", GuildID: decimal(entry.Guild), GuildTitle: string(entry.GuildRole)}
		support := map[model.HeroID]model.Hero{}
		for _, hero := range profile.SupportHeroes {
			if value, ok := a.Heroes[hero]; ok {
				support[hero] = value
			}
		}
		shown := []model.ShownHero{}
		for _, id := range profile.ShownHeroes {
			value, ok := a.Heroes[id]
			if !ok {
				id, value.Skin = 0, 0
			}
			shown = append(shown, model.ShownHero{ID: id, Skin: value.Skin})
		}
		for _, field := range []struct {
			name        string
			destination *string
			value       any
		}{
			{"supportHeroes", &card.Support, support}, {"show", &card.Show, shown}, {"ownAchieveBadgeInfo", &card.Badges, badges}, {"showTags", &card.Tags, profile.Tags}, {"season", &card.Season, s.seasonHonor(&a)},
		} {
			if !slices.Contains(r.Keys, field.name) {
				continue
			}
			*field.destination, err = server.EncodePayload(field.value)
			if err != nil {
				return err
			}
		}
		fields, err := wire.StructFields(card)
		if err != nil {
			return err
		}
		selected := map[string]string{"deleted": "0"}
		for _, key := range r.Keys {
			value, ok := fields[key].(string)
			if !ok {
				return fmt.Errorf("unknown name card field %q", key)
			}
			selected[key] = value
		}
		if slices.Contains(r.Keys, "showTags") {
			for _, key := range honorScores(s.Store.Rules) {
				selected[strconv.Itoa(key)] = decimal(a.Manual.Bests[key])
			}
		}
		cards = append(cards, selected)
	}
	return p.Send("onReqNameCard", r.Players, cards)
}

func helperNameCard(keys []string, scores []int) (map[string]string, error) {
	const scarletWitch model.HeroID = 103
	card := model.NameCard{
		Name: "Helper", Head: "5103", Frame: "0", Level: "1", Deleted: "0",
		Signature: "Your local server helper.",
		Story:     "0", Offline: "0", Online: "0", SeasonLevel: "0", PVP: "0",
		Heroes: "1", Badge: "0", Background: "0", Host: "1", Points: "0",
		Created: "0", Skins: "0", Server: "Local", GuildID: "0",
		GuildTitle: "",
	}
	wanda := model.Hero{
		ID: scarletWitch, Level: 1, Star: 3,
		Talents: map[int]int{}, AwakeIDs: []int{}, Artifacts: map[int]model.InstanceID{},
	}
	for _, field := range []struct {
		name        string
		destination *string
		value       any
	}{
		{"supportHeroes", &card.Support, map[model.HeroID]model.Hero{scarletWitch: wanda}},
		{"show", &card.Show, []model.ShownHero{{ID: scarletWitch}}},
		{"ownAchieveBadgeInfo", &card.Badges, map[int]int{}},
		{"showTags", &card.Tags, model.HonorTags{}},
		{"season", &card.Season, map[string]int{"season": 0, "stars": 0, "passed": 0}},
	} {
		var err error
		*field.destination, err = server.EncodePayload(field.value)
		if err != nil {
			return nil, err
		}
	}
	fields, err := wire.StructFields(card)
	if err != nil {
		return nil, err
	}
	selected := map[string]string{"deleted": "0"}
	for _, key := range keys {
		value, ok := fields[key].(string)
		if !ok {
			return nil, fmt.Errorf("unknown name card field %q", key)
		}
		selected[key] = value
	}
	for _, key := range scores {
		selected[strconv.Itoa(key)] = "0"
	}
	return selected, nil
}

func honorScores(r *account.Rules) []int {
	var scores []int
	if r == nil {
		return scores
	}
	for _, tag := range r.Constants.HonorTags {
		if key, err := strconv.Atoi(tag); err == nil {
			scores = append(scores, key)
		}
	}
	return scores
}

func storyMission(missions []model.MissionID, passed map[model.MissionID]model.MissionProgress) model.MissionID {
	for _, id := range missions {
		if _, ok := passed[id]; !ok {
			return id
		}
	}
	if len(missions) == 0 {
		return 0
	}
	return missions[len(missions)-1]
}

func pvpScore(r *account.Rules, a *account.Account) int {
	score, _ := a.NetworkScores(r)
	return score
}

func (s service) seasonHonor(a *account.Account) map[string]int {
	stars := 0
	if rules := s.Store.Rules; rules != nil {
		for _, group := range rules.Journey.SpiralGroups {
			for _, mission := range group.Missions {
				stars += bits.OnesCount(uint(a.Journey.Spiral[mission] & 7))
			}
		}
	}
	return map[string]int{"season": a.Events.Season.ID, "stars": stars, "passed": len(a.Journey.Spiral)}
}

func decimal[T ~int | ~int32 | ~int64](value T) string { return strconv.FormatInt(int64(value), 10) }
