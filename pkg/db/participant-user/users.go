package participantuser

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	umTypes "github.com/case-framework/case-backend/pkg/user-management/types"
)

var indexesForParticipantUsersCollection = []mongo.IndexModel{
	{
		Keys: bson.D{
			{Key: "timestamps.markedForDeletion", Value: 1},
		},
		Options: options.Index().SetName("timestamps.markedForDeletion_1"),
	},
	{
		Keys: bson.D{
			{Key: "account.accountID", Value: 1},
		},
		Options: options.Index().SetUnique(true).
			SetName("uniq_account.accountID_1"),
	},
	{
		Keys: bson.D{
			{Key: "timestamps.createdAt", Value: 1},
		},
		Options: options.Index().SetName("timestamps.createdAt_1"),
	},
	{
		Keys: bson.D{
			{Key: "account.accountConfirmedAt", Value: 1},
			{Key: "timestamps.createdAt", Value: 1},
		},
		Options: options.Index().SetName("account.accountConfirmedAt_1_timestamps.createdAt_1"),
	},
	{
		Keys: bson.D{
			{Key: "contactPreferences.receiveWeeklyMessageDayOfWeek", Value: 1},
		},
		Options: options.Index().SetName("contactPreferences.receiveWeeklyMessageDayOfWeek_1"),
	},
}

func (dbService *ParticipantUserDBService) DropIndexForParticipantUsersCollection(instanceID string, dropAll bool) {
	ctx, cancel := dbService.getContext()
	defer cancel()

	if dropAll {
		_, err := dbService.collectionParticipantUsers(instanceID).Indexes().DropAll(ctx)
		if err != nil {
			slog.Error("Error dropping all indexes for participant users", slog.String("error", err.Error()))
		}
	} else {
		for _, index := range indexesForParticipantUsersCollection {
			if index.Options == nil || index.Options.Name == nil {
				slog.Error("Index name is nil for participant users collection", slog.String("index", fmt.Sprintf("%+v", index)))
				continue
			}
			indexName := *index.Options.Name
			_, err := dbService.collectionParticipantUsers(instanceID).Indexes().DropOne(ctx, indexName)
			if err != nil {
				slog.Error("Error dropping index for participant users", slog.String("error", err.Error()), slog.String("indexName", indexName))
			}
		}
	}
}

func (dbService *ParticipantUserDBService) CreateDefaultIndexesForParticipantUsersCollection(instanceID string) {
	ctx, cancel := dbService.getContext()
	defer cancel()

	_, err := dbService.collectionParticipantUsers(instanceID).Indexes().CreateMany(
		ctx, indexesForParticipantUsersCollection,
	)
	if err != nil {
		slog.Error("Error creating index for participant users", slog.String("error", err.Error()))
	}
}

func (dbService *ParticipantUserDBService) FixFieldNameForContactInfos(instanceID string) error {
	ctx, cancel := dbService.getContext()
	defer cancel()

	collection := dbService.collectionParticipantUsers(instanceID)
	filter := bson.M{
		"contactinfos": bson.M{
			"$exists": true,
		},
	}
	update := bson.M{"$rename": bson.M{"contactinfos": "contactInfos"}}
	_, err := collection.UpdateMany(ctx, filter, update)
	return err
}

func (dbService *ParticipantUserDBService) AddUser(instanceID string, user umTypes.User) (id string, err error) {
	ctx, cancel := dbService.getContext()
	defer cancel()

	filter := bson.M{"account.accountID": user.Account.AccountID}
	upsert := true
	opts := options.UpdateOptions{
		Upsert: &upsert,
	}
	res, err := dbService.collectionParticipantUsers(instanceID).UpdateOne(ctx, filter, bson.M{
		"$setOnInsert": user,
	}, &opts)
	if err != nil {
		return
	}

	if res.UpsertedCount < 1 {
		err = errors.New("user already exists")
		return
	}

	id = res.UpsertedID.(primitive.ObjectID).Hex()
	return
}

func (dbService *ParticipantUserDBService) GetUser(instanceID, objectID string) (umTypes.User, error) {
	ctx, cancel := dbService.getContext()
	defer cancel()

	_id, err := primitive.ObjectIDFromHex(objectID)
	if err != nil {
		return umTypes.User{}, err
	}

	var user umTypes.User
	filter := bson.M{"_id": _id}
	err = dbService.collectionParticipantUsers(instanceID).FindOne(ctx, filter).Decode(&user)
	return user, err
}

func (dbService *ParticipantUserDBService) GetUserByAccountID(instanceID, accountID string) (umTypes.User, error) {
	ctx, cancel := dbService.getContext()
	defer cancel()

	var user umTypes.User
	filter := bson.M{"account.accountID": accountID}
	err := dbService.collectionParticipantUsers(instanceID).FindOne(ctx, filter).Decode(&user)
	return user, err
}

func (dbService *ParticipantUserDBService) GetUserByProfileID(instanceID, profileID string) (umTypes.User, error) {
	ctx, cancel := dbService.getContext()
	defer cancel()

	var user umTypes.User
	_profileID, err := primitive.ObjectIDFromHex(profileID)
	if err != nil {
		return umTypes.User{}, err
	}
	filter := bson.M{"profiles._id": _profileID}
	err = dbService.collectionParticipantUsers(instanceID).FindOne(ctx, filter).Decode(&user)
	return user, err
}

func (dbService *ParticipantUserDBService) SaveFailedLoginAttempt(instanceID string, userID string) error {
	ctx, cancel := dbService.getContext()
	defer cancel()

	_id, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return err
	}

	filter := bson.M{"_id": _id}
	update := bson.M{
		"$push": bson.M{
			"account.failedLoginAttempts": time.Now().Unix(),
		},
	}
	_, err = dbService.collectionParticipantUsers(instanceID).UpdateOne(ctx, filter, update)
	return err
}

// low level find and replace
func (dbService *ParticipantUserDBService) _updateUserInDB(orgID string, user umTypes.User) (umTypes.User, error) {
	ctx, cancel := dbService.getContext()
	defer cancel()

	elem := umTypes.User{}
	filter := bson.M{"_id": user.ID}
	rd := options.After
	fro := options.FindOneAndReplaceOptions{
		ReturnDocument: &rd,
	}
	err := dbService.collectionParticipantUsers(orgID).FindOneAndReplace(ctx, filter, user, &fro).Decode(&elem)
	return elem, err
}

func (dbService *ParticipantUserDBService) ReplaceUser(instanceID string, updatedUser umTypes.User) (umTypes.User, error) {
	// Set last update time
	updatedUser.Timestamps.UpdatedAt = time.Now().Unix()
	return dbService._updateUserInDB(instanceID, updatedUser)
}

func (dbService *ParticipantUserDBService) CountRecentlyCreatedUsers(instanceID string, interval int64) (count int64, err error) {
	ctx, cancel := dbService.getContext()
	defer cancel()

	filter := bson.M{"timestamps.createdAt": bson.M{"$gt": time.Now().Unix() - interval}}
	count, err = dbService.collectionParticipantUsers(instanceID).CountDocuments(ctx, filter)
	return
}

func (dbService *ParticipantUserDBService) DeleteUser(instanceID, userID string) error {
	ctx, cancel := dbService.getContext()
	defer cancel()

	_id, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return err
	}

	filter := bson.M{"_id": _id}
	res, err := dbService.collectionParticipantUsers(instanceID).DeleteOne(ctx, filter)
	if err != nil {
		return err
	}
	if res.DeletedCount < 1 {
		return errors.New("no user found with the given id")
	}

	if err = dbService.DeleteAllUserAttributes(ctx, instanceID, userID); err != nil {
		return err
	}

	// Clean up temporary rate-limit state last. An absent document is fine;
	// on failure, the TTL index will eventually remove the remaining document.
	if _, err = dbService.collectionPasswordResetLimits(instanceID).DeleteOne(ctx, filter); err != nil {
		slog.Error("failed to delete password reset limits after user deletion",
			slog.String("instanceID", instanceID), slog.String("userID", userID), slog.String("error", err.Error()))
	}
	return nil
}

func (dbService *ParticipantUserDBService) UpdateUser(instanceID string, userID string, update bson.M) error {
	ctx, cancel := dbService.getContext()
	defer cancel()

	_id, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return err
	}

	filter := bson.M{"_id": _id}
	_, err = dbService.collectionParticipantUsers(instanceID).UpdateOne(ctx, filter, update)
	return err
}

func (dbService *ParticipantUserDBService) FindAndExecuteOnUsers(
	ctx context.Context,
	instanceID string,
	filter bson.M,
	sort bson.M,
	returnOnError bool,
	fn func(user umTypes.User, args ...interface{}) error,
	args ...interface{},
) error {
	opts := options.Find().SetSort(sort).SetBatchSize(32)

	cursor, err := dbService.collectionParticipantUsers(instanceID).Find(ctx, filter, opts)
	if err != nil {
		return err
	}
	defer cursor.Close(ctx)

	for cursor.Next(ctx) {
		var user umTypes.User
		if err = cursor.Decode(&user); err != nil {
			return err
		}

		if err = fn(user, args...); err != nil {
			slog.Error("Error while executing function on user", slog.String("userID", user.ID.Hex()), slog.String("error", err.Error()))
			if returnOnError {
				return err
			}
			continue
		}
	}
	return nil
}
