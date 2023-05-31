//go:build small_tests || all_tests

package db

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/mbland/elistman/testutils"
)

// Most of the methods on TestDynamoDbClient are unimplemented, because
// dynamodb_contract_test.go tests most of them.
//
// The original exception to this was Scan(), which was the reason why the
// DynamoDbClient interface was created. Testing all the cases of the code that
// relies on Scan() is annoying, difficult, and/or nearly impossible without
// using this test double.
//
// CreateTable, DescribeTable, and UpdateTimeToLive are also implemented. The
// dynamodb_contract_test tests and validates these individual operations. Given
// that, CreateSubscribersTable can then be tested more quickly and reliably
// using this test double.
type TestDynamoDbClient struct {
	ServerErr         error
	CreateTableInput  *dynamodb.CreateTableInput
	CreateTableOutput *dynamodb.CreateTableOutput
	CreateTableErr    error
	DescTableInput    *dynamodb.DescribeTableInput
	DescTableOutput   *dynamodb.DescribeTableOutput
	DescTableErr      error
	UpdateTtlInput    *dynamodb.UpdateTimeToLiveInput
	UpdateTtlOutput   *dynamodb.UpdateTimeToLiveOutput
	UpdateTtlErr      error
	Subscribers       []dbAttributes
	ScanSize          int
	ScanCalls         int
	ScanErr           error
}

// NewTestDynamoDbClient returns an initialized TestDynamoDbClient.
//
// Specifically, all of its *Output members are initialized to default non-nil
// values.
func NewTestDynamoDbClient() *TestDynamoDbClient {
	tableDesc := &types.TableDescription{
		TableName:   aws.String(""),
		TableStatus: types.TableStatusActive,
	}

	return &TestDynamoDbClient{
		CreateTableOutput: &dynamodb.CreateTableOutput{
			TableDescription: tableDesc,
		},
		DescTableOutput: &dynamodb.DescribeTableOutput{Table: tableDesc},
		UpdateTtlOutput: &dynamodb.UpdateTimeToLiveOutput{
			TimeToLiveSpecification: &types.TimeToLiveSpecification{},
		},
		Subscribers: []dbAttributes{},
	}
}

func (client *TestDynamoDbClient) SetAllErrors(msg string) {
	err := testutils.AwsServerError(msg)
	client.ServerErr = err
	client.CreateTableErr = err
	client.DescTableErr = err
	client.UpdateTtlErr = err
	client.ScanErr = err
}

func (client *TestDynamoDbClient) SetCreateTableError(msg string) {
	client.CreateTableErr = testutils.AwsServerError(msg)
}

func (client *TestDynamoDbClient) SetDescribeTableError(msg string) {
	client.DescTableErr = testutils.AwsServerError(msg)
}

func (client *TestDynamoDbClient) SetUpdateTimeToLiveError(msg string) {
	client.UpdateTtlErr = testutils.AwsServerError(msg)
}

func (client *TestDynamoDbClient) SetScanError(msg string) {
	client.ScanErr = testutils.AwsServerError(msg)
}

func (client *TestDynamoDbClient) CreateTable(
	_ context.Context,
	input *dynamodb.CreateTableInput,
	_ ...func(*dynamodb.Options),
) (*dynamodb.CreateTableOutput, error) {
	client.CreateTableInput = input
	return client.CreateTableOutput, client.CreateTableErr
}

func (client *TestDynamoDbClient) DescribeTable(
	_ context.Context,
	input *dynamodb.DescribeTableInput,
	_ ...func(*dynamodb.Options),
) (*dynamodb.DescribeTableOutput, error) {
	client.DescTableInput = input
	return client.DescTableOutput, client.DescTableErr
}

func (client *TestDynamoDbClient) UpdateTimeToLive(
	_ context.Context,
	input *dynamodb.UpdateTimeToLiveInput,
	_ ...func(*dynamodb.Options),
) (*dynamodb.UpdateTimeToLiveOutput, error) {
	client.UpdateTtlInput = input
	return client.UpdateTtlOutput, client.UpdateTtlErr
}

func (client *TestDynamoDbClient) DeleteTable(
	context.Context, *dynamodb.DeleteTableInput, ...func(*dynamodb.Options),
) (*dynamodb.DeleteTableOutput, error) {
	return nil, client.ServerErr
}

func (client *TestDynamoDbClient) GetItem(
	context.Context, *dynamodb.GetItemInput, ...func(*dynamodb.Options),
) (*dynamodb.GetItemOutput, error) {
	return nil, client.ServerErr
}

func (client *TestDynamoDbClient) PutItem(
	context.Context, *dynamodb.PutItemInput, ...func(*dynamodb.Options),
) (*dynamodb.PutItemOutput, error) {
	return nil, client.ServerErr
}

func (client *TestDynamoDbClient) DeleteItem(
	context.Context, *dynamodb.DeleteItemInput, ...func(*dynamodb.Options),
) (*dynamodb.DeleteItemOutput, error) {
	return nil, client.ServerErr
}

func (client *TestDynamoDbClient) addSubscriberRecord(sub dbAttributes) {
	client.Subscribers = append(client.Subscribers, sub)
}

func (client *TestDynamoDbClient) addSubscribers(subs []*Subscriber) {
	for _, sub := range subs {
		subRec := newSubscriberRecord(sub)
		client.Subscribers = append(client.Subscribers, subRec)
	}
}

func (client *TestDynamoDbClient) Scan(
	_ context.Context, input *dynamodb.ScanInput, _ ...func(*dynamodb.Options),
) (output *dynamodb.ScanOutput, err error) {
	client.ScanCalls++

	err = client.ScanErr
	if err != nil {
		return
	}

	// Remember that our schema is to keep pending and verified subscribers
	// partitioned across disjoint Global Secondary Indexes. So we first filter
	// for subscribers in the desired state.
	subscribers := make([]dbAttributes, 0, len(client.Subscribers))
	for _, sub := range client.Subscribers {
		if _, ok := sub[aws.ToString(input.IndexName)]; !ok {
			continue
		}
		subscribers = append(subscribers, sub)
	}

	// Scan starting just past the start key until we reach the scan limit.
	items := make([]dbAttributes, 0, len(subscribers))
	getEmail := func(attrs dbAttributes) (email string) {
		email, _ = (&dbParser{attrs}).GetString("email")
		return
	}
	startKey := getEmail(input.ExclusiveStartKey)
	started := len(startKey) == 0
	atScanLimit := func() bool {
		return client.ScanSize != 0 && len(items) == client.ScanSize
	}
	var lastKey dbAttributes

	for i, sub := range subscribers {
		if !started {
			started = getEmail(sub) == startKey
			continue
		}
		items = append(items, sub)

		if atScanLimit() {
			if i != (len(subscribers) - 1) {
				lastKey = dbAttributes{"email": sub["email"]}
			}
			break
		}
	}
	output = &dynamodb.ScanOutput{Items: items, LastEvaluatedKey: lastKey}
	return
}

func newSubscriberRecord(sub *Subscriber) dbAttributes {
	return dbAttributes{
		"email":            &dbString{Value: sub.Email},
		"uid":              &dbString{Value: sub.Uid.String()},
		string(sub.Status): toDynamoDbTimestamp(sub.Timestamp),
	}
}
