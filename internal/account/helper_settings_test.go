package account

import (
	"encoding/json"
	"reflect"
	"testing"

	"mmm-revived/internal/model"
)

func TestHelperSettingsAreScopedToAccount(t *testing.T) {
	rules := &Rules{
		Constants: ConstantRules{
			Items:         ClientItems{Energy: 1},
			FormationSize: 3,
		},
		TeamSize: 3,
		TeamLevels: map[int]TeamLevelRule{
			1: {EnergyLimit: 100},
		},
		Economy: Economy{
			FreeDraws:      true,
			DreamstoneShop: true,
			AllBanners:     true,
			Pools:          map[model.DrawPoolID]bool{5: true, 12: true},
		},
		Draw: DrawRules{Pools: map[model.DrawPoolID]DrawPoolRule{
			5:  {End: 100, Ticket: 20},
			12: {End: 100, Ticket: 21},
		}},
		BattleSettings: map[model.BattleID]BattleRule{
			1: {Heroes: 3},
		},
	}

	sender := &Account{
		Level: 1,
		Items: map[model.ItemID]int64{},
	}
	other := &Account{
		Level: 1,
		Items: map[model.ItemID]int64{},
	}
	for _, setting := range []string{"infinite_energy", "free_draws", "dreamstone_shop", "all_banners"} {
		value := setting == "infinite_energy"
		if err := sender.SetHelperBoolean(setting, value); err != nil {
			t.Fatalf("SetHelperBoolean(%q): %v", setting, err)
		}
	}
	featured := []model.DrawPoolID{5}
	sender.SetHelperFeaturedBanners(featured)
	featured[0] = 12
	if err := sender.SetHelperTeamSize(4); err != nil {
		t.Fatalf("SetHelperTeamSize: %v", err)
	}

	sender.ApplyHelperEconomy(rules)
	other.ApplyHelperEconomy(rules)

	if got := sender.Items[rules.Constants.Items.Energy]; got != 100 {
		t.Errorf("sender energy = %d, want 100", got)
	}
	if got := other.Items[rules.Constants.Items.Energy]; got != 0 {
		t.Errorf("other account energy = %d, want 0", got)
	}
	if got := sender.Items[20]; got != 0 {
		t.Errorf("sender free-draw ticket count = %d, want 0", got)
	}
	if got := other.Items[20]; got != freeDrawKeys {
		t.Errorf("other account free-draw ticket count = %d, want %d", got, freeDrawKeys)
	}

	senderDraw := model.DrawComponent{}
	sender.EconomySnapshot(rules, &senderDraw)
	otherDraw := model.DrawComponent{}
	other.EconomySnapshot(rules, &otherDraw)
	if senderDraw.DreamstoneShop != 0 || otherDraw.DreamstoneShop != 1 {
		t.Errorf("Dreamstone shop visibility: sender=%d, other=%d; want 0, 1", senderDraw.DreamstoneShop, otherDraw.DreamstoneShop)
	}
	if senderDraw.Open[5] != 1 || senderDraw.Open[12] != 0 {
		t.Errorf("sender banner visibility = %v, want pool 5 open and 12 closed", senderDraw.Open)
	}
	if otherDraw.Open[5] != 1 || otherDraw.Open[12] != 1 {
		t.Errorf("other account banner visibility = %v, want both pools open", otherDraw.Open)
	}

	if limit, custom := sender.BattleHeroLimit(rules, 1); limit != 4 || !custom {
		t.Errorf("sender battle limit = %d, custom=%t; want 4, true", limit, custom)
	}
	if limit, custom := other.BattleHeroLimit(rules, 1); limit != 3 || custom {
		t.Errorf("other account battle limit = %d, custom=%t; want 3, false", limit, custom)
	}

	sender.SetHelperFeaturedBanners(nil)
	resetDraw := model.DrawComponent{}
	sender.EconomySnapshot(rules, &resetDraw)
	if resetDraw.Open[5] != 1 || resetDraw.Open[12] != 1 {
		t.Errorf("clearing banner overrides did not restore server defaults: %v", resetDraw.Open)
	}

	allBannersOff := &Account{}
	if err := allBannersOff.SetHelperBoolean("all_banners", false); err != nil {
		t.Fatal(err)
	}
	normalRotation := model.DrawComponent{}
	allBannersOff.EconomySnapshot(rules, &normalRotation)
	if len(normalRotation.Open) != 0 {
		t.Errorf("all_banners=false forced a static banner list instead of calendar rotation: %v", normalRotation.Open)
	}
	if !allBannersOff.drawPoolOpen(rules, 5, rules.Draw.Pools[5], 50) {
		t.Error("all_banners=false did not preserve a banner open by the calendar")
	}
}

func TestHelperSettingsPersistAndValidate(t *testing.T) {
	account := Account{}
	if err := account.SetHelperBoolean("unknown", true); err == nil {
		t.Fatal("SetHelperBoolean accepted an unknown setting")
	}
	if err := account.SetHelperTeamSize(0); err == nil {
		t.Fatal("SetHelperTeamSize accepted a team size below 1")
	}
	if err := account.SetHelperTeamSize(6); err == nil {
		t.Fatal("SetHelperTeamSize accepted a team size above 5")
	}

	if err := account.SetHelperBoolean("infinite_energy", false); err != nil {
		t.Fatal(err)
	}
	if err := account.SetHelperBoolean("free_draws", true); err != nil {
		t.Fatal(err)
	}
	if err := account.SetHelperBoolean("dreamstone_shop", false); err != nil {
		t.Fatal(err)
	}
	if err := account.SetHelperBoolean("all_banners", false); err != nil {
		t.Fatal(err)
	}
	account.SetHelperFeaturedBanners([]model.DrawPoolID{5, 12})
	if err := account.SetHelperTeamSize(4); err != nil {
		t.Fatal(err)
	}

	data, err := json.Marshal(account)
	if err != nil {
		t.Fatalf("marshal account: %v", err)
	}
	var restored Account
	if err = json.Unmarshal(data, &restored); err != nil {
		t.Fatalf("unmarshal account: %v", err)
	}
	if !reflect.DeepEqual(restored.HelperSettings, account.HelperSettings) {
		t.Errorf("restored Helper settings = %+v, want %+v", restored.HelperSettings, account.HelperSettings)
	}
}
