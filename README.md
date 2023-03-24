# EListMan - Email List Manager

Mailing list system providing address validation and unsubscribe URIs.

_**NOTE:** This project is just beginning. When it's operational, I'll remove
this notice._

Only serves one list at a time as defined by deployment parameters. (_I may
change this in a future version._)

Implemented in [Go][] using the following [Amazon Web Services][]:

- [API Gateway][]
- [Lambda][]
- [DynamoDB][]
- [Simple Email Service][]

Uses [CloudFormation][] and the [AWS Serverless Application Model (SAM)][] for
deploying the Lambda function, binding to the API Gateway, managing permissions,
and other configuration parameters.

Based on implementation hints from [victoriadrake/simple-subscribe][], but
otherwise contains original code.

## Prerequisites

### Install tools

Run the `bin/check-tools.sh` script to check that the required tools are
installed.

- This script will try to install some missing tools itself. If any are missing,
  the script will provide a link to installation instructions.

- **Note**: The script does _not_ check for the presence of `make`, as it comes
  in many flavors and aliases and already ships with many operating systems. The
  notable exception is Microsoft Windows.

  However, if the `winget` command is available, you can install the
  [GnuWin32.Make][] package:

  ```cmd
  winget install -e --id GnuWin32.Make
  ```

  Then:
  - Press _**Win-R**_ and enter `systempropertiesadvanced` to open the _**System
    Properties > Advanced**_ pane.
  - Click the _**Environment Variables...**_ button.
  - Select _**Path**_ in either the _User variables_ or _System variables_ pane,
    then click the corresponding _**Edit...**_ button.
  - Click the _**New**_ button, then click the _**Browse...**_ button.
  - Navigate to _**This PC > Local Disk (C:) > Program Files (x86) > GnuWin32 > bin**_.
  - Click the _**OK**_ button, then keep clicking the _**OK**_ button until all
    of the _**System Properties**_ panes are closed.

  Make should then be available as either `make` or `make.exe`.

### Configure the AWS CLI

Configure your credentials for a region from the **Email Receiving Endpoints**
section of [Amazon Simple Email Service endpoints and quotas][].

Follow the guidance on the [AWS Command Line Interface: Quick Setup][] page if
necessary.

### Configure AWS Simple Email Service (SES)

Set up SES in the region selected in the above step. Make sure to enable DKIM
and create a verified domain identity per [Verifying your domain for Amazon SES
email receiving][].

Assuming you have your AWS CLI environment set up correctly, this should confirm
that SES is properly configured (with your own identity listed, of course):

```sh
$ aws sesv2 list-email-identities

{
    "EmailIdentities": [
        {
            "IdentityType": "DOMAIN",
            "IdentityName": "mike-bland.com",
            "SendingEnabled": true,
            "VerificationStatus": "SUCCESS"
        }
    ]
}
```

### Configure AWS API Gateway

Set up a custom domain name in API Gateway in the region selected in the
above step. Create a SSL certificate in Certificate Manager for it as well.

- [Setting up custom domain names for HTTP APIs][]

If done correctly, the following command should produce output resembling the
example:

```sh
$ aws apigatewayv2 get-domain-names

{
    "Items": [
        {
            "ApiMappingSelectionExpression": "$request.basepath",
            "DomainName": "api.mike-bland.com",
            "DomainNameConfigurations": [
                {
                    "ApiGatewayDomainName": "<...>",
                    "CertificateArn": "<...>",
                    "DomainNameStatus": "AVAILABLE",
                    "EndpointType": "REGIONAL",
                    "HostedZoneId": "<...>",
                    "SecurityPolicy": "TLS_1_2"
                }
            ]
        }
    ]
}
```

### Create the DynamoDB table

Run the `bin/create-subscribers-table.sh` script to create the DynamoDB table,
using a table name of your choice. Then run `aws dynamodb list-tables` to
confirm that the new table is present.

## Deployment

Create a `deploy.env` file in the root directory of the following form
(replacing each value with your own as appropriate):

```sh
API_DOMAIN_NAME="api.mike-bland.com"
API_ROUTE_KEY="email"
EMAIL_DOMAIN_NAME="mike-bland.com"
SENDER_NAME="Mike Bland's blog"
SUBSCRIBERS_TABLE_NAME="TABLE_NAME"
```

Then run `make deploy`.

## URI Schema

- `https://<api_hostname>/<route_key>/<operation>`
- `mailto:unsubscribe@<email_hostname>?subject=<email>%20<uid>`

Where:

- `<api_hostname>`: Hostname for the API Gateway instance
- `<route_key>`: Route key for the API Gateway
- `<operation>`: Endpoint for the list management operation:
  - `/subscribe/<email>`
  - `/verify/<email>/<uid>`
  - `/unsubscribe/<email>/<uid>`
- `<email>`: Subscriber's URI encoded (for `https`) or query encoded (for
  `mailto`) email address
- `<uid>`: Identifier assigned to the subscriber by the system
- `<email_hostname>`: Hostname serving as an SES verified identity for receiving email

## Development

The [Makefile](./Makefile) is very short and readable. Use it to run common
tasks, or learn common commands from it to use as you please.

## Algorithms

Unless otherwise noted, all responses will be [HTTP 303 See Other][], with the
target page specified in the [Location HTTP header][].

- The one exception will be unsubscribe requests from mail clients using the
  `List-Unsubscribe` and `List-Unsubscribe-Post` email headers.

### Generating a new subscriber verification link

1. An HTTP request from the API Gateway comes in, containing the email address
   of a potential subscriber.
1. Validate the email address.
   1. Parse the name as closely as possible to [RFC 5322 Section 3.2.3][].
   1. Reject any common aliases, like "no-reply" or "postmaster."
   1. Check the MX record of the host, or find an A or AAAA record that accepts
      connections on port 587 or 2525.
   1. If it fails validation, return the `INVALID` page.
1. Look for an existing DynamoDB record for the email address.
   1. If it exists and `SubscriberStatus` is `Verified`, return the
      `ALREADY_SUBSCRIBED` page.
1. Generate a UID.
1. Write a DynamoDB record containing the email address, the UID, a timestamp,
   and with `SubscriberStatus` set to `Unverified`.
1. Generate a verification link using the email address and UID.
1. Send the verification link to the email address.
   1. If the mail bounces or fails to send, return the `INVALID` page.
1. Return the `CONFIRM` page.

### Responding to a subscriber verification link

1. An HTTP request from the API Gateway comes in, containing a subscriber's
   email address and UID.
1. Check whether there is a record for the email address in DynamoDB.
   1. If not, return the `NOT_FOUND` page.
1. Check whether the UID matches that from the DynamoDB record.
   1. If not, return the `NOT_FOUND` page.
1. Set the `SubscriberStatus` of the record to `Verified`.
1. Send a `SUBSCRIBED` email containing the unsubscribe link.
1. Return the `SUBSCRIBED` page.

### Responding to an unsubscribe request

1. Either an HTTP Request from the API Gateway or a mailto: event from SES comes
   in, containing a subscriber's email address and UID.
1. Check whether there is a record for the email address in DynamoDB.
   1. If not, return the `NOT_FOUND` page.
1. Check whether the UID matches that from the DynamoDB record.
   1. If not, return the `NOT_FOUND` page.
1. Delete the DynamoDB record for the email address.
1. If the request was an HTTP Request:
   1. If it uses the `POST` method, and the data contains
      `List-Unsubscribe=One-Click`, return [HTTP 204 No Content][].
   1. Otherwise return the `UNSUBSCRIBED` page.

### Expiring unused subscriber verification links

1. Retrieve all existing DynamoDB records.
1. Delete any `Unverified` records exceeding the timeout (1h).

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
- [One-Click List-Unsubscribe Header – RFC 8058][]
- [List-Unsubscribe header critical for sustained email delivery][]
- [Stack Overflow: Post parameter in path or in body][]
- [How to Verify Email Address Without Sending an Email][]
- [25, 2525, 465, 587, and Other Numbers: All About SMTP Ports][]
- [How to Choose the Right SMTP Port (Port 25, 587, 465, or 2525)][]
- [Which SMTP port should I use? Understanding ports 25, 465 & 587][]
- [Using curl to send email][]

[Go]: https://go.dev/
[Amazon Web Services]: https://aws.amazon.com
[API Gateway]: https://aws.amazon.com/api-gateway/
[Lambda]: https://aws.amazon.com/lambda/
[DynamoDB]: https://aws.amazon.com/dynamodb/
[Simple Email Service]: https://aws.amazon.com/ses/
[CloudFormation]: https://aws.amazon.com/cloudformation/
[AWS Serverless Application Model (SAM)]: https://aws.amazon.com/serverless/sam/
[victoriadrake/simple-subscribe]: https://github.com/victoriadrake/simple-subscribe/
[GnuWin32.Make]: https://winget.run/pkg/GnuWin32/Make
[Amazon Simple Email Service endpoints and quotas]: https://docs.aws.amazon.com/general/latest/gr/ses.html
[AWS Command Line Interface: Quick Setup]: https://docs.aws.amazon.com/cli/latest/userguide/getting-started-quickstart.html
[Verifying your domain for Amazon SES email receiving]: https://docs.aws.amazon.com/ses/latest/dg/receiving-email-verification.html
[Setting up custom domain names for HTTP APIs]: https://docs.aws.amazon.com/apigateway/latest/developerguide/http-api-custom-domain-names.html
[HTTP 303 See Other]: https://developer.mozilla.org/en-US/docs/Web/HTTP/Status/303
[Location HTTP Header]: https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Location
[RFC 5322 Section 3.2.3]: https://datatracker.ietf.org/doc/html/rfc5322#section-3.2.3
[oss-def]:     https://opensource.org/osd-annotated
[HTTP 204 No Content]: https://developer.mozilla.org/en-US/docs/Web/HTTP/Status/204
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
[One-Click List-Unsubscribe Header – RFC 8058]: https://certified-senders.org/wp-content/uploads/2017/07/CSA_one-click_list-unsubscribe.pdf
[List-Unsubscribe header critical for sustained email delivery]: https://www.postmastery.com/list-unsubscribe-header-critical-for-sustained-email-delivery/
[Stack Overflow: Post parameter in path or in body]: https://stackoverflow.com/questions/42390564/post-parameter-in-path-or-in-body
[How to Verify Email Address Without Sending an Email]: https://mailtrap.io/blog/verify-email-address-without-sending/
[25, 2525, 465, 587, and Other Numbers: All About SMTP Ports]: https://mailtrap.io/blog/smtp-ports-25-465-587-used-for/
[How to Choose the Right SMTP Port (Port 25, 587, 465, or 2525)]: https://kinsta.com/blog/smtp-port/
[Which SMTP port should I use? Understanding ports 25, 465 & 587]: https://www.mailgun.com/blog/email/which-smtp-port-understanding-ports-25-465-587/
[Using curl to send email]: https://stackoverflow.com/questions/14722556/using-curl-to-send-email
