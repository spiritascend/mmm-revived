package account

import (
	"fmt"
	"maps"
	"slices"

	"mmm-revived/internal/model"
)

type HelperSettings struct {
	InfiniteEnergy     bool               `json:"infinite_energy,omitempty"`
	InfiniteEnergySet  bool               `json:"infinite_energy_set,omitempty"`
	FreeDraws          bool               `json:"free_draws,omitempty"`
	FreeDrawsSet       bool               `json:"free_draws_set,omitempty"`
	DreamstoneShop     bool               `json:"dreamstone_shop,omitempty"`
	DreamstoneShopSet  bool               `json:"dreamstone_shop_set,omitempty"`
	AllBanners         bool               `json:"all_banners,omitempty"`
	AllBannersSet      bool               `json:"all_banners_set,omitempty"`
	FeaturedBanners    []model.DrawPoolID `json:"featured_banners,omitempty"`
	FeaturedBannersSet bool               `json:"featured_banners_set,omitempty"`
	TeamSize           int                `json:"team_size,omitempty"`
	TeamSizeSet        bool               `json:"team_size_set,omitempty"`
}

func (a *Account) SetHelperBoolean(name string, value bool) error {
	settings := &a.HelperSettings
	switch name {
	case "infinite_energy":
		settings.InfiniteEnergy, settings.InfiniteEnergySet = value, true
	case "free_draws":
		settings.FreeDraws, settings.FreeDrawsSet = value, true
	case "dreamstone_shop":
		settings.DreamstoneShop, settings.DreamstoneShopSet = value, true
	case "all_banners":
		settings.AllBanners, settings.AllBannersSet = value, true
		settings.FeaturedBanners, settings.FeaturedBannersSet = nil, false
	default:
		return fmt.Errorf("unknown Helper setting %q", name)
	}
	return nil
}

func (a *Account) SetHelperFeaturedBanners(ids []model.DrawPoolID) {
	if len(ids) == 0 {
		a.HelperSettings.FeaturedBanners = nil
		a.HelperSettings.FeaturedBannersSet = false
		a.HelperSettings.AllBanners = false
		a.HelperSettings.AllBannersSet = false
		return
	}
	a.HelperSettings.AllBanners = false
	a.HelperSettings.AllBannersSet = false
	a.HelperSettings.FeaturedBanners = slices.Clone(ids)
	a.HelperSettings.FeaturedBannersSet = true
}

func (a *Account) SetHelperTeamSize(size int) error {
	if size < 1 || size > 5 {
		return fmt.Errorf("team size must be between 1 and 5, got %d", size)
	}
	a.HelperSettings.TeamSize = size
	a.HelperSettings.TeamSizeSet = true
	return nil
}

func (a *Account) Economy(r *Rules) Economy {
	if r == nil {
		return Economy{}
	}
	settings := a.HelperSettings
	economy := r.Economy
	if settings.InfiniteEnergySet {
		economy.InfiniteEnergy = settings.InfiniteEnergy
	}
	if settings.FreeDrawsSet {
		economy.FreeDraws = settings.FreeDraws
	}
	if settings.DreamstoneShopSet {
		economy.DreamstoneShop = settings.DreamstoneShop
	}
	if settings.FeaturedBannersSet && len(settings.FeaturedBanners) > 0 {
		economy.Pools = map[model.DrawPoolID]bool{}
		for id, pool := range r.Draw.Pools {
			if pool.End > 0 {
				economy.Pools[id] = slices.Contains(settings.FeaturedBanners, id)
			}
		}
		economy.AllBanners = false
	} else if settings.AllBannersSet {
		economy.AllBanners = settings.AllBanners
		if settings.AllBanners {
			economy.Pools = map[model.DrawPoolID]bool{}
			for id, pool := range r.Draw.Pools {
				if pool.End > 0 {
					economy.Pools[id] = true
				}
			}
		} else if len(economy.FeaturedBanners) > 0 {
			economy.Pools = map[model.DrawPoolID]bool{}
			for id, pool := range r.Draw.Pools {
				if pool.End > 0 {
					economy.Pools[id] = slices.Contains(economy.FeaturedBanners, id)
				}
			}
		} else {
			economy.Pools = nil
		}
	} else {
		economy.Pools = maps.Clone(economy.Pools)
	}
	return economy
}

func (a *Account) EconomySnapshot(r *Rules, draw *model.DrawComponent) {
	economy := a.Economy(r)
	if len(economy.Pools) > 0 {
		draw.Open = make(map[model.DrawPoolID]int, len(economy.Pools))
		for id, open := range economy.Pools {
			draw.Open[id] = 0
			if open {
				draw.Open[id] = 1
			}
		}
	}
	if economy.DreamstoneShop {
		draw.DreamstoneShop = 1
	}
}

func (a *Account) drawPoolOpen(r *Rules, id model.DrawPoolID, pool DrawPoolRule, now int64) bool {
	if open, forced := a.Economy(r).Pools[id]; forced {
		return open
	}
	calendar := now + r.Draw.CalendarOffset
	return calendar >= pool.Start && (pool.End <= 0 || calendar < pool.End)
}

func (a *Account) battleTeamSize(r *Rules) int {
	if r == nil {
		return 0
	}
	if a.HelperSettings.TeamSizeSet {
		return a.HelperSettings.TeamSize
	}
	return r.TeamSize
}
