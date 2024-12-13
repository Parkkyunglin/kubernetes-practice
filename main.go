package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"

	"github.com/go-redis/redis/v8"
	"github.com/joho/godotenv"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	pb "kubeprac/gen/proto/v1"
)

// server is used to implement the gRPC service.
type server struct {
	pb.UnimplementedYourServiceServer
	redisClient *redis.Client
	db          *gorm.DB
}

// ExampleMethod implements one of the gRPC methods.
func (s *server) ExampleGetMethod(ctx context.Context, req *pb.ExampleGetMethodRequest) (*pb.ExampleGetMethodResponse, error) {
	idValue := req.GetId()

	// Example: Fetch data from Redis
	val, err := s.redisClient.Get(ctx, "example_key").Result()
	if err != nil {
		if err == redis.Nil {
			logrus.WithFields(logrus.Fields{
				"event": "get_redis_key_error",
				"key":   "example_key",
			}).Info("Redis error")
		} else {
			logrus.WithFields(logrus.Fields{
				"event": "redis_error",
				"key":   "example_key",
			}).Info("Redis error")
			return nil, err
		}
	}

	// Example: Query from PostgreSQL
	var result YourModel
	if err := s.db.First(&result, "id = ?", idValue).Error; err != nil {
		logrus.WithFields(logrus.Fields{
			"event": "postgresql_error",
			"id":    idValue,
		}).Info("PostgreSQL error")
		return nil, err
	}

	logrus.WithFields(logrus.Fields{
		"event":      "success",
		"id":         idValue,
		"some_field": result.SomeField,
	}).Info("Get record success")

	return &pb.ExampleGetMethodResponse{Message: fmt.Sprintf("Value: %s, DB: %s", val, result.SomeField)}, nil
}

func (s *server) ExamplePostMethod(ctx context.Context, req *pb.ExamplePostMethodRequest) (*pb.ExamplePostMethodResponse, error) {
	someFieldValue := req.GetSomeField()

	newRecord := YourModel{
		SomeField: someFieldValue,
	}

	if err := s.db.Create(&newRecord).Error; err != nil {
		logrus.WithFields(logrus.Fields{
			"event":      "postgresql_create_error",
			"some_field": someFieldValue,
		}).Info("PostgreSQL create error")
		return nil, err
	}

	logrus.WithFields(logrus.Fields{
		"event":      "success",
		"some_field": someFieldValue,
	}).Info("Create record success")

	return &pb.ExamplePostMethodResponse{Message: "Record saved successfully"}, nil
}

func main() {

	if err := godotenv.Load(); err != nil {
		log.Printf("Warning: .env file not found, using environment variables")
	}

	if err := os.MkdirAll("logs", 0755); err != nil {
		log.Fatalf("Failed to create log directory: %v", err)
	}

	// JSON 형식으로 로그 설정
	logrus.SetFormatter(&logrus.JSONFormatter{})
	file, err := os.OpenFile("logs/grpc-server.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("Failed to open log file: %v", err)
	}
	logrus.SetOutput(file)

	// 로그 레벨 설정
	logrus.SetLevel(logrus.InfoLevel)

	// Initialize Redis client
	redisHost := os.Getenv("REDIS_HOST")
	if redisHost == "" {
		redisHost = "localhost"
	}
	redisClient := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:6379", redisHost),
		Password: "",
		DB:       0,
	})

	// Ping Redis to check connection
	if _, err := redisClient.Ping(context.Background()).Result(); err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}

	// Initialize PostgreSQL connection
	postgresHost := os.Getenv("POSTGRES_HOST")
	postgresUser := os.Getenv("POSTGRES_USER")
	postgresPassword := os.Getenv("POSTGRES_PASSWORD")
	postgresDB := os.Getenv("POSTGRES_DB")

	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=5432 sslmode=disable",
		postgresHost, postgresUser, postgresPassword, postgresDB)
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to connect to PostgreSQL: %v", err)
	}

	// AutoMigrate example
	if err := db.AutoMigrate(&YourModel{}); err != nil {
		log.Fatalf("Failed to migrate PostgreSQL schema: %v", err)
	}

	// Start gRPC server
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}
	grpcServer := grpc.NewServer()
	pb.RegisterYourServiceServer(grpcServer, &server{redisClient: redisClient, db: db})

	log.Println("Server is running on port 50051")
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
}

// YourModel represents a table in your PostgreSQL database.
type YourModel struct {
	ID        uint   `gorm:"primaryKey"`
	SomeField string `gorm:"not null"`
}
