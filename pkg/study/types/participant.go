package types

import "go.mongodb.org/mongo-driver/bson/primitive"

const (
	PARTICIPANT_STUDY_STATUS_ACTIVE          = "active"
	PARTICIPANT_STUDY_STATUS_TEMPORARY       = "temporary" // for participants without a registered account
	PARTICIPANT_STUDY_STATUS_VIRTUAL         = "virtual"   // for participants created by study managers
	PARTICIPANT_STUDY_STATUS_EXITED          = "exited"
	PARTICIPANT_STUDY_STATUS_ACCOUNT_DELETED = "accountDeleted"
)

// Participant defines the datamodel for current state of the participant in a study as stored in the database
type Participant struct {
	ID                  primitive.ObjectID   `bson:"_id,omitempty" json:"id,omitempty"`
	ParticipantID       string               `bson:"participantID" json:"participantId"` // reference to the study specific participant ID
	CurrentStudySession string               `bson:"currentStudySession" json:"currentStudySession"`
	ModifiedAt          int64                `bson:"modifiedAt" json:"modifiedAt"`
	EnteredAt           int64                `bson:"enteredAt" json:"enteredAt"`
	StudyStatus         string               `bson:"studyStatus" json:"studyStatus"`
	Flags               map[string]string    `bson:"flags" json:"flags"`
	LinkingCodes        map[string]string    `bson:"linkingCodes" json:"linkingCodes"`
	AssignedSurveys     []AssignedSurvey     `bson:"assignedSurveys" json:"assignedSurveys"`
	LastSubmissions     map[string]int64     `bson:"lastSubmission" json:"lastSubmissions"` // surveyKey with timestamp
	Messages            []ParticipantMessage `bson:"messages" json:"messages"`
	HashedAccountID     *string              `bson:"hashedAccountID,omitempty" json:"hashedAccountID,omitempty"`
	IsMainProfile       *bool                `bson:"isMainProfile,omitempty" json:"isMainProfile,omitempty"`
}

type ParticipantMessage struct {
	ID           string `bson:"id" json:"id"`
	Type         string `bson:"type" json:"type"`
	ScheduledFor int64  `bson:"scheduledFor" json:"scheduledFor"`
}
