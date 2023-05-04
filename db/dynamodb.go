package db

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/google/uuid"
)

// https://docs.aws.amazon.com/amazondynamodb/latest/developerguide/WorkingWithItems.html
type DynamoDb struct {
	Client    *dynamodb.Client
	TableName string
}

var DynamoDbPrimaryKey string = "email"

var DynamoDbCreateTableInput = &dynamodb.CreateTableInput{
	AttributeDefinitions: []types.AttributeDefinition{
		{
			AttributeName: &DynamoDbPrimaryKey,
			AttributeType: types.ScalarAttributeTypeS,
		},
	},
	KeySchema: []types.KeySchemaElement{
		{AttributeName: &DynamoDbPrimaryKey, KeyType: types.KeyTypeHash},
	},
	BillingMode: types.BillingModePayPerRequest,
}

func (db *DynamoDb) CreateTable(ctx context.Context) (err error) {
	var input dynamodb.CreateTableInput = *DynamoDbCreateTableInput
	input.TableName = &db.TableName

	if _, err = db.Client.CreateTable(ctx, &input); err != nil {
		err = fmt.Errorf("failed to create db table %s: %s", db.TableName, err)
	}
	return
}

func (db *DynamoDb) WaitForTable(
	ctx context.Context, maxAttempts int, sleep func(),
) error {
	if maxAttempts <= 0 {
		const errFmt = "maxAttempts to wait for DB table must be >= 0, got: %d"
		return fmt.Errorf(errFmt, maxAttempts)
	}

	for current := 0; ; {
		td, err := db.DescribeTable(ctx)

		if err == nil && td.TableStatus == types.TableStatusActive {
			return nil
		} else if current++; current == maxAttempts {
			const errFmt = "db table %s not active after " +
				"%d attempts to check; last error: %s"
			return fmt.Errorf(errFmt, db.TableName, maxAttempts, err)
		}
		sleep()
	}
}

func (db *DynamoDb) DescribeTable(
	ctx context.Context,
) (td *types.TableDescription, err error) {
	input := &dynamodb.DescribeTableInput{TableName: &db.TableName}
	output, descErr := db.Client.DescribeTable(ctx, input)

	if descErr != nil {
		const errFmt = "failed to describe db table %s: %s"
		err = fmt.Errorf(errFmt, db.TableName, descErr)
	} else {
		td = output.Table
	}
	return
}

func (db *DynamoDb) DeleteTable(ctx context.Context) error {
	input := &dynamodb.DeleteTableInput{TableName: &db.TableName}
	if _, err := db.Client.DeleteTable(ctx, input); err != nil {
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

type dbParser struct {
	attrs dbAttributes
}

func ParseSubscriber(attrs dbAttributes) (subscriber *Subscriber, err error) {
	p := dbParser{attrs}
	s := &Subscriber{}
	errs := make([]error, 0, 4)
	addErr := func(e error) {
		errs = append(errs, e)
	}

	if s.Email, err = p.GetString("email"); err != nil {
		addErr(err)
	}
	if s.Uid, err = p.GetUid("uid"); err != nil {
		addErr(err)
	}
	if s.Timestamp, err = p.GetTime("timestamp"); err != nil {
		addErr(err)
	}

	_, pending := attrs[string(SubscriberStatePending)]
	_, verified := attrs[string(SubscriberStateVerified)]

	if pending && verified {
		const errFmt = "contains both '%s' and '%s' attributes"
		addErr(
			fmt.Errorf(errFmt, SubscriberStatePending, SubscriberStateVerified),
		)
	} else if !(pending || verified) {
		const errFmt = "has neither '%s' or '%s' attributes"
		addErr(
			fmt.Errorf(errFmt, SubscriberStatePending, SubscriberStateVerified),
		)
	} else {
		s.Verified = verified
	}

	if err = errors.Join(errs...); err != nil {
		err = errors.New("failed to parse subscriber: " + err.Error())
	} else {
		subscriber = s
	}
	return
}

func (p *dbParser) GetString(name string) (value string, err error) {
	return getAttribute(name, p.attrs, func(attr *dbString) (string, error) {
		return attr.Value, nil
	})
}

func (p *dbParser) GetUid(name string) (value uuid.UUID, err error) {
	return getAttribute(name, p.attrs, func(attr *dbString) (uuid.UUID, error) {
		return uuid.Parse(attr.Value)
	})
}

func (p *dbParser) GetTime(name string) (value time.Time, err error) {
	return getAttribute(name, p.attrs, func(attr *dbString) (time.Time, error) {
		return time.Parse(timeFmt, attr.Value)
	})
}

func getAttribute[T any, V any](
	name string, attrs dbAttributes, parse func(T) (V, error),
) (value V, err error) {
	if attr, ok := attrs[name]; !ok {
		err = fmt.Errorf("attribute '%s' not in: %+v", name, attrs)
	} else if dbAttr, ok := attr.(T); !ok {
		// Inspired by: https://stackoverflow.com/a/72626548
		const errFmt = "attribute '%s' is of type %T, not %T: %+v"
		err = fmt.Errorf(errFmt, name, attr, new(T), attr)
	} else if value, err = parse(dbAttr); err != nil {
		value = *new(V)
		const errFmt = "failed to parse '%s' from: %+v: %s"
		err = fmt.Errorf(errFmt, name, dbAttr, err)
	}
	return
}

func (db *DynamoDb) Get(
	ctx context.Context, email string,
) (subscriber *Subscriber, err error) {
	input := &dynamodb.GetItemInput{
		Key: subscriberKey(email), TableName: &db.TableName,
	}
	var output *dynamodb.GetItemOutput

	if output, err = db.Client.GetItem(ctx, input); err != nil {
		err = fmt.Errorf("failed to get %s: %s", email, err)
	} else if len(output.Item) == 0 {
		err = fmt.Errorf("%s is not a subscriber", email)
	} else {
		subscriber, err = ParseSubscriber(output.Item)
	}
	return
}

func (db *DynamoDb) Put(ctx context.Context, record *Subscriber) (err error) {
	stateKey := string(SubscriberStatePending)

	if record.Verified {
		stateKey = string(SubscriberStateVerified)
	}
	input := &dynamodb.PutItemInput{
		Item: map[string]types.AttributeValue{
			"email":  &dbString{Value: record.Email},
			"uid":    &dbString{Value: record.Uid.String()},
			stateKey: &dbString{Value: "Y"},
			"timestamp": &dbString{
				Value: record.Timestamp.Format(timeFmt),
			},
		},
		TableName: &db.TableName,
	}
	if _, err = db.Client.PutItem(ctx, input); err != nil {
		err = fmt.Errorf("failed to put %s: %s", record.Email, err)
	}
	return
}

func (db *DynamoDb) Delete(ctx context.Context, email string) (err error) {
	input := &dynamodb.DeleteItemInput{
		Key: subscriberKey(email), TableName: &db.TableName,
	}
	if _, err = db.Client.DeleteItem(ctx, input); err != nil {
		err = fmt.Errorf("failed to delete %s: %s", email, err)
	}
	return
}
