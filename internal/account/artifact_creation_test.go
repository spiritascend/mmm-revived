package account

import (
	"testing"

	"mmm-revived/internal/model"
)

func TestGainArtifactUsesSelectedStatsAndRarityRules(t *testing.T) {
	rules := testArtifactCreationRules()
	account := Account{
		Artifacts:  map[model.InstanceID]model.Artifact{},
		Highlights: map[model.InstanceID]model.Highlight{},
	}
	starting := []string{"attack", "attack", "defense"}
	upgrades := []string{"health", "critical", "attack"}

	if err := account.GainArtifact(rules, 5001, 1, starting, upgrades, 2); err != nil {
		t.Fatal(err)
	}
	if len(account.Artifacts) != 2 {
		t.Fatalf("got %d artifacts, want 2", len(account.Artifacts))
	}
	for _, artifact := range account.Artifacts {
		if artifact.Level != 12 {
			t.Errorf("got artifact level %d, want 12", artifact.Level)
		}
		if got := len(artifact.Growth["attack"]); got != 3 {
			t.Errorf("got %d attack rolls, want 3", got)
		}
		if artifact.Sub["attack"] != sumRolls(artifact.Growth["attack"]) {
			t.Errorf("attack total %d does not match rolls %v", artifact.Sub["attack"], artifact.Growth["attack"])
		}
	}
}

func TestGainArtifactRejectsUpgradeSelectionOutsideRarityRules(t *testing.T) {
	account := Account{
		Artifacts:  map[model.InstanceID]model.Artifact{},
		Highlights: map[model.InstanceID]model.Highlight{},
	}
	err := account.GainArtifact(testArtifactCreationRules(), 5001, 1,
		[]string{"attack", "attack", "defense"},
		[]string{"health", "attack"}, 1)
	if err == nil {
		t.Fatal("GainArtifact accepted an existing stat while a new stat was required")
	}
	if len(account.Artifacts) != 0 || account.NextInstance != 0 {
		t.Fatalf("invalid request changed account state: artifacts=%v next=%d", account.Artifacts, account.NextInstance)
	}
}

func testArtifactCreationRules() *Rules {
	return &Rules{
		Constants: ConstantRules{Constants: ClientConstants{
			ArtifactInitialAttributes: map[int]int{5: 3},
			ArtifactMaximumAttributes: map[int]int{5: 4},
		}},
		Artifacts: map[model.ArtifactID]ArtifactRule{
			5001: {Star: 5, Templates: []int{1}, SubLevels: []int{4, 8, 12}, MaxLevel: 12},
		},
		ArtifactTemplates: map[int]ArtifactTemplate{
			1: {
				Attributes: map[string][2]int64{
					"attack":   {10, 20},
					"critical": {10, 20},
					"defense":  {10, 20},
					"health":   {10, 20},
				},
				Weights: map[string]int64{
					"attack": 1, "critical": 1, "defense": 1, "health": 1,
				},
			},
		},
	}
}

func sumRolls(rolls []int64) int64 {
	var total int64
	for _, roll := range rolls {
		total += roll
	}
	return total
}
