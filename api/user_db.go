package main

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func (c apiConfig) createUser(username string, password string, ctx context.Context) error {
	userCollection := c.DB.Collection("users")

	user := User{
		Username: username,
		Password: password,
	}

	_, err := userCollection.InsertOne(ctx, user)

	if err != nil {
		return err
	}

	return nil
}

func (c apiConfig) getUserByEmail(username string, ctx context.Context) (*User, error) {
	var user User
	userCollection := c.DB.Collection("users")

	err := userCollection.FindOne(ctx, bson.M{"username": username}).Decode(&user)
	if err != nil {
		return nil, err
	}

	return &user, nil
}

func (c apiConfig) addProject(ctx context.Context, projectID primitive.ObjectID, userID primitive.ObjectID) error {
	userCollection := c.DB.Collection("users")
	filter := bson.M{"_id": userID}
	update := bson.D{
		{"$push", bson.D{
			{"projects", projectID},
		}},
	}

	result := userCollection.FindOneAndUpdate(ctx, filter, update)
	if err := result.Err(); err != nil {
		return err
	}

	return nil
}
