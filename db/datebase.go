package db

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Post ...
type Post struct {
	Title      string `bson:"title" json:"title"`
	Body       string `bson:"body" json:"body"`
	URL        string `bson:"url" json:"url"`
	Collection string `bson:"collection" json:"collection"` // Topic/Project/Theme grouping
	IsIndex    bool   `bson:"isindex" json:"isIndex"`       // Mark if this is an index file
}

// ConnectToDB ...
func ConnectToDB() (*mongo.Collection, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	// mongodb+srv://beld:124252@cluster0-wmuco.mongodb.net/blog
	client, err := mongo.Connect(ctx, options.Client().ApplyURI("mongodb://mongo:27017/go-markdown-server"))
	if err != nil {
		return nil, err
	}
	collection := client.Database("blog").Collection("posts")
	return collection, nil
}

// GetPosts ...
func GetPosts(collection *mongo.Collection) ([]Post, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	opts := options.Find()
	opts.SetSort(bson.D{primitive.E{Key: "_id", Value: -1}})
	cursor, err := collection.Find(ctx, bson.D{}, opts)
	if err != nil {
		return []Post{}, err
	}
	ctx5, cancel5 := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel5()
	var posts []Post
	for cursor.Next(ctx5) {
		var post Post
		err := cursor.Decode(&post)
		if err != nil {
			return []Post{}, err
		}
		posts = append(posts, post)
	}
	return posts, nil
}

// GetPostByName ...
func GetPostByName(collection *mongo.Collection, name string) (Post, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	filter := bson.M{"url": name}
	var post Post
	if err := collection.FindOne(ctx, filter).Decode(&post); err != nil {
		return Post{}, err
	}
	return post, nil
}

// GetIndexPost returns the index post for a collection (if exists)
func GetIndexPost(collection *mongo.Collection, collectionName string) (Post, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	filter := bson.M{"isindex": true}
	if collectionName != "" {
		filter["collection"] = collectionName
	}
	
	var post Post
	if err := collection.FindOne(ctx, filter).Decode(&post); err != nil {
		fmt.Println("DEBUG GetIndexPost: Error decoding:", err)
		return Post{}, err
	}
	fmt.Println("DEBUG GetIndexPost: Found post:", post.Title, "IsIndex=", post.IsIndex, "URL=", post.URL)
	return post, nil
}

// InsertPost ...
func InsertPost(collection *mongo.Collection, post Post, key string) (*mongo.InsertOneResult, error) {
	if key == "124252" {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		obj, err := collection.InsertOne(ctx, post)
		if err != nil {
			return nil, err
		}
		return obj, err
	}
	return nil, errors.New("Key is not valid")
}

// UpsertPost updates existing post or inserts new one based on collection+url
func UpsertPost(collection *mongo.Collection, post Post, key string) error {
	if key != "124252" {
		return errors.New("Key is not valid")
	}
	
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	filter := bson.M{
		"collection": post.Collection,
		"url":        post.URL,
	}
	
	update := bson.M{
		"$set": bson.M{
			"title":      post.Title,
			"body":       post.Body,
			"collection": post.Collection,
			"isindex":    post.IsIndex,
		},
	}
	
	opts := options.Update().SetUpsert(true)
	_, err := collection.UpdateOne(ctx, filter, update, opts)
	return err
}

// GetCollections returns list of unique collection names
func GetCollections(collection *mongo.Collection) ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	collections, err := collection.Distinct(ctx, "collection", bson.D{})
	if err != nil {
		return []string{}, err
	}
	
	result := make([]string, 0, len(collections))
	for _, c := range collections {
		if str, ok := c.(string); ok && str != "" {
			result = append(result, str)
		}
	}
	return result, nil
}

// GetPostsByCollection returns all posts in a collection
func GetPostsByCollection(collection *mongo.Collection, collectionName string) ([]Post, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	filter := bson.M{"collection": collectionName}
	opts := options.Find()
	opts.SetSort(bson.D{primitive.E{Key: "title", Value: 1}})
	
	cursor, err := collection.Find(ctx, filter, opts)
	if err != nil {
		return []Post{}, err
	}
	
	var posts []Post
	ctx2, cancel2 := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel2()
	for cursor.Next(ctx2) {
		var post Post
		err := cursor.Decode(&post)
		if err != nil {
			return []Post{}, err
		}
		posts = append(posts, post)
	}
	return posts, nil
}

// DeleteCollection removes all posts from a collection
func DeleteCollection(collection *mongo.Collection, collectionName string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	filter := bson.M{"collection": collectionName}
	_, err := collection.DeleteMany(ctx, filter)
	return err
}

// DeletePostByPath deletes a post from database based on collection name and URL derived from file path
func DeletePostByPath(collection *mongo.Collection, collectionName string, url string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	filter := bson.M{
		"collection": collectionName,
		"url":        url,
	}

	result, err := collection.DeleteOne(ctx, filter)
	if err != nil {
		return err
	}

	if result.DeletedCount == 0 {
		return fmt.Errorf("no post found with collection=%s, url=%s", collectionName, url)
	}

	return nil
}

// RenameCollection updates the collection name for all posts
func RenameCollection(collection *mongo.Collection, oldName, newName string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	filter := bson.M{"collection": oldName}
	update := bson.M{"$set": bson.M{"collection": newName}}
	_, err := collection.UpdateMany(ctx, filter, update)
	return err
}
