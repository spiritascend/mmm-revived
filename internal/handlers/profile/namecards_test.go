package profile

import (
	"testing"

	"mmm-revived/internal/model"
	"mmm-revived/internal/protocol"
	"mmm-revived/internal/server"
)

func TestHelperNameCard(t *testing.T) {
	keys := []string{"name", "deleted", "h", "lv", "sign", "heroes", "supportHeroes", "show", "season"}
	card, err := helperNameCard(keys, []int{1001})
	if err != nil {
		t.Fatal(err)
	}
	if card["name"] != "Helper" || card["deleted"] != "0" || card["h"] != "5103" ||
		card["lv"] != "1" || card["sign"] != "Your local server helper." ||
		card["heroes"] != "1" || card["1001"] != "0" {
		t.Fatalf("unexpected helper name card fields: %v", card)
	}
	wanda := model.Hero{
		ID: 103, Level: 1, Star: 3,
		Talents: map[int]int{}, AwakeIDs: []int{}, Artifacts: map[int]model.InstanceID{},
	}
	wantShow, err := server.EncodePayload([]model.ShownHero{{ID: 103}})
	if err != nil {
		t.Fatal(err)
	}
	wantSupport, err := server.EncodePayload(map[model.HeroID]model.Hero{103: wanda})
	if err != nil {
		t.Fatal(err)
	}
	if card["show"] != wantShow || card["supportHeroes"] != wantSupport {
		t.Fatalf("Helper profile does not feature Scarlet Witch: %v", card)
	}
	for _, key := range keys {
		if _, ok := card[key]; !ok {
			t.Errorf("helper name card is missing requested field %q", key)
		}
	}
}

func TestDecodeNameCardsAcceptsHelperID(t *testing.T) {
	request, err := DecodeNameCards(protocol.Call{
		Args: []any{
			[]any{"name"},
			[]any{int64(model.HelperUID)},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(request.Players) != 1 || request.Players[0] != model.HelperUID {
		t.Fatalf("decoded player IDs = %v, want [%d]", request.Players, model.HelperUID)
	}
}
