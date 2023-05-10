package db

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/google/uuid"
	"github.com/mbland/elistman/ops"
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

const DynamoDbPrimaryKey = "email"

// Sparse Global Secondary Index for records containing a "pending" attribute.
const DynamoDbPendingIndexName = string(SubscriberPending)
const DynamoDbPendingIndexPartitionKey = string(SubscriberPending)

// Sparse Global Secondary Index for records containing a "verified" attribute.
const DynamoDbVerifiedIndexName string = string(SubscriberVerified)
const DynamoDbVerifiedIndexPartitionKey string = string(SubscriberVerified)

var DynamoDbIndexProjection *types.Projection = &types.Projection{
	ProjectionType: types.ProjectionTypeAll,
}

var DynamoDbCreateTableInput = &dynamodb.CreateTableInput{
	AttributeDefinitions: []types.AttributeDefinition{
		{
			AttributeName: aws.String(DynamoDbPrimaryKey),
			AttributeType: types.ScalarAttributeTypeS,
		},
		{
			AttributeName: aws.String(DynamoDbPendingIndexPartitionKey),
			AttributeType: types.ScalarAttributeTypeN,
		},
		{
			AttributeName: aws.String(DynamoDbVerifiedIndexPartitionKey),
			AttributeType: types.ScalarAttributeTypeN,
		},
	},
	KeySchema: []types.KeySchemaElement{
		{
			AttributeName: aws.String(DynamoDbPrimaryKey),
			KeyType:       types.KeyTypeHash,
		},
	},
	BillingMode: types.BillingModePayPerRequest,
	GlobalSecondaryIndexes: []types.GlobalSecondaryIndex{
		{
			IndexName: aws.String(DynamoDbPendingIndexName),
			KeySchema: []types.KeySchemaElement{
				{
					AttributeName: aws.String(DynamoDbPendingIndexPartitionKey),
					KeyType:       types.KeyTypeHash,
				},
			},
			Projection: DynamoDbIndexProjection,
		},
		{
			IndexName: aws.String(DynamoDbVerifiedIndexName),
			KeySchema: []types.KeySchemaElement{
				{
					AttributeName: aws.String(
						DynamoDbVerifiedIndexPartitionKey,
					),
					KeyType: types.KeyTypeHash,
				},
			},
			Projection: DynamoDbIndexProjection,
		},
	},
}

func (db *DynamoDb) CreateTable(ctx context.Context) (err error) {
	var input dynamodb.CreateTableInput = *DynamoDbCreateTableInput
	input.TableName = aws.String(db.TableName)

	if _, err = db.Client.CreateTable(ctx, &input); err != nil {
		err = ops.AwsError(
			fmt.Errorf("failed to create db table %s: %w", db.TableName, err),
		)
	}
	return
}

func (db *DynamoDb) UpdateTimeToLive(
	ctx context.Context,
) (ttlSpec *types.TimeToLiveSpecification, err error) {
	spec := &types.TimeToLiveSpecification{
		AttributeName: aws.String(string(SubscriberPending)),
		Enabled:       aws.Bool(true),
	}
	input := &dynamodb.UpdateTimeToLiveInput{
		TableName: aws.String(db.TableName), TimeToLiveSpecification: spec,
	}

	var output *dynamodb.UpdateTimeToLiveOutput
	if output, err = db.Client.UpdateTimeToLive(ctx, input); err != nil {
		err = ops.AwsError(fmt.Errorf("failed to update Time To Live: %w", err))
	} else {
		ttlSpec = output.TimeToLiveSpecification
	}
	return
}

func (db *DynamoDb) DeleteTable(ctx context.Context) (err error) {
	input := &dynamodb.DeleteTableInput{TableName: aws.String(db.TableName)}
	if _, err = db.Client.DeleteTable(ctx, input); err != nil {
		err = ops.AwsError(
			fmt.Errorf("failed to delete db table %s: %w", db.TableName, err),
		)
	}
	return
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
		Key: subscriberKey(email), TableName: aws.String(db.TableName),
	}
	var output *dynamodb.GetItemOutput

	if output, err = db.Client.GetItem(ctx, input); err != nil {
		err = ops.AwsError(fmt.Errorf("failed to get %s: %w", email, err))
	} else if len(output.Item) == 0 {
		err = ErrSubscriberNotFound
	} else {
		subscriber, err = parseSubscriber(output.Item)
	}
	return
}

func (db *DynamoDb) Put(ctx context.Context, sub *Subscriber) (err error) {
	input := &dynamodb.PutItemInput{
		Item: dbAttributes{
			"email":            &dbString{Value: sub.Email},
			"uid":              &dbString{Value: sub.Uid.String()},
			string(sub.Status): toDynamoDbTimestamp(sub.Timestamp),
		},
		TableName: aws.String(db.TableName),
	}
	if _, err = db.Client.PutItem(ctx, input); err != nil {
		err = ops.AwsError(fmt.Errorf("failed to put %s: %w", sub.Email, err))
	}
	return
}

func (db *DynamoDb) Delete(ctx context.Context, email string) (err error) {
	input := &dynamodb.DeleteItemInput{
		Key: subscriberKey(email), TableName: aws.String(db.TableName),
	}
	if _, err = db.Client.DeleteItem(ctx, input); err != nil {
		err = ops.AwsError(fmt.Errorf("failed to delete %s: %w", email, err))
	}
	return
}

func (db *DynamoDb) ProcessSubscribersInState(
	ctx context.Context, status SubscriberStatus, sp SubscriberProcessor,
) error {
	input := &dynamodb.ScanInput{
		TableName: aws.String(db.TableName),
		IndexName: aws.String(string(status)),
	}
	paginator := dynamodb.NewScanPaginator(db.Client, input)

	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)

		if err != nil {
			return ops.AwsError(
				fmt.Errorf("failed to get %s subscribers: %w", status, err),
			)
		}

		for _, item := range output.Items {
			if s, err := parseSubscriber(item); err != nil || !sp.Process(s) {
				return err
			}
		}
	}
	return nil
}
