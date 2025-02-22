package main

import (
	"context"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

func (c apiConfig) handleCreateProject(ctx context.Context, ProjectName string, GithubRepo string, FrameWork string) (primitive.ObjectID, error) {
	projectCollection := c.DB.Collection("projects")
	project := Project{
		GithubRepo:  GithubRepo,
		ProjectName: ProjectName,
		FrameWork:   FrameWork,
	}

	res, err := projectCollection.InsertOne(ctx, project)

	if err != nil {
		return primitive.NilObjectID, err
	}

	obj := res.InsertedID.(primitive.ObjectID)

	return obj, nil
}
