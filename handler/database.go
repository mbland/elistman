package handler

import (
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/google/uuid"
)

type UserStatus int

const (
	UNVERIFIED UserStatus = iota
	VERIFIED
)

func (us UserStatus) String() string {
	switch {
	case us == UNVERIFIED:
		return "UNVERIFIED"
	case us == VERIFIED:
		return "VERIFIED"
	}
	return "UNKNOWN"
}

type UserRecord struct {
	Email     string
	UUID      string
	Status    UserStatus
	Timestamp time.Time
}

type Database interface {
	Get(email string) (*UserRecord, error)
	Put(record *UserRecord) error
	Delete(email string) error
}

func NewUserRecord(email string) *UserRecord {
	return &UserRecord{
		Email:     email,
		UUID:      uuid.NewString(),
		Timestamp: time.Now(),
	}
}

type DynamoDb struct {
	Client    *dynamodb.Client
	TableName string
}

func NewDynamoDb(awsConfig aws.Config, tableName string) *DynamoDb {
	return &DynamoDb{
		Client:    dynamodb.NewFromConfig(awsConfig),
		TableName: tableName,
	}
}

func (db DynamoDb) Get(email string) (*UserRecord, error) {
	return nil, nil
}

func (db DynamoDb) Put(record *UserRecord) error {
	return nil
}

func (db DynamoDb) Delete(email string) error {
	return nil
}
