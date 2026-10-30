package account

import (
	"math"
	"slices"

	"mmm-revived/internal/model"
)

const (
	headItemType      = 1022
	headFrameItemType = 1023
	timedItemType     = 1050
	badgeItemType     = 1035
	backItemType      = 1045
)

func (a *Account) grantTimedCosmetic(r *Rules, rule ItemRule, amount, now int64) bool {
	target := model.ItemID(rule.Parameter)
	if kind := r.ItemRules[target].Type; kind != headItemType && kind != headFrameItemType || rule.Secondary <= 0 || amount > math.MaxInt64/rule.Secondary {
		return false
	}
	if a.TimedCosmetics == nil {
		a.TimedCosmetics = map[model.ItemID]int64{}
	}
	a.TimedCosmetics[target] = max(a.TimedCosmetics[target], now) + rule.Secondary*amount
	return true
}

func (a *Account) ownsPermanentCosmetic(c Catalog, kind int, item model.ItemID) bool {
	if !slices.Contains(c.Cosmetics[kind], item) {
		return false
	}
	if !a.Retail {
		return true
	}
	switch item {
	case c.DefaultHead, c.DefaultFrame, c.DefaultBack:
		return true
	}
	return a.Items[item] > 0
}

func (a *Account) ownsTimedCosmetic(c Catalog, kind int, item model.ItemID, now int64) bool {
	return slices.Contains(c.Cosmetics[kind], item) && a.TimedCosmetics[item] > now
}

func (a *Account) timedCosmeticSnapshot(c Catalog, appearance *model.AppearanceComponent) {
	appearance.LimitedHeads, appearance.LimitedFrames = map[int]model.LimitedCosmetic{}, map[int]model.LimitedCosmetic{}
	for item, valid := range a.TimedCosmetics {
		if a.ownsPermanentCosmetic(c, headItemType, item) || a.ownsPermanentCosmetic(c, headFrameItemType, item) {
			continue
		}
		switch {
		case slices.Contains(c.Cosmetics[headItemType], item):
			appearance.LimitedHeads[int(item)] = model.LimitedCosmetic{Valid: valid}
		case slices.Contains(c.Cosmetics[headFrameItemType], item):
			appearance.LimitedFrames[int(item)] = model.LimitedCosmetic{Valid: valid}
		}
	}
}

func (a *Account) ExpireCosmetics(c Catalog, now int64) bool {
	expired := false
	for _, slot := range [...]struct {
		kind     int
		item     *model.ItemID
		fallback model.ItemID
	}{{headItemType, &a.Profile.Head, c.DefaultHead}, {headFrameItemType, &a.Profile.Frame, c.DefaultFrame}} {
		if valid, timed := a.TimedCosmetics[*slot.item]; timed && valid <= now && !a.ownsPermanentCosmetic(c, slot.kind, *slot.item) {
			*slot.item = slot.fallback
			expired = true
		}
	}
	return expired
}
