package releaseswatcher

import (
	"context"

	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var log logrus.Logger

const databaseName = "music"

type DB struct {
	client *mongo.Client
	coll   *mongo.Collection
}

type Album struct {
	Artist string `bson:"artist"`
	Album  string `bson:"album"`
}

func (db *DB) Disconnect(ctx context.Context) error {
	return db.client.Disconnect(ctx)
}

func (db *DB) Insert(ctx context.Context, album Album) error {
	filter := bson.D{
		{Key: "artist", Value: album.Artist},
		{Key: "album", Value: album.Album},
	}
	update := bson.D{{
		Key: "$set", Value: album,
	}}
	_, err := db.coll.UpdateOne(ctx, filter, update, options.Update().SetUpsert(true))
	return err
}

func NewDB(connection string) (DB, error) {
	var db DB
	serverAPI := options.ServerAPI(options.ServerAPIVersion1)
	opts := options.Client().ApplyURI(connection).SetServerAPIOptions(serverAPI)

	// Create a new client and connect to the server
	client, err := mongo.Connect(context.TODO(), opts)
	if err != nil {
		panic(err)
	}

	if err := client.Database(databaseName).RunCommand(context.TODO(), bson.D{{Key: "ping", Value: 1}}).Err(); err != nil {
		return DB{}, err
	}
	log.Info("Pinged your deployment. You successfully connected to MongoDB!")

	db.client = client
	db.coll = client.Database(databaseName).Collection("albums")
	return db, nil
}
