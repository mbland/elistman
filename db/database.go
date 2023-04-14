package db

import (
	"context"
	"errors"
	"fmt"
	"strings"
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

// This might be worth trying to contribute upstream to the stringer project one
// day.
func ParseSubscriberStatus(status string) (SubscriberStatus, error) {
	nameIndex := strings.Index(_SubscriberStatus_name, status)
	for i := range _SubscriberStatus_index {
		if i == nameIndex {
			return SubscriberStatus(i), nil
		}
	}
	return SubscriberStatus(nameIndex),
		fmt.Errorf("unknown SubscriberStatus: %s", status)
}

type Subscriber struct {
	Email     string
	Uid       uuid.UUID
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
		Uid:       uuid.New(),
		Status:    Unverified,
		Timestamp: time.Now().Truncate(time.Second),
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

// https://docs.aws.amazon.com/amazondynamodb/latest/developerguide/WorkingWithItems.html
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

type dbString = types.AttributeValueMemberS

var timeFmt = time.RFC3339

func subscriberKey(email string) map[string]types.AttributeValue {
	return map[string]types.AttributeValue{"email": &dbString{Value: email}}
}

func getAttribute(name string, output *dynamodb.GetItemOutput) (string, error) {
	if attrib, ok := output.Item[name]; !ok {
		return "", fmt.Errorf("attribute '%s' not in: %s", name, output.Item)
	} else if value, ok := attrib.(*dbString); !ok {
		return "", fmt.Errorf("attribute '%s' not a string: %s", name, attrib)
	} else {
		return value.Value, nil
	}
}

func parseAttributeError(name, attrStr string, err error) error {
	const errFmt = "failed to parse '%s' from: %s: %s"
	return fmt.Errorf(errFmt, name, attrStr, err)
}

func (db *DynamoDb) Get(email string) (subscriber *Subscriber, err error) {
	input := &dynamodb.GetItemInput{
		Key: subscriberKey(email), TableName: &db.TableName,
	}
	var output *dynamodb.GetItemOutput
	var uid string
	var status string
	var tstamp string
	record := &Subscriber{}
	errs := make([]error, 4)

	if output, err = db.Client.GetItem(context.TODO(), input); err != nil {
		err = fmt.Errorf("failed to get %s: %s", email, err)
		return
	} else if len(output.Item) == 0 {
		err = fmt.Errorf("%s is not a subscriber", email)
		return
	}
	if record.Email, err = getAttribute("email", output); err != nil {
		errs = append(errs, err)
	}
	if uid, err = getAttribute("uid", output); err != nil {
		errs = append(errs, err)
	} else if record.Uid, err = uuid.Parse(uid); err != nil {
		errs = append(errs, parseAttributeError("uid", uid, err))
	}
	if status, err = getAttribute("status", output); err != nil {
		errs = append(errs, err)
	} else if record.Status, err = ParseSubscriberStatus(status); err != nil {
		errs = append(errs, parseAttributeError("status", status, err))
	}
	if tstamp, err = getAttribute("timestamp", output); err != nil {
		errs = append(errs, err)
	} else if record.Timestamp, err = time.Parse(timeFmt, tstamp); err != nil {
		errs = append(errs, parseAttributeError("timestamp", tstamp, err))
	}

	if err = errors.Join(errs...); err == nil {
		subscriber = record
	}
	return
}

func (db *DynamoDb) Put(record *Subscriber) (err error) {
	input := &dynamodb.PutItemInput{
		Item: map[string]types.AttributeValue{
			"email":  &dbString{Value: record.Email},
			"uid":    &dbString{Value: record.Uid.String()},
			"status": &dbString{Value: record.Status.String()},
			"timestamp": &dbString{
				Value: record.Timestamp.Format(timeFmt),
			},
		},
		TableName: &db.TableName,
	}
	if _, err = db.Client.PutItem(context.TODO(), input); err != nil {
		err = fmt.Errorf("failed to put %s: %s", record.Email, err)
	}
	return
}

func (db *DynamoDb) Delete(email string) (err error) {
	input := &dynamodb.DeleteItemInput{
		Key: subscriberKey(email), TableName: &db.TableName,
	}
	if _, err = db.Client.DeleteItem(context.TODO(), input); err != nil {
		err = fmt.Errorf("failed to delete %s: %s", email, err)
	}
	return
}
