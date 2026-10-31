package server

import (
	"crypto/rand"
	"encoding/hex"

	"mmm-revived/internal/account"
	"mmm-revived/internal/actor"
	"mmm-revived/internal/gate"
	"mmm-revived/internal/model"
)

type BattleState uint8

const (
	Lobby BattleState = iota
	Deployment
	Combat
	Queued
	PVPDeployment
	PVPCombat
)

type Player struct {
	CampActivity          model.ActivityID
	CampNode              int
	Deployment            []model.DeploymentHero
	ArtifactWizard        *ArtifactWizard
	ActivityBattle        model.ActivityID
	ActivityMission       int
	ActivityNode          int
	Brawl                 model.ActivityID
	LimitedBoss           model.ActivityID
	LimitedBossGroup      int
	LimitedBossDifficulty int
	Challenge             int
	Spiral                int
	BossMission           int
	RogueBattle           bool
	Limited               map[model.HeroID]model.RogueLimitedHero
	Chip                  *model.MementoChipID
	SelectedHeroes        []model.HeroID
	HeroLimit             int
	LastBattleWon         bool
	Account               *account.Account
	BattleID              model.BattleID
	State                 BattleState
	properties            *actor.State
	Nonce                 string
	Mission               model.MissionID
	Session               *gate.Session
	Room                  *Room
	Level                 int
	Day                   int64
	mailSeen              model.MailID
	mailReady             bool
}

type ArtifactWizardStage uint8

const (
	ArtifactWizardID ArtifactWizardStage = iota
	ArtifactWizardRarity
	ArtifactWizardStartingCount
	ArtifactWizardStartingStats
	ArtifactWizardUpgradeCount
	ArtifactWizardUpgradeStats
	ArtifactWizardInventory
	ArtifactWizardCopies
)

type ArtifactWizard struct {
	ArtifactID    model.ArtifactID
	Template      int
	StartingCount int
	StartingStats []string
	UpgradeCount  int
	UpgradeStats  []string
	Stage         ArtifactWizardStage
}

type reply struct {
	method string
	args   []any
}

type Outbox []reply

func (o *Outbox) Add(method string, args ...any) { *o = append(*o, reply{method, args}) }

func (p *Player) Flush(o Outbox) error {
	for _, r := range o {
		if err := p.Send(r.method, r.args...); err != nil {
			return err
		}
	}
	return nil
}

func RandomToken() string {
	var token [16]byte
	rand.Read(token[:])
	return hex.EncodeToString(token[:])
}

func (p *Player) Send(method string, args ...any) error {
	payload, err := actor.RPC(method, args...)
	if err != nil {
		return err
	}
	return p.Session.Send(payload)
}

func (p *Player) ShowRewards(rewards []model.ItemReward) error {
	return p.Send("showReward", []model.RewardID{}, rewards, 1)
}

func (p *Player) ShowRewardsIfAny(rewards []model.ItemReward) error {
	if len(rewards) == 0 {
		return nil
	}
	return p.ShowRewards(rewards)
}

func (p *Player) Update(path []string, value any) error {
	payloads, err := p.properties.Synchronize(path, value)
	if err != nil {
		return err
	}
	for _, payload := range payloads {
		if err = p.Session.Send(payload); err != nil {
			return err
		}
	}
	return nil
}

func (p *Player) Opponent() *Player {
	if p.Room.Players[0] == p {
		return p.Room.Players[1]
	}
	return p.Room.Players[0]
}
