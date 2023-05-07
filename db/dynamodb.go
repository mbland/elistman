package db

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/google/uuid"
)

type DynamoDbClient interface {
	CreateTable(
		context.Context, *dynamodb.CreateTableInput, ...func(*dynamodb.Options),
	) (*dynamodb.CreateTableOutput, error)

	DescribeTable(
		context.Context,
		*dynamodb.DescribeTableInput,
		...func(*dynamodb.Options),
	) (*dynamodb.DescribeTableOutput, error)

	UpdateTimeToLive(
		context.Context,
		*dynamodb.UpdateTimeToLiveInput,
		...func(*dynamodb.Options),
	) (*dynamodb.UpdateTimeToLiveOutput, error)

	DeleteTable(
		context.Context, *dynamodb.DeleteTableInput, ...func(*dynamodb.Options),
	) (*dynamodb.DeleteTableOutput, error)

	GetItem(
		context.Context, *dynamodb.GetItemInput, ...func(*dynamodb.Options),
	) (*dynamodb.GetItemOutput, error)

	PutItem(
		context.Context, *dynamodb.PutItemInput, ...func(*dynamodb.Options),
	) (*dynamodb.PutItemOutput, error)

	DeleteItem(
		context.Context, *dynamodb.DeleteItemInput, ...func(*dynamodb.Options),
	) (*dynamodb.DeleteItemOutput, error)

	Scan(
		context.Context, *dynamodb.ScanInput, ...func(*dynamodb.Options),
	) (*dynamodb.ScanOutput, error)
}

// https://docs.aws.amazon.com/amazondynamodb/latest/developerguide/WorkingWithItems.html
type DynamoDb struct {
	Client    DynamoDbClient
	TableName string
}

var DynamoDbPrimaryKey string = "email"

// Sparse Global Secondary Index for records containing a "pending" attribute.
var DynamoDbPendingIndexName string = string(SubscriberPending)
var DynamoDbPendingIndexPartitionKey string = string(SubscriberPending)

// Sparse Global Secondary Index for records containing a "verified" attribute.
var DynamoDbVerifiedIndexName string = string(SubscriberVerified)
var DynamoDbVerifiedIndexPartitionKey string = string(SubscriberVerified)

var DynamoDbIndexProjection *types.Projection = &types.Projection{
	ProjectionType: types.ProjectionTypeAll,
}

var DynamoDbCreateTableInput = &dynamodb.CreateTableInput{
	AttributeDefinitions: []types.AttributeDefinition{
		{
			AttributeName: &DynamoDbPrimaryKey,
			AttributeType: types.ScalarAttributeTypeS,
		},
		{
			AttributeName: &DynamoDbPendingIndexPartitionKey,
			AttributeType: types.ScalarAttributeTypeN,
		},
		{
			AttributeName: &DynamoDbVerifiedIndexPartitionKey,
			AttributeType: types.ScalarAttributeTypeN,
		},
	},
	KeySchema: []types.KeySchemaElement{
		{AttributeName: &DynamoDbPrimaryKey, KeyType: types.KeyTypeHash},
	},
	BillingMode: types.BillingModePayPerRequest,
	GlobalSecondaryIndexes: []types.GlobalSecondaryIndex{
		{
			IndexName: &DynamoDbPendingIndexName,
			KeySchema: []types.KeySchemaElement{
				{
					AttributeName: &DynamoDbPendingIndexPartitionKey,
					KeyType:       types.KeyTypeHash,
				},
			},
			Projection: DynamoDbIndexProjection,
		},
		{
			IndexName: &DynamoDbVerifiedIndexName,
			KeySchema: []types.KeySchemaElement{
				{
					AttributeName: &DynamoDbVerifiedIndexPartitionKey,
					KeyType:       types.KeyTypeHash,
				},
			},
			Projection: DynamoDbIndexProjection,
		},
	},
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

func (db *DynamoDb) UpdateTimeToLive(
	ctx context.Context,
) (ttlSpec *types.TimeToLiveSpecification, err error) {
	pendingAttr := string(SubscriberPending)
	enabled := true
	spec := &types.TimeToLiveSpecification{
		AttributeName: &pendingAttr, Enabled: &enabled,
	}
	input := &dynamodb.UpdateTimeToLiveInput{
		TableName: &db.TableName, TimeToLiveSpecification: spec,
	}

	var output *dynamodb.UpdateTimeToLiveOutput
	if output, err = db.Client.UpdateTimeToLive(ctx, input); err != nil {
		err = fmt.Errorf("failed to update Time To Live: %s", err)
	} else {
		ttlSpec = output.TimeToLiveSpecification
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
	dbNumber     = types.AttributeValueMemberN
	dbAttributes = map[string]types.AttributeValue
)

func subscriberKey(email string) dbAttributes {
	return dbAttributes{"email": &dbString{Value: email}}
}

type dbParser struct {
	attrs dbAttributes
}

func parseSubscriber(attrs dbAttributes) (subscriber *Subscriber, err error) {
	p := dbParser{attrs}
	s := &Subscriber{}
	errs := make([]error, 0, 3)
	addErr := func(e error) {
		errs = append(errs, e)
	}

	if s.Email, err = p.GetString("email"); err != nil {
		addErr(err)
	}
	if s.Uid, err = p.GetUid("uid"); err != nil {
		addErr(err)
	}

	_, pending := attrs[string(SubscriberPending)]
	_, verified := attrs[string(SubscriberVerified)]

	s.Status = SubscriberPending
	if verified {
		s.Status = SubscriberVerified
	}

	if pending && verified {
		const errFmt = "contains both '%s' and '%s' attributes"
		addErr(fmt.Errorf(errFmt, SubscriberPending, SubscriberVerified))
	} else if !(pending || verified) {
		const errFmt = "has neither '%s' or '%s' attributes"
		addErr(fmt.Errorf(errFmt, SubscriberPending, SubscriberVerified))
	} else if s.Timestamp, err = p.GetTime(string(s.Status)); err != nil {
		addErr(err)
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

func toDynamoDbTimestamp(t time.Time) *dbNumber {
	return &dbNumber{Value: strconv.FormatInt(t.Unix(), 10)}
}

func (p *dbParser) GetTime(name string) (value time.Time, err error) {
	return getAttribute(name, p.attrs, func(attr *dbNumber) (time.Time, error) {
		if ts, err := strconv.ParseInt(attr.Value, 10, 0); err != nil {
			return time.Time{}, err
		} else {
			return time.Unix(ts, 0), nil
		}
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
		subscriber, err = parseSubscriber(output.Item)
	}
	return
}

func (db *DynamoDb) Put(ctx context.Context, record *Subscriber) (err error) {
	input := &dynamodb.PutItemInput{
		Item: dbAttributes{
			"email":               &dbString{Value: record.Email},
			"uid":                 &dbString{Value: record.Uid.String()},
			string(record.Status): toDynamoDbTimestamp(record.Timestamp),
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

func (db *DynamoDb) ProcessSubscribersInState(
	ctx context.Context, status SubscriberStatus, processor SubscriberProcessor,
) (err error) {
	var subs []*Subscriber
	var next StartKey

	for {
		subs, next, err = db.GetSubscribersInState(ctx, status, next)

		if err != nil {
			return
		}
		for _, sub := range subs {
			if !processor.Process(sub) {
				return
			}
		}
		if next == nil {
			return
		}
	}
}

type dynamoDbStartKey struct {
	attrs dbAttributes
}

func (*dynamoDbStartKey) isDbStartKey() {}

func (db *DynamoDb) GetSubscribersInState(
	ctx context.Context, state SubscriberStatus, startKey StartKey,
) (subs []*Subscriber, nextStartKey StartKey, err error) {
	const errFmt = "failed to get %s subscribers: %s"
	var input *dynamodb.ScanInput
	var output *dynamodb.ScanOutput

	if input, err = newScanInput(db.TableName, state, startKey); err != nil {
		err = fmt.Errorf(errFmt, state, err)
	} else if output, err = db.Client.Scan(ctx, input); err != nil {
		err = fmt.Errorf(errFmt, state, err)
	} else {
		subs, nextStartKey, err = processScanOutput(output)
	}
	return
}

func newScanInput(
	tableName string, state SubscriberStatus, startKey StartKey,
) (input *dynamodb.ScanInput, err error) {
	var dbStartKey *dynamoDbStartKey
	var ok bool

	if startKey == nil {
		dbStartKey = &dynamoDbStartKey{}
	} else if dbStartKey, ok = startKey.(*dynamoDbStartKey); !ok {
		err = fmt.Errorf("not a *db.dynamoDbStartKey: %T", startKey)
		return
	}

	indexName := string(state)
	input = &dynamodb.ScanInput{
		TableName:         &tableName,
		IndexName:         &indexName,
		ExclusiveStartKey: dbStartKey.attrs,
	}
	return
}

func processScanOutput(
	output *dynamodb.ScanOutput,
) (subs []*Subscriber, nextStartKey StartKey, err error) {
	if len(output.LastEvaluatedKey) != 0 {
		nextStartKey = &dynamoDbStartKey{output.LastEvaluatedKey}
	}

	subs = make([]*Subscriber, len(output.Items))
	errs := make([]error, len(subs))

	for i, item := range output.Items {
		subs[i], errs[i] = parseSubscriber(item)
	}
	err = errors.Join(errs...)
	return
}
