package friend

import (
	"mmm-revived/internal/model"
	"mmm-revived/internal/protocol"
	"mmm-revived/internal/rpc"
	"mmm-revived/internal/server"
)

type service struct{ *server.Service }

func Register(srv *server.Service, r *server.Router) {
	s := service{srv}
	r.Handle("applyForFriend", rpc.Bind(DecodeFriendApplication, s.applyForFriend))
	r.Handle("acceptApplyForFriend", rpc.Bind(DecodeFriendDecision, s.decideFriend))
	r.Handle("acceptAllFriendApply", rpc.Bind(DecodePlayers, s.acceptFriends))
	r.Handle("rejectAllFriendApply", rpc.Bare(s.rejectFriends))
	r.Handle("delFriend", rpc.Bind(DecodePlayers, s.removeFriends))
	r.Handle("addPlayer2Blacklist", rpc.Bind(protocol.DecodePlayer, func(p *server.Player, target model.UserID) error {
		return s.updateFriends(p, s.Store.Social.Block(p.Account.UID, target), p.Account.UID, target)
	}))
	r.Handle("removeFromBlacklist", rpc.Bind(protocol.DecodePlayer, func(p *server.Player, target model.UserID) error {
		return s.updateFriends(p, s.Store.Social.Unblock(p.Account.UID, target), p.Account.UID)
	}))
	r.Handle("changeFriendNickName", rpc.Bind(DecodeFriendName, func(p *server.Player, r FriendName) error {
		return s.updateFriends(p, s.Store.Social.Rename(p.Account.UID, r.Target, r.Name), p.Account.UID)
	}))
	r.Handle("searchPlayerById", rpc.Bind(DecodeSearchID, func(p *server.Player, target model.UserID) error {
		return s.findPlayers(p, target, "", false)
	}))
	r.Handle("searchPlayerByName", rpc.Bind(DecodeSearchName, func(p *server.Player, name string) error {
		return s.findPlayers(p, 0, name, false)
	}))
	r.Handle("getRecommendFriends", rpc.Bare(func(p *server.Player) error { return s.findPlayers(p, 0, "", true) }))
	r.Handle("sendGift2Friends", rpc.Bind(DecodePlayers, s.sendGifts))
	r.Handle("recvFriendGifts", rpc.Bind(DecodePlayers, s.claimGifts))
	r.Handle("acqChatMsg", rpc.Bind(DecodeChatPeer, func(p *server.Player, peer model.UserID) error {
		if peer == model.HelperUID {
			return nil
		}
		return s.Store.Social.Acknowledge(p.Account.UID, peer)
	}))
	r.Handle("sendChat2Channel", rpc.Bind(DecodeChat, s.sendChat))
}

func Sync(srv *server.Service, players ...model.UserID) error {
	return service{srv}.syncSocial(players...)
}
