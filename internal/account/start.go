package account

import (
	"errors"
	"fmt"
	"maps"
	"slices"

	"mmm-revived/internal/model"
)

type Start string

const (
	StartItems    Start = "items"
	StartUnlocked Start = "unlocked"
)

func ParseStart(value string) (Start, error) {
	switch start := Start(value); start {
	case StartItems, StartUnlocked:
		return start, nil
	default:
		return "", fmt.Errorf("start must be %q or %q", StartItems, StartUnlocked)
	}
}

func (a *Account) ApplyStart(r *Rules, catalog Catalog, start Start, now int64) error {
	if start == "" {
		return nil
	}
	if r == nil {
		return errors.New("start modes require game rules")
	}
	commands := []model.GMCommand{{Kind: model.GMGetAll, Value: 9999999}}
	if start == StartUnlocked {
		if len(r.TeamLevels) > 0 {
			commands = append(commands, model.GMCommand{Kind: model.GMTeamLevel, Value: int64(slices.Max(slices.Collect(maps.Keys(r.TeamLevels))))})
		}
		commands = append(commands,
			model.GMCommand{Kind: model.GMGuideFinish},
			model.GMCommand{Kind: model.GMNoviceFinish},
			model.GMCommand{Kind: model.GMDreamlandFinish, Text: "all"},
		)
	}
	state := a.GM
	for _, command := range commands {
		if _, err := a.ExecuteGM(r, catalog, command, now); err != nil {
			return fmt.Errorf("apply %s start: %w", start, err)
		}
	}
	a.GM = state
	return nil
}
