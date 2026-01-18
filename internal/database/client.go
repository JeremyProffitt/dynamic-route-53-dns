package database

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
)

// Client wraps the DynamoDB client and table configuration.
type Client struct {
	db        *dynamodb.Client
	tableName string
}

// NewClient creates a new DynamoDB client using the default AWS configuration.
// The table name is used for all database operations.
func NewClient(ctx context.Context, tableName string) (*Client, error) {
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, err
	}

	client := dynamodb.NewFromConfig(cfg)

	return &Client{
		db:        client,
		tableName: tableName,
	}, nil
}

// DB returns the underlying DynamoDB client.
func (c *Client) DB() *dynamodb.Client {
	return c.db
}

// TableName returns the configured table name.
func (c *Client) TableName() string {
	return c.tableName
}
