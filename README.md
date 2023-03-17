# SES Subscription Verifier

A system for validating email addresses of potential mailing list subscribers
and adding them to an [Amazon Web Services][] [Simple Email Service][]
contact list.

Implemented in [Go][] using [API Gateway][], [Lambda][] and [DynamoDB][].
Deployed using [CloudFormation][].

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

[Amazon Web Services]: https://aws.amazon.com
[Simple Email Service]: https://aws.amazon.com/ses/
[Go]: https://go.dev/
[API Gateway]: https://aws.amazon.com/api-gateway/
[Lambda]: https://aws.amazon.com/lambda/
[DynamoDB]: https://aws.amazon.com/dynamodb/
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
