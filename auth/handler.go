package auth

import (
	"context"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// RegisterRequest is the expected payload for user registration
type RegisterRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

func getMongoURI() string {
	// prefer explicit ME_CONFIG_MONGODB_URL (used for mongo-express), then MONGO_URL, then fallback
	if v := os.Getenv("ME_CONFIG_MONGODB_URL"); v != "" {
		return v
	}
	if v := os.Getenv("MONGO_URL"); v != "" {
		return v
	}
	// default used by docker-compose in this project
	return "mongodb://admin:secretpassword@mongo:27017/nuvola?authSource=admin"
}

func getMongoClient(ctx context.Context) (*mongo.Client, error) {
	uri := getMongoURI()
	clientOpts := options.Client().ApplyURI(uri)
	return mongo.Connect(ctx, clientOpts)
}

func usersCollection(client *mongo.Client) *mongo.Collection {
	db := "nuvola"
	if v := os.Getenv("MONGO_DB"); v != "" {
		db = v
	}
	return client.Database(db).Collection("users")
}

// RegisterHandler handles POST /auth/register and inserts a user document into MongoDB.
func RegisterHandler(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// connect to mongo with a short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	client, err := getMongoClient(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to connect to database"})
		return
	}
	defer client.Disconnect(ctx)

	userID := uuid.New().String()

	coll := usersCollection(client)
	doc := bson.M{
		"_id":        userID,
		"username":   req.Username,
		"password":   req.Password, // intentionally plain for mock
		"created_at": time.Now(),
	}
	if _, err := coll.InsertOne(ctx, doc); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"status":  "ok",
		"user_id": userID,
	})
}

// LoginRequest is the expected payload for user login
type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// LoginHandler handles POST /auth/login and verifies credentials against MongoDB.
func LoginHandler(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	client, err := getMongoClient(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to connect to database"})
		return
	}
	defer client.Disconnect(ctx)

	coll := usersCollection(client)
	filter := bson.M{"username": req.Username, "password": req.Password}
	var result bson.M
	err = coll.FindOne(ctx, filter).Decode(&result)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
		return
	}

	// success
	var userIDStr string
	if idStr, ok := result["_id"].(string); ok {
		userIDStr = idStr
	}

	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		secret = "default-secret-key"
	}

	claims := jwt.MapClaims{
		"user_id": userIDStr,
		"exp":     time.Now().Add(24 * time.Hour).Unix(),
		"iat":     time.Now().Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(secret))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
		"token": tokenString,
	})
}
