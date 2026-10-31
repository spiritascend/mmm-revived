package social

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"mmm-revived/internal/calendar"
	"mmm-revived/internal/database"
	"mmm-revived/internal/model"
	"mmm-revived/internal/storage"
)

const daySeconds = 86400

var ErrRejected = errors.New("social request rejected")

const (
	CodeCommon             = 5
	CodeNoApplication      = 8
	CodeInvalidPlayer      = 9
	CodeGuildDeleted       = 101
	CodeTargetNotInGuild   = 102
	CodeAlreadyInGuild     = 103
	CodeNotInGuild         = 104
	CodeNotAdmin           = 105
	CodeNotBoss            = 106
	CodeTransferFirst      = 107
	CodeTargetNotAdmin     = 108
	CodeTargetIsAdmin      = 109
	CodeGuildFull          = 110
	CodeApplicationMissing = 111
	CodeTargetInOtherGuild = 112
	CodeInvalidGuildName   = 113
	CodeInvalidSignature   = 114
	CodeInvalidAnnounce    = 115
	CodeGuildNameTaken     = 116
	CodeApplicationLimit   = 117
	CodeJoinCooldown       = 118
	CodeInviteOnly         = 119
	CodeCreationCost       = 120
	CodeLevelTooLow        = 121
	CodeInviteInvalid      = 122
	CodeInAnotherGuild     = 123
	CodeNotFriend          = 125
	CodeAlreadyFriend      = 126
	CodeFriendsFull        = 127
	CodeTargetBlocked      = 128
	CodeTargetFriendsFull  = 129
	CodeBlockedByTarget    = 130
	CodeAdminLimit         = 142
	CodeInviteLimit        = 143
	CodeAlreadyLeft        = 145
	CodePartialAccept      = 148
	CodeMessageTooLong     = 162
)

type Rejection struct {
	Code    int
	Args    []int64
	message string
}

func (e *Rejection) Error() string { return ErrRejected.Error() + ": " + e.message }

func (e *Rejection) Unwrap() error { return ErrRejected }

func Rejected(err error) (*Rejection, bool) {
	if rejection, ok := errors.AsType[*Rejection](err); ok {
		return rejection, true
	}
	if errors.Is(err, ErrRejected) {
		return &Rejection{Code: CodeCommon, message: err.Error()}, true
	}
	return nil, false
}

func reject(code int, message string, args ...int64) error {
	return &Rejection{Code: code, Args: args, message: message}
}

type Limits struct {
	Friends       int
	Applications  int
	SentGifts     int
	ReceivedGifts int
	GiftEnergy    int64
	GiftItem      model.ItemID
	History       int
	Blocked       int
	Message       int
}

const messageHistory = 200

type Config struct {
	Limits          Limits
	GuildLimits     GuildLimits
	GuildLevels     []GuildLevel
	Wars            []WarPeriod
	SettlementTexts [3]string
	SettlementMail  int64
}

type Repository struct {
	GuildLimits GuildLimits
	db          *storage.Database
	Limits      Limits
	progress    guildProgress
	levels      []GuildLevel
	wars        []WarPeriod
	settlement  [3]string
	settleMail  int64
}

func Open(db *storage.Database) (*Repository, error) {
	return &Repository{db: db, Limits: Limits{History: messageHistory}, levels: []GuildLevel{{}}}, nil
}

func (r *Repository) Configure(c Config) {
	r.Limits, r.GuildLimits, r.wars, r.settlement, r.settleMail = c.Limits, c.GuildLimits, c.Wars, c.SettlementTexts, c.SettlementMail
	r.Limits.History = messageHistory
	r.levels = append([]GuildLevel{{}}, c.GuildLevels...)
}

var patternEscaper = strings.NewReplacer("[", "[[]", "*", "[*]", "?", "[?]")

func searchPattern(name string) string {
	return patternEscaper.Replace(strings.ToLower(name)) + "*"
}

func today() int64 { return calendar.LogicDay(time.Now().Unix()) }

func (r *Repository) transaction(operation func(*sql.Tx) error) error {
	tx, err := r.db.SQL.BeginTx(context.Background(), nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = operation(tx); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *Repository) ClosePresence(now int64) error {
	_, err := r.db.SQL.ExecContext(context.Background(), "UPDATE presence SET offline=? WHERE online>=offline", now)
	return err
}

func (r *Repository) Presence(uid model.UserID, online bool) error {
	ctx, now := context.Background(), time.Now().Unix()
	if online {
		return r.db.Queries.SetPresenceOnline(ctx, database.SetPresenceOnlineParams{UID: uid, Time: now})
	}
	return r.db.Queries.SetPresenceOffline(ctx, database.SetPresenceOfflineParams{UID: uid, Time: now})
}

func (r *Repository) Snapshot(uid model.UserID) (model.FriendComponent, error) {
	result := model.FriendComponent{Friends: map[model.UserID]model.Friend{}, Applications: map[model.UserID]model.FriendApplication{}, Blocked: []model.UserID{}, Gifts: map[model.UserID]int{}, Sent: []model.UserID{}, SupportRecord: map[model.UserID]map[model.HeroID]int{}}
	ctx, q := context.Background(), r.db.Queries
	friends, err := q.ListFriends(ctx, uid)
	if err != nil {
		return result, err
	}
	for _, row := range friends {
		result.Friends[row.Peer] = model.Friend{Nickname: row.Nickname, Added: row.Added}
	}
	result.Friends[model.HelperUID] = model.Friend{Nickname: "Helper"}
	applications, err := q.ListApplications(ctx, uid)
	if err != nil {
		return result, err
	}
	for _, row := range applications {
		result.Applications[row.Sender] = model.FriendApplication{Source: row.Source, Created: row.Created}
	}
	if result.Blocked, err = q.ListBlocks(ctx, uid); err != nil {
		return result, err
	}
	day := today()
	gifts, err := q.ListReceivedGifts(ctx, database.ListReceivedGiftsParams{Receiver: uid, Day: day})
	if err != nil {
		return result, err
	}
	for _, row := range gifts {
		if row.Claimed != 0 {
			result.Received++
		} else {
			result.Gifts[row.Sender]++
		}
	}
	sent, err := q.ListSentGifts(ctx, database.ListSentGiftsParams{Sender: uid, Day: day})
	if err != nil {
		return result, err
	}
	result.Sent = append(result.Sent, sent...)
	times, err := q.GetSocialTimes(ctx, uid)
	result.LastSearch, result.LastRecommend = times.Search, times.Recommend
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return result, err
	}
	usage, err := q.ListSupportUsage(ctx, uid)
	if err != nil {
		return result, err
	}
	for _, row := range usage {
		if row.Uses <= row.Claimed {
			continue
		}
		heroes := result.SupportRecord[row.Borrower]
		if heroes == nil {
			heroes = map[model.HeroID]int{}
			result.SupportRecord[row.Borrower] = heroes
		}
		heroes[row.Hero] += int(row.Uses - row.Claimed)
	}
	result.LastSupportRefresh, err = q.GetSupportRefresh(ctx, uid)
	if errors.Is(err, sql.ErrNoRows) {
		err = nil
	}
	return result, err
}
