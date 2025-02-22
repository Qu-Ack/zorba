package main

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

func (c apiConfig) insertDeployment(ctx context.Context, deployment *Deployment) (*Deployment, error) {
	deployments := c.DB.Collection("deployments")

	_, err := deployments.InsertOne(ctx, deployment)

	if err != nil {
		return nil, err
	}

	return deployment, nil
}

func (c apiConfig) findDeploymentWithID(ctx context.Context, ID string) (*Deployment, error) {
	var deployment Deployment

	deploymentCollection := c.DB.Collection("deployments")

	err := deploymentCollection.FindOne(ctx, bson.M{"id": ID}).Decode(&deployment)

	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, err
	}

	return &deployment, nil
}
