package db

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/google/uuid"
)

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

type (
	dbString     = types.AttributeValueMemberS
	dbAttributes = map[string]types.AttributeValue
)

var timeFmt = time.RFC3339

func subscriberKey(email string) dbAttributes {
	return dbAttributes{"email": &dbString{Value: email}}
}

type subscriberElements interface {
	string | uuid.UUID | SubscriberStatus | time.Time
}

func getAttribute[T subscriberElements](
	name string, attrs dbAttributes, parse func(string) (T, error),
) (result T, err error) {
	if attr, ok := attrs[name]; !ok {
		err = fmt.Errorf("attribute '%s' not in: %s", name, attrs)
	} else if value, ok := attr.(*dbString); !ok {
		err = fmt.Errorf("attribute '%s' not a string: %s", name, attr)
	} else if result, err = parse(value.Value); err != nil {
		const errFmt = "failed to parse '%s' from: %s: %s"
		err = fmt.Errorf(errFmt, name, value.Value, err)
	}
	return
}

func (db *DynamoDb) Get(email string) (subscriber *Subscriber, err error) {
	input := &dynamodb.GetItemInput{
		Key: subscriberKey(email), TableName: &db.TableName,
	}
	var output *dynamodb.GetItemOutput

	if output, err = db.Client.GetItem(context.TODO(), input); err != nil {
		err = fmt.Errorf("failed to get %s: %s", email, err)
		return
	} else if len(output.Item) == 0 {
		err = fmt.Errorf("%s is not a subscriber", email)
		return
	}

	attrs := output.Item
	record := &Subscriber{}
	errs := make([]error, 4)
	parseStr := func(s string) (string, error) { return s, nil }
	parseStatus := ParseSubscriberStatus
	parseTime := func(s string) (time.Time, error) {
		return time.Parse(timeFmt, s)
	}

	if record.Email, err = getAttribute("email", attrs, parseStr); err != nil {
		errs = append(errs, err)
	}
	if record.Uid, err = getAttribute("uid", attrs, uuid.Parse); err != nil {
		errs = append(errs, err)
	}
	if record.Status, err = getAttribute("status", attrs, parseStatus); err != nil {
		errs = append(errs, err)
	}
	if record.Timestamp, err = getAttribute("timestamp", attrs, parseTime); err != nil {
		errs = append(errs, err)
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
