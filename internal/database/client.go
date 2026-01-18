package database

import (
	"context"
	"os"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
)

var (
	client    *dynamodb.Client
	tableName string
)

// Init initializes the DynamoDB client
func Init(ctx context.Context) error {
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return err
	}

	client = dynamodb.NewFromConfig(cfg)
	tableName = os.Getenv("DYNAMODB_TABLE")
	if tableName == "" {
		tableName = "dynamic-dns-table"
	}

	return nil
}

// GetClient returns the DynamoDB client
func GetClient() *dynamodb.Client {
	return client
}

// GetTableName returns the table name
func GetTableName() string {
	return tableName
}
