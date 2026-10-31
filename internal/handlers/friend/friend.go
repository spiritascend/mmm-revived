package friend

import (
	"database/sql"
	"errors"
	"fmt"
	"math"
	"time"

	"mmm-revived/internal/account"
	"mmm-revived/internal/handlers/guild"
	"mmm-revived/internal/model"
	"mmm-revived/internal/server"
	"mmm-revived/internal/social"
)

func (s service) syncSocial(players ...model.UserID) error {
	seen := make(map[model.UserID]bool, len(players))
	for _, uid := range players {
		if seen[uid] {
			continue
		}
		seen[uid] = true
		p := s.Online(uid)
		if p == nil {
			continue
		}
		state, err := s.Store.Social.Snapshot(uid)
		if err != nil {
			return err
		}
		if err = p.Update([]string{"friendComp"}, state); err != nil {
			p.Session.Logger.Warn("social update failed", "error", err)
			if err = p.Session.Conn.Close(); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s service) updateFriends(p *server.Player, err error, changed ...model.UserID) error {
	if err != nil {
		return guild.Reject(p, err)
	}
	if err = s.syncSocial(changed...); err != nil {
		return err
	}
	return server.Acknowledge(p)
}

func (s service) deliver(p *server.Player, message model.ChatMessage) error {
	if err := p.Send("notifyNewChatMsg", message); err != nil {
		return err
	}
	peer := message.Receiver
	if peer == p.Account.UID {
		peer = message.Sender
	}
	if target := s.Online(peer); target != nil {
		if err := target.Send("notifyNewChatMsg", message); err != nil {
			target.Session.Logger.Warn("chat delivery failed", "error", err)
			target.Session.Conn.Close()
		}
	}
	return nil
}

func (s service) applyForFriend(p *server.Player, r FriendApplication) error {
	err := s.Store.Social.Apply(p.Account.UID, r.Target, r.Source)
	if rejection, ok := social.Rejected(err); ok && rejection.Code == social.CodeInvalidPlayer {
		return p.Send("notifyApplySuccess", r.Target, false)
	}
	if err != nil {
		return guild.Reject(p, err)
	}
	if err = p.Send("notifyApplySuccess", r.Target, true); err != nil {
		return err
	}
	return s.syncSocial(p.Account.UID, r.Target)
}

func (s service) decideFriend(p *server.Player, r FriendDecision) error {
	welcome, err := s.Store.Social.Decide(p.Account.UID, r.Target, r.Accept)
	if err != nil {
		return guild.Reject(p, err)
	}
	if err = s.syncSocial(p.Account.UID, r.Target); err != nil {
		return err
	}
	if !r.Accept {
		return p.Send("onRejectFriendFinish", r.Target)
	}
	if err = s.deliver(p, welcome); err != nil {
		return err
	}
	return p.Send("onAcceptFriendFinish", r.Target)
}

func (s service) acceptFriends(p *server.Player, targets []model.UserID) error {
	changed := append(make([]model.UserID, 0, len(targets)+1), p.Account.UID)
	welcomes := make([]model.ChatMessage, 0, len(targets))
	failed := false
	for _, target := range targets {
		welcome, err := s.Store.Social.Decide(p.Account.UID, target, true)
		if err != nil && !errors.Is(err, social.ErrRejected) {
			return err
		}
		if err == nil {
			changed = append(changed, target)
			welcomes = append(welcomes, welcome)
		}
		failed = failed || err != nil
		if err = p.Send("onAcceptFriendFinishInMultiHandle", target, err == nil); err != nil {
			return err
		}
	}
	if err := s.syncSocial(changed...); err != nil {
		return err
	}
	for _, welcome := range welcomes {
		if err := s.deliver(p, welcome); err != nil {
			return err
		}
	}
	if failed {
		if err := p.Send("showErr", social.CodePartialAccept, []int64{}); err != nil {
			return err
		}
	}
	return p.Send("onAcceptAllFriendFinish")
}

func (s service) rejectFriends(p *server.Player) error {
	if err := s.Store.Social.RejectAll(p.Account.UID); err != nil {
		return err
	}
	if err := s.syncSocial(p.Account.UID); err != nil {
		return err
	}
	return p.Send("onRejectFriendFinish", 0)
}

func (s service) removeFriends(p *server.Player, targets []model.UserID) error {
	if err := s.Store.Social.Remove(p.Account.UID, targets); err != nil {
		return err
	}
	if err := s.syncSocial(append([]model.UserID{p.Account.UID}, targets...)...); err != nil {
		return err
	}
	return p.Send("onDelFriendFinish")
}

func (s service) findPlayers(p *server.Player, target model.UserID, name string, recommend bool) error {
	ids, err := s.Store.Social.Find(p.Account.UID, target, name, recommend)
	if err != nil {
		return err
	}
	if err = s.syncSocial(p.Account.UID); err != nil {
		return err
	}
	if recommend {
		return p.Send("notifyRecommendFriend", ids)
	}
	return p.Send("notifyFriendSearchResult", ids)
}

func (s service) sendGifts(p *server.Player, targets []model.UserID) error {
	err := s.Store.Update(p.Account, func(tx *sql.Tx) error {
		count, err := s.Store.Social.SendGiftsTx(tx, p.Account.UID, targets)
		if err != nil {
			return err
		}
		p.Account.RecordJourneyTaskEvent(s.Store.Rules, 47, count, 0)
		return nil
	})
	if err != nil {
		return guild.Reject(p, err)
	}
	if err = s.syncSocial(append([]model.UserID{p.Account.UID}, targets...)...); err != nil {
		return err
	}
	if err = s.Sync(p, "taskComp"); err != nil {
		return err
	}
	return p.Send("onSendGiftFinish")
}

func (s service) claimGifts(p *server.Player, targets []model.UserID) error {
	repo := s.Store.Social
	count := 0
	err := s.Store.Update(p.Account, func(tx *sql.Tx) error {
		var err error
		if count, err = repo.ClaimGifts(tx, p.Account.UID, targets); err != nil {
			return err
		}
		amount := int64(count) * repo.Limits.GiftEnergy
		if p.Account.Items[repo.Limits.GiftItem] > math.MaxInt64-amount {
			return fmt.Errorf("%w: energy overflow", account.ErrProgression)
		}
		p.Account.Items[repo.Limits.GiftItem] += amount
		return nil
	})
	if err != nil {
		return err
	}
	if err = s.Sync(p, "itemComp"); err != nil {
		return err
	}
	if err = s.syncSocial(p.Account.UID); err != nil {
		return err
	}
	if count == 0 {
		return server.Acknowledge(p)
	}
	return p.Send("onRecvFriendGift", repo.Limits.GiftItem, int64(count)*repo.Limits.GiftEnergy)
}

func (s service) sendChat(p *server.Player, r SendChat) error {
	switch r.Channel {
	case model.GuildChat:
		return guild.SendChat(s.Service, p, r.Kind, r.Text)
	case model.PrivateChat:
		if r.Target == model.HelperUID {
			return s.helperChat(p, r.Kind, r.Text)
		}
	default:
		if err := s.Store.Update(p.Account, func(*sql.Tx) error {
			p.Account.RecordJourneyTaskEvent(s.Store.Rules, 96, 1, 0)
			return nil
		}); err != nil {
			return err
		}
		if err := s.Sync(p, "taskComp"); err != nil {
			return err
		}
		return guild.SendWorldChat(s.Service, p, r.Kind, r.Text)
	}
	var message model.ChatMessage
	err := s.Store.Update(p.Account, func(tx *sql.Tx) error {
		var err error
		if message, err = s.Store.Social.SendPrivateTx(tx, model.ChatMessage{Sender: p.Account.UID, Receiver: r.Target, Kind: r.Kind, Text: r.Text, Time: time.Now().Unix()}); err != nil {
			return err
		}
		p.Account.RecordJourneyTaskEvent(s.Store.Rules, 46, 1, 0)
		return nil
	})
	if err != nil {
		return guild.Reject(p, err)
	}
	if err = s.Sync(p, "taskComp"); err != nil {
		return err
	}
	return s.deliver(p, message)
}
