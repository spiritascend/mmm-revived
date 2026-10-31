package friend

import (
	"database/sql"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"mmm-revived/internal/account"
	"mmm-revived/internal/model"
	"mmm-revived/internal/server"
)

func (s service) helperChat(p *server.Player, kind int, text string) error {
	if kind != 0 && kind != 7 {
		return s.helperReply(p, "Helper only supports text messages.")
	}
	fields := strings.Fields(text)
	if len(fields) == 0 {
		return s.helperReply(p, "Send /commands to see available Helper commands.")
	}
	command := strings.ToLower(fields[0])
	switch command {
	case "/commands":
		if len(fields) != 1 {
			return s.helperReply(p, "Usage: /commands")
		}
		return s.helperReply(p, helperCommands)
	case "/artifact_ids":
		if len(fields) != 1 {
			return s.helperReply(p, "Usage: /artifact_ids")
		}
		if s.Store.Rules == nil {
			return server.ErrProgressionRules
		}
		return s.sendArtifactIDs(p, s.Store.Rules)
	case "/gain_artifact":
		if len(fields) != 1 {
			return s.helperReply(p, "Usage: /gain_artifact")
		}
		if p.ArtifactWizard != nil {
			return s.helperReply(p, "An artifact wizard is already active. Reply /cancel before starting another.")
		}
		if s.Store.Rules == nil {
			return server.ErrProgressionRules
		}
		p.ArtifactWizard = &server.ArtifactWizard{Stage: server.ArtifactWizardID}
		return s.helperReply(p, "Available artifact IDs: "+availableArtifactIDs(s.Store.Rules)+". Enter the ID you want, or /cancel.")
	case "/infinte_energy", "/infinite_energy":
		return s.setHelperBoolean(p, fields, "infinite_energy")
	case "/free_draws":
		return s.setHelperBoolean(p, fields, "free_draws")
	case "/dreamstone_shop":
		return s.setHelperBoolean(p, fields, "dreamstone_shop")
	case "/all_banners":
		return s.setHelperBoolean(p, fields, "all_banners")
	case "/featured_banners":
		return s.setFeaturedBanners(p, fields)
	case "/team_size":
		return s.setTeamSize(p, fields)
	case "/cancel":
		if p.ArtifactWizard == nil {
			return s.helperReply(p, "No artifact wizard is active.")
		}
		return s.answerArtifactWizard(p, "/cancel")
	default:
		if p.ArtifactWizard != nil {
			return s.answerArtifactWizard(p, text)
		}
		return s.helperReply(p, "Unknown command. Send /commands to see available Helper commands.")
	}
}

const helperCommands = "Helper commands (setting changes are saved to your account only):\n" +
	"/gain_artifact - create an artifact with guided prompts\n" +
	"/artifact_ids - list available artifact IDs and names\n" +
	"/cancel - stop the artifact wizard\n" +
	"/infinite_energy on|off (also /infinte_energy; true|false and 1|0 also work)\n" +
	"/free_draws on|off (true|false and 1|0 also work)\n" +
	"/dreamstone_shop on|off (true|false and 1|0 also work)\n" +
	"/all_banners on|off (true|false and 1|0 also work)\n" +
	"/featured_banners <comma-separated README banner IDs|none>\n" +
	"/team_size 1-5\n" +
	"/commands - show this list"

func (s service) helperReply(p *server.Player, text string) error {
	message := model.ChatMessage{
		ID:       strconv.FormatInt(time.Now().UnixNano(), 10),
		Sender:   model.HelperUID,
		Receiver: p.Account.UID,
		Text:     text,
		Time:     time.Now().Unix(),
	}
	return p.Send("notifyNewChatMsg", message)
}

func (s service) sendArtifactIDs(p *server.Player, rules *account.Rules) error {
	entries := artifactIDNames(rules)
	if len(entries) == 0 {
		return s.helperReply(p, "No available artifacts.")
	}
	const entriesPerMessage = 10
	total := (len(entries) + entriesPerMessage - 1) / entriesPerMessage
	for start := 0; start < len(entries); start += entriesPerMessage {
		end := min(start+entriesPerMessage, len(entries))
		if err := s.helperReply(p, fmt.Sprintf("Artifact IDs (%d/%d):\n%s", start/entriesPerMessage+1, total, strings.Join(entries[start:end], "\n"))); err != nil {
			return err
		}
	}
	return nil
}

func artifactIDNames(rules *account.Rules) []string {
	var ids []int
	for id, rule := range rules.Artifacts {
		if rule.Unreleased || rule.Star < 3 || rule.Star > 5 || !hasUsableArtifactTemplate(rules, rule) {
			continue
		}
		ids = append(ids, int(id))
	}
	slices.Sort(ids)
	entries := make([]string, 0, len(ids))
	for _, id := range ids {
		name := strings.TrimSpace(rules.Artifacts[model.ArtifactID(id)].Name)
		if name == "" {
			name = "name unavailable"
		}
		entries = append(entries, fmt.Sprintf("%d - %s", id, name))
	}
	return entries
}

func availableArtifactIDs(rules *account.Rules) string {
	var ids []int
	for id, rule := range rules.Artifacts {
		if rule.Unreleased || rule.Star < 3 || rule.Star > 5 {
			continue
		}
		for _, template := range rule.Templates {
			if _, known := rules.ArtifactTemplates[template]; known {
				ids = append(ids, int(id))
				break
			}
		}
	}
	slices.Sort(ids)
	formatted := make([]string, len(ids))
	for i, id := range ids {
		formatted[i] = strconv.Itoa(id)
	}
	return strings.Join(formatted, ", ")
}

func (s service) setHelperBoolean(p *server.Player, fields []string, name string) error {
	if len(fields) != 2 {
		return s.helperReply(p, fmt.Sprintf("Usage: /%s on|off", name))
	}
	value, err := parseHelperBoolean(fields[1])
	if err != nil {
		return s.helperReply(p, fmt.Sprintf("Usage: /%s on|off", name))
	}
	if err := s.Store.Update(p.Account, func(*sql.Tx) error {
		if err := p.Account.SetHelperBoolean(name, value); err != nil {
			return err
		}
		p.Account.ApplyHelperEconomy(s.Store.Rules)
		return nil
	}); err != nil {
		return err
	}
	switch name {
	case "infinite_energy":
		if err := s.Sync(p, "energyComp", "itemComp"); err != nil {
			return err
		}
	case "free_draws":
		if err := s.Sync(p, "itemComp"); err != nil {
			return err
		}
	case "dreamstone_shop", "all_banners":
		if err := s.Sync(p, "drawComp"); err != nil {
			return err
		}
	}
	return s.helperReply(p, fmt.Sprintf("Saved %s = %t to your account. This setting affects only you.", name, value))
}

func parseHelperBoolean(text string) (bool, error) {
	switch strings.ToLower(text) {
	case "true", "on", "yes", "1":
		return true, nil
	case "false", "off", "no", "0":
		return false, nil
	default:
		return false, fmt.Errorf("expected on/off, true/false, yes/no, or 1/0")
	}
}

func (s service) setFeaturedBanners(p *server.Player, fields []string) error {
	if len(fields) < 2 {
		return s.helperReply(p, "Usage: /featured_banners <comma-separated README banner IDs|none>")
	}
	if s.Store.Rules == nil {
		return server.ErrProgressionRules
	}
	banners, err := parseFeaturedBanners(strings.Join(fields[1:], " "), s.Store.Rules.Draw.Pools)
	if err != nil {
		return s.helperReply(p, err.Error())
	}
	if err = s.Store.Update(p.Account, func(*sql.Tx) error {
		poolIDs := make([]model.DrawPoolID, len(banners))
		for i, id := range banners {
			poolIDs[i] = model.DrawPoolID(id)
		}
		p.Account.SetHelperFeaturedBanners(poolIDs)
		p.Account.ApplyHelperEconomy(s.Store.Rules)
		return nil
	}); err != nil {
		return err
	}
	if err = s.Sync(p, "drawComp"); err != nil {
		return err
	}
	if len(banners) == 0 {
		return s.helperReply(p, "Cleared your featured banner override. This setting affects only you.")
	}
	return s.helperReply(p, fmt.Sprintf("Saved featured_banners = %s to your account. This setting affects only you.", strings.Join(int32Strings(banners), ", ")))
}

func parseFeaturedBanners(text string, pools map[model.DrawPoolID]account.DrawPoolRule) ([]int32, error) {
	if strings.EqualFold(strings.TrimSpace(text), "none") {
		return nil, nil
	}
	allowed := map[int32]bool{
		2: true, 3: true, 4: true, 5: true, 6: true, 7: true, 8: true, 9: true, 10: true,
		11: true, 12: true, 13: true, 14: true, 15: true, 17: true, 19: true, 20: true,
		21: true, 23: true, 25: true, 27: true, 29: true,
	}
	parts := strings.Split(text, ",")
	banners := make([]int32, 0, len(parts))
	seen := make(map[int32]bool, len(parts))
	for _, part := range parts {
		value, err := strconv.ParseInt(strings.TrimSpace(part), 10, 32)
		id := int32(value)
		if err != nil || !allowed[id] {
			return nil, fmt.Errorf("Invalid banner ID %q. Use IDs listed in the README, separated by commas, or none to clear.", strings.TrimSpace(part))
		}
		if seen[id] {
			return nil, fmt.Errorf("Banner ID %d is listed more than once.", id)
		}
		if _, known := pools[model.DrawPoolID(id)]; !known {
			return nil, fmt.Errorf("Banner ID %d is not available in this server's rules.", id)
		}
		seen[id] = true
		banners = append(banners, id)
	}
	return banners, nil
}

func (s service) setTeamSize(p *server.Player, fields []string) error {
	if len(fields) != 2 {
		return s.helperReply(p, "Usage: /team_size 1-5")
	}
	size, err := strconv.Atoi(fields[1])
	if err != nil || size < 1 || size > 5 {
		return s.helperReply(p, "Team size must be a whole number from 1 to 5.")
	}
	if err = s.Store.Update(p.Account, func(*sql.Tx) error {
		return p.Account.SetHelperTeamSize(size)
	}); err != nil {
		return err
	}
	return s.helperReply(p, fmt.Sprintf("Saved team_size = %d to your account. This setting affects only you.", size))
}

func int32Strings(values []int32) []string {
	formatted := make([]string, len(values))
	for i, value := range values {
		formatted[i] = strconv.FormatInt(int64(value), 10)
	}
	return formatted
}

func (s service) answerArtifactWizard(p *server.Player, text string) error {
	wizard := p.ArtifactWizard
	answer := strings.TrimSpace(text)
	if strings.EqualFold(answer, "/cancel") {
		p.ArtifactWizard = nil
		return s.helperReply(p, "Artifact command cancelled.")
	}
	if s.Store.Rules == nil {
		p.ArtifactWizard = nil
		return server.ErrProgressionRules
	}
	rules := s.Store.Rules
	switch wizard.Stage {
	case server.ArtifactWizardID:
		id, err := strconv.ParseInt(answer, 10, 64)
		if err != nil {
			return s.helperReply(p, "Enter a numeric artifact ID from the list, or /cancel.")
		}
		artifactID := model.ArtifactID(id)
		rule, known := rules.Artifacts[artifactID]
		if id <= 0 || !known || rule.Unreleased || rule.Star < 3 || rule.Star > 5 || !hasUsableArtifactTemplate(rules, rule) {
			return s.helperReply(p, "That artifact ID is unavailable. Enter an ID from the list, or /cancel.")
		}
		wizard.ArtifactID = artifactID
		wizard.Stage = server.ArtifactWizardRarity
		return s.helperReply(p, "Choose rarity: blue, purple, or yellow.")

	case server.ArtifactWizardRarity:
		rarity := map[string]int{"blue": 3, "purple": 4, "yellow": 5}[strings.ToLower(answer)]
		if rarity == 0 || rules.Artifacts[wizard.ArtifactID].Star != rarity {
			return s.helperReply(p, "Rarity does not match this artifact ID. Choose blue, purple, or yellow.")
		}
		template, err := rules.ChooseArtifactTemplate(wizard.ArtifactID)
		if err != nil {
			return err
		}
		wizard.Template = template
		wizard.Stage = server.ArtifactWizardStartingCount
		maxStarting := rules.Constants.Constants.ArtifactInitialAttributes[rarity]
		return s.helperReply(p, fmt.Sprintf("How many starting rolls? Enter 0-%d.", maxStarting))

	case server.ArtifactWizardStartingCount:
		maximum := rules.Constants.Constants.ArtifactInitialAttributes[rules.Artifacts[wizard.ArtifactID].Star]
		count, err := parseWizardCount(answer, maximum)
		if err != nil {
			return s.helperReply(p, fmt.Sprintf("Enter a whole number from 0 to %d.", maximum))
		}
		wizard.StartingCount = count
		wizard.StartingStats = make([]string, 0, count)
		if count == 0 {
			wizard.Stage = server.ArtifactWizardUpgradeCount
			return s.promptUpgradeCount(p, wizard, rules)
		}
		wizard.Stage = server.ArtifactWizardStartingStats
		return s.promptArtifactStat(p, wizard, rules, "Choose a starting stat (repeats allowed).")

	case server.ArtifactWizardStartingStats:
		if !hasArtifactAttribute(rules, wizard.Template, answer) {
			return s.promptArtifactStat(p, wizard, rules, "That stat is unavailable. Choose a starting stat (repeats allowed).")
		}
		wizard.StartingStats = append(wizard.StartingStats, answer)
		if len(wizard.StartingStats) == wizard.StartingCount {
			wizard.Stage = server.ArtifactWizardUpgradeCount
			return s.promptUpgradeCount(p, wizard, rules)
		}
		return s.promptArtifactStat(p, wizard, rules, "Choose a starting stat (repeats allowed).")

	case server.ArtifactWizardUpgradeCount:
		maxUpgrades := availableWizardUpgradeRolls(rules, wizard)
		count, err := parseWizardCount(answer, maxUpgrades)
		if err != nil {
			return s.helperReply(p, fmt.Sprintf("Enter a whole number from 0 to %d.", maxUpgrades))
		}
		wizard.UpgradeCount = count
		wizard.UpgradeStats = make([]string, 0, count)
		if count == 0 {
			wizard.Stage = server.ArtifactWizardInventory
			return s.promptArtifactInventory(p, wizard, rules)
		}
		wizard.Stage = server.ArtifactWizardUpgradeStats
		return s.promptUpgradeStat(p, wizard, rules)

	case server.ArtifactWizardUpgradeStats:
		allowed := allowedUpgradeAttributes(rules, wizard)
		if !slices.Contains(allowed, answer) {
			return s.promptUpgradeStat(p, wizard, rules)
		}
		wizard.UpgradeStats = append(wizard.UpgradeStats, answer)
		if len(wizard.UpgradeStats) == wizard.UpgradeCount {
			wizard.Stage = server.ArtifactWizardInventory
			return s.promptArtifactInventory(p, wizard, rules)
		}
		return s.promptUpgradeStat(p, wizard, rules)

	case server.ArtifactWizardInventory:
		switch strings.ToLower(answer) {
		case "yes", "y":
			wizard.Stage = server.ArtifactWizardCopies
			return s.helperReply(p, "How many copies? Enter 1-1000.")
		case "no", "n":
			p.ArtifactWizard = nil
			return s.helperReply(p, "Preview complete; no artifacts were added.")
		default:
			return s.promptArtifactInventory(p, wizard, rules)
		}

	case server.ArtifactWizardCopies:
		copies, err := parseWizardCount(answer, 1000)
		if err != nil || copies == 0 {
			return s.helperReply(p, "Enter a whole number from 1 to 1000.")
		}
		if err := s.Store.Update(p.Account, func(*sql.Tx) error {
			return p.Account.GainArtifact(rules, wizard.ArtifactID, wizard.Template, wizard.StartingStats, wizard.UpgradeStats, copies)
		}); err != nil {
			return err
		}
		p.ArtifactWizard = nil
		if err := s.Sync(p, "heroComp"); err != nil {
			return err
		}
		return s.helperReply(p, fmt.Sprintf("Added %d artifact copies to your inventory.", copies))
	}
	p.ArtifactWizard = nil
	return s.helperReply(p, "Artifact command ended. Send /gain_artifact to start again.")
}

func (s service) promptUpgradeCount(p *server.Player, wizard *server.ArtifactWizard, rules *account.Rules) error {
	maxUpgrades := availableWizardUpgradeRolls(rules, wizard)
	return s.helperReply(p, fmt.Sprintf("How many upgrade rolls? Enter 0-%d.", maxUpgrades))
}

func (s service) promptArtifactInventory(p *server.Player, wizard *server.ArtifactWizard, rules *account.Rules) error {
	rarity := map[int]string{3: "blue", 4: "purple", 5: "yellow"}[rules.Artifacts[wizard.ArtifactID].Star]
	starting, upgrades := strings.Join(wizard.StartingStats, ", "), strings.Join(wizard.UpgradeStats, ", ")
	if starting == "" {
		starting = "none"
	}
	if upgrades == "" {
		upgrades = "none"
	}
	summary := fmt.Sprintf("Artifact %d (%s); starting stats: %s; upgrade stats: %s.",
		wizard.ArtifactID, rarity, starting, upgrades)
	return s.helperReply(p, summary+" Add it to your inventory? Reply yes or no.")
}

func (s service) promptArtifactStat(p *server.Player, wizard *server.ArtifactWizard, rules *account.Rules, message string) error {
	return s.helperReply(p, fmt.Sprintf("%s Available stats: %s.", message, strings.Join(artifactAttributes(rules, wizard.Template), ", ")))
}

func (s service) promptUpgradeStat(p *server.Player, wizard *server.ArtifactWizard, rules *account.Rules) error {
	allowed := allowedUpgradeAttributes(rules, wizard)
	if len(allowed) == 0 {
		return s.helperReply(p, "No stats are valid for another upgrade roll. Choose a lower number of upgrade rolls.")
	}
	return s.helperReply(p, "Choose an upgrade stat. Available stats: "+strings.Join(allowed, ", ")+".")
}

func parseWizardCount(text string, maximum int) (int, error) {
	count, err := strconv.Atoi(text)
	if err != nil || count < 0 || count > maximum {
		return 0, fmt.Errorf("count must be between 0 and %d", maximum)
	}
	return count, nil
}

func availableWizardUpgradeRolls(rules *account.Rules, wizard *server.ArtifactWizard) int {
	rule := rules.Artifacts[wizard.ArtifactID]
	attributes := artifactAttributes(rules, wizard.Template)
	growth := map[string]bool{}
	for _, name := range wizard.StartingStats {
		growth[name] = true
	}
	maximum := rules.Constants.Constants.ArtifactMaximumAttributes[rule.Star]
	count := 0
	for _, level := range rule.SubLevels {
		if level <= 1 || level > rule.MaxLevel {
			continue
		}
		if len(growth) < maximum {
			available := ""
			for _, name := range attributes {
				if !growth[name] {
					available = name
					break
				}
			}
			if available == "" {
				break
			}
			growth[available] = true
		} else if len(growth) == 0 {
			break
		}
		count++
	}
	return count
}

func artifactAttributes(rules *account.Rules, templateID int) []string {
	template := rules.ArtifactTemplates[templateID]
	attributes := make([]string, 0, len(template.Attributes))
	for name, limits := range template.Attributes {
		if limits[0] >= 0 && limits[1] >= limits[0] && template.Weights[name] > 0 {
			attributes = append(attributes, name)
		}
	}
	slices.Sort(attributes)
	return attributes
}

func hasArtifactAttribute(rules *account.Rules, templateID int, name string) bool {
	return slices.Contains(artifactAttributes(rules, templateID), name)
}

func hasUsableArtifactTemplate(rules *account.Rules, rule account.ArtifactRule) bool {
	for _, id := range rule.Templates {
		if _, known := rules.ArtifactTemplates[id]; known {
			return true
		}
	}
	return false
}

func allowedUpgradeAttributes(rules *account.Rules, wizard *server.ArtifactWizard) []string {
	attributes := artifactAttributes(rules, wizard.Template)
	growth := map[string]bool{}
	for _, name := range wizard.StartingStats {
		growth[name] = true
	}
	for _, name := range wizard.UpgradeStats {
		growth[name] = true
	}
	maximum := rules.Constants.Constants.ArtifactMaximumAttributes[rules.Artifacts[wizard.ArtifactID].Star]
	if len(growth) < maximum {
		return slices.DeleteFunc(attributes, func(name string) bool { return growth[name] })
	}
	return slices.DeleteFunc(attributes, func(name string) bool { return !growth[name] })
}
