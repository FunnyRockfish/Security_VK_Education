package database

import (
	"context"
	"log"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func ConnectToMongoDataBase() *mongo.Database {
	ctx := context.TODO()
	clientOptions := options.Client().ApplyURI("mongodb://FunnyRockfish:homework3@localhost:27017")
	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		log.Fatal("DataBase connect err:", err)
	}

	err = client.Ping(ctx, nil)
	if err != nil {
		log.Fatal("DataBase ping err:", err)
	}
	log.Println("Successful connected to MongoDB")

	database := client.Database("task3")
	return database
}
