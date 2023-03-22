# SES Subscription Verifier

Mailing list system providing address validation and unsubscribe URIs.

Implemented in [Go][] using the following [Amazon Web Services][]:
- [API Gateway][]
- [Lambda][]
- [DynamoDB][]
- [Simple Email Service][]

Uses [CloudFormation][] for deploying the Lambda function, binding to the API
Gateway, managing permissions, and other configuration parameters.

Based on implementation hints from [victoriadrake/simple-subscribe][], but
otherwise contains original code.

**NOTE:** This project is just beginning. When it's operational, I'll remove
this notice.

## Algorithms

All responses will be [HTTP 303 See Other][], with the target page specified in
the [Location HTTP header][].

### Generating a new subscriber verification link

1. An HTTP request from the API Gateway comes in, containing the email address
   of a potential subscriber.
1. Validate the email address.
   1. Parse the name as closely as possible to [RFC 5322 Section 3.2.3][].
   1. Reject any common aliases, like "no-reply" or "postmaster."
   1. Check the MX record of the host, or find an A or AAAA record that accepts
      connections on port 587 or 2525.
   1. If it fails validation, return the `INVALID` page.
1. Check whether the email address is already in the SES contact list.
   1. If so, return the `ALREADY_SUBSCRIBED` page.
1. Generate a new verification UUID.
1. Look for an existing DynamoDB record for the email address and its
   verification UUID. If it does not exist:
   1. If it doesn't exist, create a new DynamoDB record containing the email
      address, the UUID, and a timestamp.
   1. If it does exist, replace the old UUID with the new one.
1. Generate a verification link using the UUID.
1. Email this link to the given address.
   1. If the mail bounces or fails to send, return the `INVALID` page.
1. Return the `CONFIRM` page.

### Responding to a subscriber verification link

1. An HTTP request from the API Gateway comes in, containing POST data from a
   user's unique verification link.
1. Check whether there is a verification link record for the encoded email
   address in DynamoDB.
   1. If not, return the `NOT_FOUND` page.
1. Check whether the verification link UUID matches that from the DynamoDB
   record.
   1. If not, return the `NOT_FOUND` page.
1. Add the email address to the SES contact list.
1. Remove the DynamoDB record.
1. Send a `SUBSCRIBED` email containing the unsubscribe link (from SES).
1. Return the `SUBSCRIBED` page.

### Expiring unused subscriber verification links

1. Retrieve all existing DynamoDB records.
1. Delete any records exceeding the timeout (1h).

### Removing unsubscribed email addresses

1. Retrieve all contacts from the contact list.
1. Delete all contacts that have unsubscribed.

## Open Source License

This software is made available as [Open Source software][oss-def] under the
[Mozilla Public License 2.0][]. For the text of the license, see the
[LICENSE.txt](LICENSE.txt) file.

## References

- [Building Lambda functions with Go][]
- [Using AWS Lambda with other services][]
- [aws/aws-sdk-go][]
- [aws/aws-lambda-go][]
- [Blank AWS Lambda function in Go][]
- [Installing or updating the latest version of the AWS CLI][]
- [The Complete AWS Sam Workshop][]
- [AWS Serverless Application Model (AWS SAM) specification][]
- [AWS Serverless Application Model (SAM) Version 2016-10-31][]
- [AWS Serverless Application Model (AWS SAM) Documentation][]
- [AWS CloudFormation Parameters][]
- [AWS CloudFormation Template Reference][]
- [AWS::Serverless::HttpApi][]
- [Setting up custom domain names for REST APIs][]
- [AWS::Serverless::Connector][]
- [How can I set up a custom domain name for my API Gateway API?][]
- [Tutorial: Build a CRUD API with Lambda and DynamoDB][]
- [AWS SAM policy templates][]
- [Serverless Land: Lambda to SES][]
- [How to use AWS secret manager and SES with AWS SAM][]
- [AWS SDK for Go V2][]
- [AWS Lambda function handler in Go][]
- [aws-lambda-go APIGatewayV2 event structures][]
- [aws-lambda-go APIGatewayV2 event example][]
- [Working with AWS Lambda proxy integrations for HTTP APIs][]
- [Using templates to send personalized email with the Amazon SES API][]
- [AWS SES Sample incoming email event][]
- [AWS CloudFormation AWS::SES::ReceiptRuleSet][]
- [AWS CloudFormation AWS::SES::ReceiptRule][]
- [AWS SES Invoke Lambda function action][]
- [Using AWS Lambda with Amazon SES][]
- [aws/aws-lambda-go/events/README_SES.md][]
- [Regions and Amazon SES][]
- [DMARC GUIDE | DMARC: What is DMARC?][]
- [Packing multiple binaries in a Golang package][]

[Go]: https://go.dev/
[Amazon Web Services]: https://aws.amazon.com
[API Gateway]: https://aws.amazon.com/api-gateway/
[Lambda]: https://aws.amazon.com/lambda/
[DynamoDB]: https://aws.amazon.com/dynamodb/
[Simple Email Service]: https://aws.amazon.com/ses/
[CloudFormation]: https://aws.amazon.com/cloudformation/
[victoriadrake/simple-subscribe]: https://github.com/victoriadrake/simple-subscribe/
[HTTP 303 See Other]: https://developer.mozilla.org/en-US/docs/Web/HTTP/Status/303
[Location HTTP Header]: https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Location
[RFC 5322 Section 3.2.3]: https://datatracker.ietf.org/doc/html/rfc5322#section-3.2.3
[oss-def]:     https://opensource.org/osd-annotated
[Mozilla Public License 2.0]: https://www.mozilla.org/en-US/MPL/
[Building Lambda functions with Go]: https://docs.aws.amazon.com/lambda/latest/dg/lambda-golang.html
[Using AWS Lambda with other services]: https://docs.aws.amazon.com/lambda/latest/dg/lambda-services.html
[aws/aws-sdk-go]: https://github.com/aws/aws-sdk-go
[aws/aws-lambda-go]: https://github.com/aws/aws-lambda-go
[Blank AWS Lambda function in Go]: https://github.com/awsdocs/aws-lambda-developer-guide/tree/main/sample-apps/blank-go
[Installing or updating the latest version of the AWS CLI]: https://docs.aws.amazon.com/cli/latest/userguide/getting-started-install.html
[The Complete AWS Sam Workshop]: https://catalog.workshops.aws/complete-aws-sam/en-US
[AWS Serverless Application Model (AWS SAM) specification]: https://docs.aws.amazon.com/serverless-application-model/latest/developerguide/sam-specification.html
[AWS Serverless Application Model (SAM) Version 2016-10-31]: https://github.com/aws/serverless-application-model/blob/master/versions/2016-10-31.md
[AWS Serverless Application Model (AWS SAM) Documentation]: https://docs.aws.amazon.com/serverless-application-model/
[AWS CloudFormation Parameters]: https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/parameters-section-structure.html
[AWS CloudFormation Template Reference]: https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/template-reference.html
[AWS::Serverless::HttpApi]: https://docs.aws.amazon.com/serverless-application-model/latest/developerguide/sam-resource-httpapi.html
[Setting up custom domain names for REST APIs]: https://docs.aws.amazon.com/apigateway/latest/developerguide/how-to-custom-domains.html
[AWS::Serverless::Connector]: https://docs.aws.amazon.com/serverless-application-model/latest/developerguide/sam-resource-connector.html
[How can I set up a custom domain name for my API Gateway API?]: https://aws.amazon.com/premiumsupport/knowledge-center/custom-domain-name-amazon-api-gateway/
[Tutorial: Build a CRUD API with Lambda and DynamoDB]: https://docs.aws.amazon.com/apigateway/latest/developerguide/http-api-dynamo-db.html
[AWS SAM policy templates]: https://docs.aws.amazon.com/serverless-application-model/latest/developerguide/serverless-policy-templates.html
[Serverless Land: Lambda to SES]: https://serverlessland.com/patterns/lambda-ses
[How to use AWS secret manager and SES with AWS SAM]: https://medium.com/nerd-for-tech/how-to-use-aws-secret-manager-and-ses-with-aws-sam-a93bb359d45a
[AWS SDK for Go V2]: https://aws.github.io/aws-sdk-go-v2/
[AWS Lambda function handler in Go]: https://docs.aws.amazon.com/lambda/latest/dg/golang-handler.html
[aws-lambda-go APIGatewayV2 event structures]: https://github.com/aws/aws-lambda-go/blob/main/events/apigw.go
[aws-lambda-go APIGatewayV2 event example]: https://github.com/aws/aws-lambda-go/blob/main/events/README_ApiGatewayEvent.md
[Working with AWS Lambda proxy integrations for HTTP APIs]: https://docs.aws.amazon.com/apigateway/latest/developerguide/http-api-develop-integrations-lambda.html
[Using templates to send personalized email with the Amazon SES API]: https://docs.aws.amazon.com/ses/latest/dg/send-personalized-email-api.html
[AWS SES Sample incoming email event]: https://docs.aws.amazon.com/ses/latest/dg/receiving-email-action-lambda-event.html
[AWS CloudFormation AWS::SES::ReceiptRuleSet]: https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-ses-receiptruleset.html
[AWS CloudFormation AWS::SES::ReceiptRule]: https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-ses-receiptrule.html
[AWS SES Invoke Lambda function action]: https://docs.aws.amazon.com/ses/latest/dg/receiving-email-action-lambda.html
[Using AWS Lambda with Amazon SES]: https://docs.aws.amazon.com/lambda/latest/dg/services-ses.html
[aws/aws-lambda-go/events/README_SES.md]: https://github.com/aws/aws-lambda-go/blob/main/events/README_SES.md
[Regions and Amazon SES]: https://docs.aws.amazon.com/ses/latest/dg/regions.html#region-endpoints
[DMARC GUIDE | DMARC: What is DMARC?]: https://dmarcguide.globalcyberalliance.org/#/dmarc/
[Packing multiple binaries in a Golang package]: https://ieftimov.com/posts/golang-package-multiple-binaries/
