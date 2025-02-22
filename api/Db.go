package main

import (
	"context"
	"log"
	"os"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func ConnectDB() *mongo.Client {

	mongoDbURI := os.Getenv("MONGODB_URI")

	log.Println(mongoDbURI)

	client, err := mongo.Connect(context.TODO(), options.Client().ApplyURI(mongoDbURI))

	if err != nil {
		panic(err)
	}

	return client

}
