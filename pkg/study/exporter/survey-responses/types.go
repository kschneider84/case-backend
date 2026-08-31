package surveyresponses

import studytypes "github.com/case-framework/case-backend/pkg/study/types"

type ParsedResponse struct {
	ID            string
	ParticipantID string
	OpenedAt      int64
	SubmittedAt   int64
	ArrivedAt     int64
	Version       string
	AccountID     string
	MainProfile   *bool
	Context       map[string]string // e.g. Language, or engine version
	Responses     map[string]interface{}
	Meta          ResponseMeta
}

// AccountTrackingInfo contains the pseudonymized account information stored
// on a participant state. AccountID is intentionally the hashed account ID.
type AccountTrackingInfo struct {
	AccountID   string
	MainProfile *bool
}

func AccountTrackingInfoFromParticipant(participant studytypes.Participant) AccountTrackingInfo {
	info := AccountTrackingInfo{MainProfile: participant.IsMainProfile}
	if participant.HashedAccountID != nil {
		info.AccountID = *participant.HashedAccountID
	}
	return info
}

type ResponseMeta struct {
	Initialised map[string][]int64
	Displayed   map[string][]int64
	Responded   map[string][]int64
	Position    map[string]int32
}

type IncludeMeta struct {
	Postion        bool
	InitTimes      bool
	DisplayedTimes bool
	ResponsedTimes bool
}

type ColumnNames struct {
	FixedColumns    []string
	ContextColumns  []string
	ResponseColumns []string
	MetaColumns     []string
}
