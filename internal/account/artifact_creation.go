package account

import (
	"slices"
	"time"

	"mmm-revived/internal/model"
)

func (r *Rules) ChooseArtifactTemplate(id model.ArtifactID) (int, error) {
	if r == nil {
		return 0, rejected("artifact rules are unavailable")
	}
	rule, known := r.Artifacts[id]
	if !known || rule.Unreleased {
		return 0, rejected("unknown artifact")
	}
	templates := slices.DeleteFunc(slices.Clone(rule.Templates), func(id int) bool {
		_, known := r.ArtifactTemplates[id]
		return !known
	})
	template, err := randomChoice(templates)
	if err != nil {
		return 0, err
	}
	return template, nil
}

func (a *Account) GainArtifact(r *Rules, id model.ArtifactID, templateID int, starting, upgrades []string, copies int) error {
	if r == nil {
		return rejected("artifact rules are unavailable")
	}
	rule, known := r.Artifacts[id]
	template, hasTemplate := r.ArtifactTemplates[templateID]
	if !known || rule.Unreleased || !slices.Contains(rule.Templates, templateID) || !hasTemplate {
		return rejected("unknown artifact template")
	}
	maxStarting, hasStarting := r.Constants.Constants.ArtifactInitialAttributes[rule.Star]
	maxAttributes, hasMaximum := r.Constants.Constants.ArtifactMaximumAttributes[rule.Star]
	if !hasStarting || !hasMaximum || maxStarting < 0 || len(starting) > maxStarting || maxAttributes < 1 {
		return rejected("invalid artifact starting rolls")
	}
	if copies < 1 || copies > 1000 || len(upgrades) > len(rule.SubLevels) {
		return rejected("invalid artifact quantity or upgrade rolls")
	}
	if len(template.Attributes) == 0 {
		return rejected("artifact attribute pool is empty")
	}
	for name, limits := range template.Attributes {
		if limits[0] < 0 || limits[1] < limits[0] || template.Weights[name] <= 0 {
			return rejected("invalid artifact attribute range")
		}
	}

	artifacts := make([]model.Artifact, 0, copies)
	for range copies {
		growth := map[string][]int64{}
		order := model.AttributeOrder{}
		for _, name := range starting {
			if _, exists := template.Attributes[name]; !exists {
				return rejected("unknown artifact attribute")
			}
			value, err := randomRange(template.Attributes[name][0], template.Attributes[name][1])
			if err != nil {
				return err
			}
			if _, exists := growth[name]; !exists {
				order = append(order, name)
			}
			growth[name] = append(growth[name], value)
		}

		level := 1
		for i, name := range upgrades {
			if i >= len(rule.SubLevels) || rule.SubLevels[i] <= level || rule.SubLevels[i] > rule.MaxLevel {
				return rejected("invalid artifact upgrade level")
			}
			_, exists := growth[name]
			if (len(growth) < maxAttributes && exists) || (len(growth) >= maxAttributes && !exists) {
				return rejected("artifact upgrade attribute violates rarity rules")
			}
			if _, exists := template.Attributes[name]; !exists {
				return rejected("unknown artifact attribute")
			}
			value, err := randomRange(template.Attributes[name][0], template.Attributes[name][1])
			if err != nil {
				return err
			}
			if !exists {
				order = append(order, name)
			}
			growth[name] = append(growth[name], value)
			level = rule.SubLevels[i]
		}

		artifact := model.Artifact{
			ID:            id,
			Level:         level,
			Template:      templateID,
			Growth:        growth,
			PendingGrowth: map[string][]int64{},
			Order:         order,
			Sub:           attributeTotals(growth),
			Obtained:      time.Now().Unix(),
		}
		artifacts = append(artifacts, artifact)
	}
	for _, artifact := range artifacts {
		a.Artifacts[a.instanceID()] = artifact
	}
	return nil
}
