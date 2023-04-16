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
	dbBool       = types.AttributeValueMemberBOOL
	dbAttributes = map[string]types.AttributeValue
)

var timeFmt = time.RFC3339

func subscriberKey(email string) dbAttributes {
	return dbAttributes{"email": &dbString{Value: email}}
}

type dbParser struct {
	attrs dbAttributes
}

func (p *dbParser) ParseSubscriber() (subscriber *Subscriber, err error) {
	s := &Subscriber{}
	errs := make([]error, 4)

	if s.Email, err = p.GetString("email"); err != nil {
		errs = append(errs, err)
	}
	if s.Uid, err = p.GetUid("uid"); err != nil {
		errs = append(errs, err)
	}
	if s.Verified, err = p.GetBool("verified"); err != nil {
		errs = append(errs, err)
	}
	if s.Timestamp, err = p.GetTime("timestamp"); err != nil {
		errs = append(errs, err)
	}
	if err = errors.Join(errs...); err == nil {
		subscriber = s
	}
	return s, nil
}

func (p *dbParser) GetString(name string) (value string, err error) {
	var attr *dbString

	if attr, err = getAttribute[*dbString](name, p.attrs); err == nil {
		value = attr.Value
	}
	return
}

func (p *dbParser) GetBool(name string) (value bool, err error) {
	var attr *dbBool

	if attr, err = getAttribute[*dbBool](name, p.attrs); err == nil {
		value = attr.Value
	}
	return
}

func (p *dbParser) GetUid(name string) (value uuid.UUID, err error) {
	return parseValue(name, p, uuid.Parse)
}

func parseTime(timeStr string) (time.Time, error) {
	return time.Parse(timeFmt, timeStr)
}

func (p *dbParser) GetTime(name string) (value time.Time, err error) {
	return parseValue(name, p, parseTime)
}

func getAttribute[T *dbString | *dbBool](
	name string, attrs dbAttributes,
) (result T, err error) {
	if attr, ok := attrs[name]; !ok {
		err = fmt.Errorf("attribute '%s' not in: %s", name, attrs)
	} else if result, ok = attr.(T); !ok {
		// Inspired by: https://stackoverflow.com/a/72626548
		const errFmt = "attribute '%s' is of type %T, not %T: %+v"
		err = fmt.Errorf(errFmt, name, attr, new(T), attr)
	}
	return
}

func parseValue[T uuid.UUID | time.Time](
	name string, parser *dbParser, parseValue func(string) (T, error),
) (value T, err error) {
	var attr string

	if attr, err = parser.GetString(name); err != nil {
		return
	} else if value, err = parseValue(attr); err != nil {
		const errFmt = "failed to parse '%s' from: %+v: %s"
		err = fmt.Errorf(errFmt, name, attr, err)
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

	parser := &dbParser{output.Item}
	return parser.ParseSubscriber()
}

func (db *DynamoDb) Put(record *Subscriber) (err error) {
	input := &dynamodb.PutItemInput{
		Item: map[string]types.AttributeValue{
			"email":    &dbString{Value: record.Email},
			"uid":      &dbString{Value: record.Uid.String()},
			"verified": &dbBool{Value: record.Verified},
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
