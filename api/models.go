package main

import "go.mongodb.org/mongo-driver/bson/primitive"

type User struct {
	ID       primitive.ObjectID   `bson:"_id""json:"ID"`
	Username string               `bson:"username"json:"username"`
	Password string               `bson:"password"json:"password"`
	Projects []primitive.ObjectID `bson:"projects" json:"projects"`
}

type Project struct {
	ID          primitive.ObjectID `json:"ID"`
	GithubRepo  string             `bson:"github_repo"json:"githubRepo"`
	ProjectName string             `bson:"project_name"json:"projectName"`
	FrameWork   string             `bson:"framework"json:"framework"`
}
