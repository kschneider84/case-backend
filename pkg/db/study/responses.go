package study

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	studyTypes "github.com/case-framework/case-backend/pkg/study/types"
)

const (
	idxResponsesParticipantID               = "participantID_1"
	idxResponsesParticipantIDKeySubmittedAt = "participantID_1_key_1_submittedAt_1"
	idxResponsesSubmittedAt                 = "submittedAt_1"
	idxResponsesArrivedAt                   = "arrivedAt_1"
	idxResponsesKey                         = "key_1"
)

var defaultResponseIndexNames = []string{
	idxResponsesParticipantID,
	idxResponsesParticipantIDKeySubmittedAt,
	idxResponsesSubmittedAt,
	idxResponsesArrivedAt,
	idxResponsesKey,
}

var indexesForResponsesCollection = []mongo.IndexModel{
	{
		Keys: bson.D{
			{Key: "participantID", Value: 1},
		},
		Options: options.Index().SetName(idxResponsesParticipantID),
	},
	{
		Keys: bson.D{
			{Key: "participantID", Value: 1},
			{Key: "key", Value: 1},
			{Key: "submittedAt", Value: 1},
		},
		Options: options.Index().SetName(idxResponsesParticipantIDKeySubmittedAt),
	},
	{
		Keys: bson.D{
			{Key: "submittedAt", Value: 1},
		},
		Options: options.Index().SetName(idxResponsesSubmittedAt),
	},
	{
		Keys: bson.D{
			{Key: "arrivedAt", Value: 1},
		},
		Options: options.Index().SetName(idxResponsesArrivedAt),
	},
	{
		Keys: bson.D{
			{Key: "key", Value: 1},
		},
		Options: options.Index().SetName(idxResponsesKey),
	},
}

func (dbService *StudyDBService) DropIndexForResponsesCollection(instanceID string, studyKey string, dropAll bool) {
	ctx, cancel := dbService.getContext()
	defer cancel()

	if dropAll {
		err := dbService.collectionResponses(instanceID, studyKey).Indexes().DropAll(ctx)
		if err != nil {
			slog.Error("Error dropping all indexes for responses", slog.String("error", err.Error()), slog.String("instanceID", instanceID), slog.String("studyKey", studyKey))
		}
	} else {
		for _, indexName := range defaultResponseIndexNames {
			if indexName == "" {
				slog.Error("Index name is empty for responses collection")
				continue
			}
			err := dbService.collectionResponses(instanceID, studyKey).Indexes().DropOne(ctx, indexName)
			if err != nil {
				slog.Error("Error dropping index for responses", slog.String("error", err.Error()), slog.String("instanceID", instanceID), slog.String("studyKey", studyKey), slog.String("indexName", indexName))
			}
		}
	}
}

func (dbService *StudyDBService) CreateDefaultIndexesForResponsesCollection(instanceID string, studyKey string) {
	ctx, cancel := dbService.getContext()
	defer cancel()

	collection := dbService.collectionResponses(instanceID, studyKey)
	_, err := collection.Indexes().CreateMany(ctx, indexesForResponsesCollection)
	if err != nil {
		slog.Error("Error creating index for responses", slog.String("error", err.Error()), slog.String("instanceID", instanceID), slog.String("studyKey", studyKey))
	}
}

func (dbService *StudyDBService) AddSurveyResponse(instanceID string, studyKey string, response studyTypes.SurveyResponse) (string, error) {
	ctx, cancel := dbService.getContext()
	defer cancel()

	if response.ArrivedAt == 0 {
		response.ArrivedAt = time.Now().Unix()
	}
	res, err := dbService.collectionResponses(instanceID, studyKey).InsertOne(ctx, response)
	if err != nil {
		return "", err
	}
	id := res.InsertedID.(bson.ObjectID)
	return id.Hex(), nil
}

// get response by id
func (dbService *StudyDBService) GetResponseByID(instanceID string, studyKey string, responseID string) (response studyTypes.SurveyResponse, err error) {
	ctx, cancel := dbService.getContext()
	defer cancel()

	_id, err := bson.ObjectIDFromHex(responseID)
	if err != nil {
		return response, err
	}

	filter := bson.M{
		"_id": _id,
	}

	err = dbService.collectionResponses(instanceID, studyKey).FindOne(ctx, filter).Decode(&response)
	return response, err
}

// get paginated responses by query
func (dbService *StudyDBService) GetResponses(instanceID string, studyKey string, filter bson.M, sort bson.M, page int64, limit int64) (responses []studyTypes.SurveyResponse, paginationInfo *PaginationInfos, err error) {
	ctx, cancel := dbService.getContext()
	defer cancel()

	totalCount, err := dbService.GetResponsesCount(instanceID, studyKey, filter)
	if err != nil {
		return responses, nil, err
	}

	paginationInfo = prepPaginationInfos(
		totalCount,
		page,
		limit,
	)

	skip := (paginationInfo.CurrentPage - 1) * paginationInfo.PageSize

	opts := options.Find().SetSort(sort).SetSkip(skip).SetLimit(paginationInfo.PageSize)
	collection := dbService.collectionResponses(instanceID, studyKey)
	cursor, err := collection.Find(ctx, filter, opts)
	if err != nil {
		return responses, nil, err
	}
	defer cursor.Close(ctx)

	err = cursor.All(ctx, &responses)
	if err != nil {
		return responses, nil, err
	}

	return responses, paginationInfo, nil
}

// get responses count by query
func (dbService *StudyDBService) GetResponsesCount(instanceID string, studyKey string, filter bson.M) (int64, error) {
	ctx, cancel := dbService.getContext()
	defer cancel()

	return dbService.collectionResponses(instanceID, studyKey).CountDocuments(ctx, filter)
}

type ResponseInfo struct {
	ID            bson.ObjectID `bson:"_id,omitempty" json:"id,omitempty"`
	Key           string        `bson:"key" json:"key"`
	ParticipantID string        `bson:"participantID" json:"participantId"`
	VersionID     string        `bson:"versionID" json:"versionId"`
	ArrivedAt     int64         `bson:"arrivedAt" json:"arrivedAt"`
}

func (dbService *StudyDBService) GetResponseInfos(instanceID string, studyKey string, filter bson.M, page int64, limit int64) (responseInfos []ResponseInfo, paginationInfo *PaginationInfos, err error) {
	ctx, cancel := dbService.getContext()
	defer cancel()

	count, err := dbService.GetResponsesCount(instanceID, studyKey, filter)
	if err != nil {
		return responseInfos, nil, err
	}

	paginationInfo = prepPaginationInfos(
		count,
		page,
		limit,
	)

	skip := (paginationInfo.CurrentPage - 1) * paginationInfo.PageSize

	sortBySubmittedAt := bson.D{
		{Key: "submittedAt", Value: -1},
	}

	opts := options.Find()
	opts.SetSort(sortBySubmittedAt)
	opts.SetSkip(skip)
	opts.SetLimit(paginationInfo.PageSize)

	projection := bson.D{
		{Key: "_id", Value: 1},
		{Key: "key", Value: 1},
		{Key: "participantID", Value: 1},
		{Key: "versionID", Value: 1},
		{Key: "arrivedAt", Value: 1},
	}
	opts.SetProjection(projection)

	cursor, err := dbService.collectionResponses(instanceID, studyKey).Find(ctx, filter, opts)
	if err != nil {
		return responseInfos, nil, err
	}
	defer cursor.Close(ctx)

	err = cursor.All(ctx, &responseInfos)
	return responseInfos, paginationInfo, err
}

// execute on responses by query
func (dbService *StudyDBService) FindAndExecuteOnResponses(
	ctx context.Context,
	instanceID string, studyKey string,
	filter bson.M,
	sort bson.M,
	returnOnError bool,
	fn func(dbService *StudyDBService, r studyTypes.SurveyResponse, instanceID string, studyKey string, args ...interface{}) error,
	args ...interface{},
) error {
	opts := options.Find().SetSort(sort)

	cursor, err := dbService.collectionResponses(instanceID, studyKey).Find(ctx, filter, opts)
	if err != nil {
		return err
	}

	defer cursor.Close(ctx)

	for cursor.Next(ctx) {
		var response studyTypes.SurveyResponse
		if err = cursor.Decode(&response); err != nil {
			slog.Error("Error while decoding response", slog.String("error", err.Error()))
			continue
		}

		if err = fn(dbService, response, instanceID, studyKey, args...); err != nil {
			slog.Error("Error while executing function on response", slog.String("responseID", response.ID.Hex()), slog.String("error", err.Error()))
			if returnOnError {
				return err
			}
			continue
		}
	}
	return nil
}

// delete response by id
func (dbService *StudyDBService) DeleteResponseByID(instanceID string, studyKey string, responseID string) error {
	ctx, cancel := dbService.getContext()
	defer cancel()

	_id, err := bson.ObjectIDFromHex(responseID)
	if err != nil {
		return err
	}

	filter := bson.M{
		"_id": _id,
	}

	res, err := dbService.collectionResponses(instanceID, studyKey).DeleteOne(ctx, filter)
	if err != nil {
		return err
	}
	if res.DeletedCount == 0 {
		return mongo.ErrNoDocuments
	}

	return err
}

func (dbService *StudyDBService) UpdateParticipantIDonResponses(instanceID string, studyKey string, oldID string, newID string) (count int64, err error) {
	ctx, cancel := dbService.getContext()
	defer cancel()

	if oldID == "" || newID == "" {
		return 0, errors.New("participant id must be defined")
	}
	filter := bson.M{"participantID": oldID}
	update := bson.M{"$set": bson.M{"participantID": newID}}

	res, err := dbService.collectionResponses(instanceID, studyKey).UpdateMany(ctx, filter, update)
	return res.ModifiedCount, err
}

// UpdateResponseSessionContext reassigns or removes the session label on all responses for a participant
// that match oldSession. If newSession is empty, the "session" key will be set to "" in the context map.
func (dbService *StudyDBService) UpdateResponseSessionContext(instanceID string, studyKey string, participantID string, oldSession string, newSession string) (count int64, err error) {
	ctx, cancel := dbService.getContext()
	defer cancel()

	if oldSession == "" {
		return 0, errors.New("oldSession must not be empty")
	}

	filter := bson.M{
		"participantID":    participantID,
		"context.session":  oldSession,
	}

	var update bson.M
	if newSession == "" {
		update = bson.M{"$set": bson.M{"context.session": ""}}
	} else {
		update = bson.M{"$set": bson.M{"context.session": newSession}}
	}

	res, err := dbService.collectionResponses(instanceID, studyKey).UpdateMany(ctx, filter, update)
	if err != nil {
		return 0, err
	}
	return res.ModifiedCount, nil
}

// delete responses by query
func (dbService *StudyDBService) DeleteResponses(instanceID string, studyKey string, filter bson.M) error {
	ctx, cancel := dbService.getContext()
	defer cancel()

	res, err := dbService.collectionResponses(instanceID, studyKey).DeleteMany(ctx, filter)
	if err != nil {
		return err
	}
	if res.DeletedCount == 0 {
		return mongo.ErrNoDocuments
	}

	return err
}
