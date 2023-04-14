package db

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/google/uuid"
)

//go:generate go run golang.org/x/tools/cmd/stringer -type=SubscriberStatus
type SubscriberStatus int

const (
	Unverified SubscriberStatus = iota
	Verified
)

type Subscriber struct {
	Email     string
	UUID      string
	Status    SubscriberStatus
	Timestamp time.Time
}

type Database interface {
	Get(email string) (*Subscriber, error)
	Put(subscriber *Subscriber) error
	Delete(email string) error
}

func NewSubscriber(email string) *Subscriber {
	return &Subscriber{
		Email:     email,
		UUID:      uuid.NewString(),
		Timestamp: time.Now(),
	}
}

var DynamoDBPrimaryKey string = "email"

var DynamoDbCreateTableInput = &dynamodb.CreateTableInput{
	AttributeDefinitions: []types.AttributeDefinition{
		{
			AttributeName: &DynamoDBPrimaryKey,
			AttributeType: types.ScalarAttributeTypeS,
		},
	},
	KeySchema: []types.KeySchemaElement{
		{AttributeName: &DynamoDBPrimaryKey, KeyType: types.KeyTypeHash},
	},
	BillingMode: types.BillingModePayPerRequest,
}

type DynamoDb struct {
	Client    *dynamodb.Client
	TableName string
}

func NewDynamoDb(awsConfig *aws.Config, tableName string) *DynamoDb {
	return &DynamoDb{
		Client:    dynamodb.NewFromConfig(*awsConfig),
		TableName: tableName,
	}
}

func (db *DynamoDb) CreateTable() error {
	var input dynamodb.CreateTableInput = *DynamoDbCreateTableInput
	input.TableName = &db.TableName

	if _, err := db.Client.CreateTable(context.TODO(), &input); err != nil {
		return fmt.Errorf("failed to create db table %s: %s", db.TableName, err)
	}
	return nil
}

func (db *DynamoDb) DeleteTable() error {
	input := &dynamodb.DeleteTableInput{TableName: &db.TableName}
	if _, err := db.Client.DeleteTable(context.TODO(), input); err != nil {
		return fmt.Errorf("failed to delete db table %s: %s", db.TableName, err)
	}
	return nil
}

func (db *DynamoDb) Get(email string) (*Subscriber, error) {
	return nil, nil
}

func (db *DynamoDb) Put(record *Subscriber) error {
	return nil
}

func (db *DynamoDb) Delete(email string) error {
	return nil
}
