package model

type FriendComponent struct {
	LastSearch         int64                        `wire:"lastSearchTime"`
	LastRecommend      int64                        `wire:"lastRecommendTime"`
	LastSupportRefresh int64                        `wire:"lastRefreshSupportTime"`
	Friends            map[UserID]Friend            `wire:"friendDataDict"`
	Applications       map[UserID]FriendApplication `wire:"applyDict"`
	Blocked            []UserID                     `wire:"blacklist"`
	Gifts              map[UserID]int               `wire:"recvGift"`
	Received           int                          `wire:"todayCountOfRecv"`
	Sent               []UserID                     `wire:"todayCountOfSendGift"`
	SupportRecord      map[UserID]map[HeroID]int    `wire:"supportRewardRecord"`
}

// HelperUID identifies the local chat contact and is never stored as an account.
const HelperUID UserID = 1

type Friend struct {
	Nickname string `wire:"nickName"`
	Added    int64  `wire:"time"`
}
type FriendApplication struct {
	Source  FriendSource `wire:"source"`
	Created int64        `wire:"time"`
}

type FriendSource int64

const (
	FriendSourceNone FriendSource = iota
	FriendSourceBattle
	FriendSourceSearch
	FriendSourceRecommend
	FriendSourceMessage
	FriendSourceRank
	FriendSourceChat
)

var friendSourceNames = [...]string{"none", "battle", "search", "recommend", "message", "Rank", "chat"}

func ParseFriendSource(value string) (FriendSource, bool) {
	for source, name := range friendSourceNames {
		if value == name {
			return FriendSource(source), true
		}
	}
	return FriendSourceNone, false
}

func (s FriendSource) Valid() bool {
	return s >= FriendSourceNone && s <= FriendSourceChat
}

func (s FriendSource) WireValue() any {
	if !s.Valid() {
		return friendSourceNames[FriendSourceNone]
	}
	return friendSourceNames[s]
}

type ChatChannel int

const (
	PrivateChat ChatChannel = 1 + iota
	GuildChat
	WorldChat
)

type ChatMessage struct {
	ID       string `wire:"msgId"`
	Sender   UserID `wire:"senderId"`
	Receiver UserID `wire:"targetId"`
	Kind     int    `wire:"type"`
	Text     string `wire:"msg"`
	Time     int64  `wire:"time"`
}

type GuildComponent struct {
	ID            GuildID           `wire:"guildId"`
	Title         GuildRole         `wire:"guildTitle"`
	NameUpdated   int64             `wire:"guildNameUpdateTime"`
	LastSearch    int64             `wire:"lastSearchTime"`
	Name          string            `wire:"guildName"`
	Level         int               `wire:"guildLv"`
	Experience    int64             `wire:"guildExp"`
	Invites       map[GuildID]int64 `wire:"guildInvites"`
	Applications  map[GuildID]int64 `wire:"joinApply"`
	Members       []UserID          `wire:"guildMembersList"`
	WarRank       int               `wire:"guildWarRankId"`
	WarID         int               `wire:"warId"`
	WarPhase      int               `wire:"warPhase"`
	BossKilled    int64             `wire:"finalBossKillTime"`
	Head          int               `wire:"guildHead"`
	Frame         int               `wire:"guildFrame"`
	LastLeft      int64             `wire:"lastLeaveGuildTime"`
	TaskRefreshed int64             `wire:"lastRefreshTaskTime"`
}

type MailComponent struct {
	SurveyRewards map[int]int `wire:"surveyRewards"`
}

type HistoryComponent struct {
	Ranks     map[string]RankEntry `wire:"rankRec"`
	Cooldowns map[string]int64     `wire:"rankCdRecord"`
}

type InviteComponent struct {
	RewardCounts map[int]int            `wire:"boundIndexMaxRwd"`
	BoundCode    string                 `wire:"bindCode"`
	Code         string                 `wire:"myCode"`
	BoundAt      map[UserID]int64       `wire:"bindPlayerTime"`
	Tasks        map[UserID]map[int]int `wire:"boundPlayerTasks"`
}

type QuestionComponent struct {
	Questions string `wire:"questionData"`
}

type PVPComponent struct {
	Score        int                      `wire:"score"`
	MaxScore     int                      `wire:"maxScore"`
	Rank         int                      `wire:"rankId"`
	Activity     ActivityID               `wire:"activityId"`
	Environment  int                      `wire:"environmentId"`
	Camp         int                      `wire:"campEnvId"`
	Info         map[HeroID]AsyncPosition `wire:"pvp_info"`
	InfoList     []HeroID                 `wire:"pvp_info_list"`
	Targets      []AsyncTarget            `wire:"targets"`
	Friends      map[UserID]AsyncTarget   `wire:"friendTargets"`
	Battles      []string                 `wire:"battleRecord"`
	FriendGames  []string                 `wire:"friendBattleRecord"`
	Rewards      []int                    `wire:"rwdRankLvList"`
	Environments []int                    `wire:"validEnvList"`
	Interactions map[int]AsyncInteraction `wire:"interact_object_info"`
	Attempts     int                      `wire:"canAtk"`
	Refreshed    int                      `wire:"refreshed"`
	LastRequest  int64                    `wire:"lastReq"`
}
