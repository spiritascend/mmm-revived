package config

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"math"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/pelletier/go-toml/v2"

	"mmm-revived/internal/account"
	"mmm-revived/internal/json"
	"mmm-revived/internal/model"
)

const (
	FileName       = "config.toml"
	legacyFileName = "config.json"
	AssetsDir      = "assets"
	DataDir        = "data"
	PatchesDir     = "patches"
	CatalogFile    = "catalog.json"
	DataTablesDir  = "datatables"
)

type Keys struct {
	Private string `json:"private" toml:"private"`
	Public  string `json:"public" toml:"public"`
}

type Config struct {
	Host             string  `json:"host" toml:"host"`
	Port             int     `json:"port" toml:"port"`
	HTTPPort         int     `json:"httpPort" toml:"httpPort"`
	FreshStart       bool    `json:"fresh_start" toml:"fresh_start"`
	StoryComplete    bool    `json:"story_complete" toml:"story_complete"`
	LegacyStart      string  `json:"start,omitempty" toml:"start,omitempty"`
	InfiniteEnergy   bool    `json:"infinite_energy" toml:"infinite_energy"`
	FreeDraws        bool    `json:"free_draws" toml:"free_draws"`
	DreamstoneShop   bool    `json:"dreamstone_shop" toml:"dreamstone_shop"`
	AllBanners       bool    `json:"all_banners" toml:"all_banners"`
	FeaturedBanners  []int32 `json:"featured_banners" toml:"featured_banners"`
	CalendarLoop     bool    `json:"calendar_loop" toml:"calendar_loop"`
	AllEvents        bool    `json:"all_events" toml:"all_events"`
	TeamSize         int     `json:"team_size" toml:"team_size"`
	EnergyRegen      float64 `json:"energy_regen" toml:"energy_regen"`
	RewardMultiplier int     `json:"reward_multiplier" toml:"reward_multiplier"`
	AllCosmetics     bool    `json:"all_cosmetics" toml:"all_cosmetics"`
	Keys             Keys    `json:"keys" toml:"keys"`
	dir              string
}

func Default() Config {
	return Config{
		Host:             "127.0.0.1",
		Port:             14100,
		HTTPPort:         18080,
		FreshStart:       true,
		CalendarLoop:     true,
		AllEvents:        true,
		TeamSize:         3,
		EnergyRegen:      1,
		RewardMultiplier: 1,
		Keys:             Keys{Private: "keys/server.pem", Public: "keys/client-public.pem"},
	}
}

func Load(path string) (Config, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return Config{}, err
	}
	c := Default()
	c.dir = filepath.Dir(absolute)
	data, err := os.ReadFile(absolute)
	if errors.Is(err, fs.ErrNotExist) {
		return c.migrate(absolute)
	}
	if err != nil {
		return c, err
	}
	if err = toml.NewDecoder(bytes.NewReader(data)).DisallowUnknownFields().Decode(&c); err != nil {
		return c, fmt.Errorf("parse %s: %w", absolute, err)
	}
	if c.LegacyStart != "" {
		if err = c.upgradeStart(); err != nil {
			return c, err
		}
		return c, c.Write(absolute)
	}
	return c, c.Validate()
}

func (c *Config) upgradeStart() error {
	if c.LegacyStart == "" {
		return nil
	}
	start, err := account.ParseStart(c.LegacyStart)
	if err != nil {
		return err
	}
	c.StoryComplete, c.LegacyStart = start == account.StartUnlocked, ""
	return nil
}

func Exists(path string) bool {
	for _, name := range []string{path, filepath.Join(filepath.Dir(path), legacyFileName)} {
		if info, err := os.Stat(name); err == nil && info.Mode().IsRegular() {
			return true
		}
	}
	return false
}

func (c Config) migrate(path string) (Config, error) {
	legacy := filepath.Join(filepath.Dir(path), legacyFileName)
	data, err := os.ReadFile(legacy)
	if errors.Is(err, fs.ErrNotExist) {
		return c, c.Write(path)
	}
	if err != nil {
		return c, err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(&c); err != nil {
		return c, fmt.Errorf("parse %s: %w", legacy, err)
	}
	if err = decoder.Decode(new(json.RawMessage)); !errors.Is(err, io.EOF) {
		return c, fmt.Errorf("parse %s: unexpected content after configuration", legacy)
	}
	if err = c.upgradeStart(); err != nil {
		return c, err
	}
	if err = c.Write(path); err != nil {
		return c, err
	}
	return c, os.Remove(legacy)
}

const fileTemplate = `host = %s
port = %d
httpPort = %d
fresh_start = %t # true = new accounts start from zero // false = every item and cosmetic unlocked
story_complete = %t # Story and tutorials already finished at max team level (used when fresh_start = false)
infinite_energy = %t # Energy refills to max after every use
free_draws = %t # Draws cost no keys (keeps 10 of each key)
dreamstone_shop = %t # Show the Dreamstone top-up page in the shop (top-ups are free)
all_banners = %t # Open every limited banner
featured_banners = %s # Limited banner ids to open instead of the calendar rotation, e.g. [5, 12]
calendar_loop = %t # Repeat the dated content calendar (Battle Pass, events, banners, PvP, guild wars) forever
all_events = %t # Keep every event open; one-at-a-time events (PvP seasons, Infinite Mayhem, bosses) rotate without gaps
team_size = %d # Max heroes per battle
energy_regen = %s # Energy regeneration speed, e.g. 1.25 = 25%% faster
reward_multiplier = %d # Multiplies Treasure Hunt, Rune Raid, Fabled Finds and other challenge rewards (XP stays the same)
all_cosmetics = %t # Unlock every avatar, frame, background, badge and outfit (used when fresh_start = true)

[keys]
private = %s
public = %s
`

func quote(value string) string {
	var builder strings.Builder
	builder.WriteByte('"')
	for _, r := range value {
		switch {
		case r == '"' || r == '\\':
			builder.WriteByte('\\')
			builder.WriteRune(r)
		case r < 0x20 || r == 0x7f:
			fmt.Fprintf(&builder, "\\u%04X", r)
		default:
			builder.WriteRune(r)
		}
	}
	builder.WriteByte('"')
	return builder.String()
}

func banners(ids []int32) string {
	values := make([]string, len(ids))
	for i, id := range ids {
		values[i] = strconv.Itoa(int(id))
	}
	return "[" + strings.Join(values, ", ") + "]"
}

func (c Config) Write(path string) error {
	if err := c.Validate(); err != nil {
		return err
	}
	data := fmt.Sprintf(fileTemplate, quote(c.Host), c.Port, c.HTTPPort, c.FreshStart, c.StoryComplete, c.InfiniteEnergy, c.FreeDraws, c.DreamstoneShop, c.AllBanners, banners(c.FeaturedBanners), c.CalendarLoop, c.AllEvents, c.TeamSize, strconv.FormatFloat(c.EnergyRegen, 'f', -1, 64), c.RewardMultiplier, c.AllCosmetics, quote(c.Keys.Private), quote(c.Keys.Public))
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(data), 0644)
}

func (c Config) Validate() error {
	var problems []error
	if net.ParseIP(c.Host) == nil {
		problems = append(problems, fmt.Errorf("host must be an IP address, got %q", c.Host))
	}
	for _, port := range []struct {
		name  string
		value int
	}{{"port", c.Port}, {"httpPort", c.HTTPPort}} {
		if port.value < 1 || port.value > 65535 {
			problems = append(problems, fmt.Errorf("%s must be between 1 and 65535", port.name))
		}
	}
	if c.Port == c.HTTPPort {
		problems = append(problems, errors.New("port and httpPort must differ"))
	}
	for _, id := range c.FeaturedBanners {
		if id <= 0 {
			problems = append(problems, fmt.Errorf("featured_banners must be draw pool ids, got %d", id))
		}
	}
	if c.TeamSize < 1 {
		problems = append(problems, fmt.Errorf("team_size must be at least 1, got %d", c.TeamSize))
	}
	if !(c.EnergyRegen > 0) || math.IsInf(c.EnergyRegen, 0) {
		problems = append(problems, fmt.Errorf("energy_regen must be greater than 0, got %v", c.EnergyRegen))
	}
	if c.RewardMultiplier < 1 {
		problems = append(problems, fmt.Errorf("reward_multiplier must be at least 1, got %d", c.RewardMultiplier))
	}
	if c.Keys.Private == "" || c.Keys.Public == "" || c.Keys.Private == c.Keys.Public {
		problems = append(problems, errors.New("keys.private and keys.public must be distinct paths"))
	}
	return errors.Join(problems...)
}

func (c Config) Address() string { return net.JoinHostPort(c.Host, strconv.Itoa(c.Port)) }

func (c Config) HTTPAddress() string { return net.JoinHostPort(c.Host, strconv.Itoa(c.HTTPPort)) }

func (c Config) StartMode() account.Start {
	if c.StoryComplete {
		return account.StartUnlocked
	}
	return account.StartItems
}

func (c Config) Economy() account.EconomyOptions {
	featured := make([]model.DrawPoolID, len(c.FeaturedBanners))
	for i, id := range c.FeaturedBanners {
		featured[i] = model.DrawPoolID(id)
	}
	return account.EconomyOptions{InfiniteEnergy: c.InfiniteEnergy, FreeDraws: c.FreeDraws, DreamstoneShop: c.DreamstoneShop, AllBanners: c.AllBanners, FeaturedBanners: featured}
}

func (c Config) Path(parts ...string) string {
	path := filepath.Join(parts...)
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(c.dir, path)
}

func (c Config) Catalog() string { return c.Path(AssetsDir, DataTablesDir, CatalogFile) }

func (c Config) Database() string { return c.Path(DataDir, "accounts") }

func (c Config) Patches() string { return c.Path(PatchesDir) }

func (c Config) PrivateKey() string { return c.Path(c.Keys.Private) }

func (c Config) PublicKey() string { return c.Path(c.Keys.Public) }
