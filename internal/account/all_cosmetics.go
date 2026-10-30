package account

import "mmm-revived/internal/model"

func (s *Store) ConfigureCosmetics(enabled bool) {
	if s.Rules != nil {
		s.Rules.AllCosmetics = enabled
	}
}

func (a *Account) unlockCosmetics(c Catalog, r *Rules) {
	unlock := func(item model.ItemID) {
		if a.Items[item] <= 0 {
			a.Items[item] = 1
		}
	}
	for _, kind := range [...]int{headItemType, headFrameItemType, badgeItemType, backItemType} {
		for _, item := range c.Cosmetics[kind] {
			unlock(item)
		}
	}
	for item := range r.ItemRules {
		if skin := r.skinItem(item); skin != 0 && !r.defaultSkin(skin) {
			if _, known := r.Skins[skin]; known {
				unlock(item)
			}
		}
	}
}
