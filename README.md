# EListMan - Email List Manager

Mailing list system providing address validation and unsubscribe URIs.

Source: <https://github.com/mbland/elistman>

[![License](https://img.shields.io/github/license/mbland/elistman.svg)](https://github.com/mbland/elistman/blob/main/LICENSE.txt)
[![CI/CD pipeline status](https://github.com/mbland/elistman/actions/workflows/pipeline.yaml/badge.svg)](https://github.com/mbland/elistman/actions/workflows/pipeline.yaml?branch=main)
[![Coverage Status](https://coveralls.io/repos/github/mbland/elistman/badge.svg?branch=main)](https://coveralls.io/github/mbland/elistman?branch=main)

_(Try force reloading the page to get the latest badges if this is a return
visit. [The browser cache may hide the latest
results](https://stackoverflow.com/a/37894321).)_

Only serves one list at a time as defined by deployment parameters.

Implemented in [Go][] using the following [Amazon Web Services][]:

- [API Gateway][]
- [Lambda][]
- [DynamoDB][]
- [Simple Email Service][]
- [Simple Notification Service][]

Uses [CloudFormation][] and the [AWS Serverless Application Model (SAM)][] for
deploying the Lambda function, binding to the API Gateway, managing permissions,
and other configuration parameters.

Initially based on implementation hints from [victoriadrake/simple-subscribe][],
but otherwise contains original code.

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

### Configure AWS Simple Notification Service (SNS)

Create a topic in SNS for the SES Receipt Rules described in the next section.
The most straightforward option is to create a rule for [Email notifications].

### Configure AWS Simple Email Service (SES)

Set up SES in the region selected in the above step. Make sure to enable DKIM
and create a verified domain identity per [Verifying your domain for Amazon SES
email receiving][].

Create a [Receipt Rule Set][] and set it as Active. Add a Receipt Rule to the
active Receipt Rule Set that includes recipient conditions for the `postmaster`
and `abuse` accounts. Add any other recipient conditions for this rule as you
wish, as well as any other receipt rules. EListMan will add a Receipt Rule for
an unsubscribe email address to this Receipt Rule Set.

When you're ready for the system to go live, [publish an MX record for Amazon SES email receiving][].

It's also advisable to configure your [account-level suppression list][] to
automatically add addresses resulting in bounces and complaints.

Assuming you have your AWS CLI environment set up correctly, this should confirm
that SES is properly configured (with your own identity listed, of course):

```json
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

You can also view other account attributes, such as account suppression list
status, via:

```json
$ aws sesv2 get-account

{
   ...
    "SendingEnabled": true,
    "SuppressionAttributes": {
        "SuppressedReasons": [
            "BOUNCE",
            "COMPLAINT"
        ]
    },
    "Details": {
        "MailType": "MARKETING",
        "WebsiteURL": "https://mike-bland.com/",
        "ContactLanguage": "EN",
        "UseCaseDescription": "This is for publishing blog posts to email subscribers.",
        "AdditionalContactEmailAddresses": [
            "mbland@acm.org"
        ],
        ...
    }
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

### Build the `elistman` CLI

Build the `elistman` command line interface program in the root directory via:

```sh
go build
```

Run the command and check the output to see if it was successful:

```sh
$ ./elistman

Mailing list system providing address validation and unsubscribe URIs

Usage:
  elistman [command]

Available Commands:
  ...
```

### Create the DynamoDB table

Run `elistman create-subscribers-table <TABLE_NAME>` to create the DynamoDB
table, replacing `<TABLE_NAME>` with a table name of your choice. Then run `aws
dynamodb list-tables` to confirm that the new table is present.

## Deployment

Create a `deploy.env` file in the root directory of the following form
(replacing each value with your own as appropriate):

```sh
API_DOMAIN_NAME="api.mike-bland.com"
API_MAPPING_KEY="email"
EMAIL_DOMAIN_NAME="mike-bland.com"
EMAIL_SITE_TITLE="Mike Bland"
SENDER_NAME="Mike Bland's blog"
SENDER_USER_NAME="posts"
UNSUBSCRIBE_USER_NAME="unsubscribe"
RECEIPT_RULE_SET_NAME="mike-bland.com"
SUBSCRIBERS_TABLE_NAME="<TABLE_NAME>"
MAX_BULK_SEND_CAPACITY="0.8"

INVALID_REQUEST_PATH="/subscribe/malformed.html"
ALREADY_SUBSCRIBED_PATH="/subscribe/already-subscribed.html"
VERIFY_LINK_SENT_PATH="/subscribe/confirm.html"
SUBSCRIBED_PATH="/subscribe/hello.html"
NOT_SUBSCRIBED_PATH="/unsubscribe/not-subscribed.html"
UNSUBSCRIBED_PATH="/unsubscribe/goodbye.html"
```

Then run `make deploy`.

## URI Schema

- `https://<api_hostname>/<route_key>/<operation>`
- `mailto:<unsubscribe_user>@<email_hostname>?subject=<email>%20<uid>`

Where:

- `<api_hostname>`: Hostname for the API Gateway instance
- `<route_key>`: Route key for the API Gateway
- `<operation>`: Endpoint for the list management operation:
  - `/subscribe`
  - `/verify/<email>/<uid>`
  - `/unsubscribe/<email>/<uid>`
- `<email>`: Subscriber's URI encoded (for `https`) or query encoded (for
  `mailto`) email address
- `<uid>`: Identifier assigned to the subscriber by the system
- `<unsubscribe_user>`: The username receiving unsubscribe emails, typically
  `unsubscribe`.
- `<email_hostname>`: Hostname serving as an SES verified identity for receiving email

## Development

The [Makefile](./Makefile) is very short and readable. Use it to run common
tasks, or learn common commands from it to use as you please.

For guidance on writing Go developer documentation, see [Go Doc Comments][].

There are two ways to view the developer documentation in a web browser.

### Viewing documentation with `godoc`

[godoc][] is reportedly deprecated, but still works well. See:

- [golang/go: x/tools/cmd/godoc: document as deprecated #49212](https://github.com/golang/go/issues/49212)
- [349051: cmd/godoc: deprecate and point to cmd/pkgsite](https://go-review.googlesource.com/c/tools/+/349051)

```sh
# Install the godoc tool.
$ go install -v golang.org/x/tools/cmd/godoc@latest

# Serve documentation from the local directory at http://localhost:6060.
$ godoc -http=:6060
```

You can then view the EListMan docs locally at:

- <http://localhost:6060/pkg/github.com/mbland/elistman/>

One of the nice features of `godoc` is that you can view documentation for
unexported symbols by adding `?m=all` to the URL. For example:

- <http://localhost:6060/pkg/github.com/mbland/elistman/?m=all>

### Viewing documentation with `pkgsite`

[pkgsite][] is the newer development documentation publishing system.

```sh
# Install the pkgsite tool.
$ go install golang.org/x/pkgsite/cmd/pkgsite@latest

# Serve documentation from the local directory at http://localhost:8080.
$ pkgsite
```

You can then view the EListMan docs locally at:

- <http://localhost:8080/github.com/mbland/elistman>

Note that, unlike `godoc`, `pkgsite` doesn't provide an option to serve documentation for unexported symbols.

## Algorithms

Unless otherwise noted, all responses will be [HTTP 303 See Other][], with the
target page specified in the [Location HTTP header][].

- The one exception will be unsubscribe requests from mail clients using the
  `List-Unsubscribe` and `List-Unsubscribe-Post` email headers.

### Generating a new subscriber verification link

1. An HTTP request from the API Gateway comes in, containing the email address
   of a potential subscriber.
1. Validate the email address.
   1. Parse the name as closely as possible to [RFC 5322 Section 3.2.3][] via [net/mail.ParseAddress][].
   1. Reject any common aliases, like "no-reply" or "postmaster."
   1. Check the MX records of the host by:
      1. Doing a reverse lookup on each mail host's IP addresses.
      1. Looking up the IP addresses of the hosts returned by the reverse lookup.
      1. Confirming at least one reverse lookup host IP address matches a mail
         host IP address.
   1. If it fails validation, return the `INVALID_REQUEST_PATH`.
1. Look for an existing DynamoDB record for the email address.
   1. If it exists, return the `VERIFY_LINK_SENT_PATH` for `Pending` subscribers
      and `ALREADY_SUBSCRIBED_PATH` for `Verified` subscribers.
1. Generate a UID.
1. Write a DynamoDB record containing the email address, the UID, a timestamp,
   and with `SubscriberStatus` set to `Pending`.
1. Generate a verification link using the email address and UID.
1. Send the verification link to the email address.
   1. If the mail bounces or fails to send, return the `INVALID_REQUEST_PATH`.
1. Return the `VERIFY_LINK_SENT_PATH`.

### Responding to a subscriber verification link

1. An HTTP request from the API Gateway comes in, containing a subscriber's
   email address and UID.
1. Check whether there is a record for the email address in DynamoDB.
   1. If not, return the `NOT_SUBSCRIBED_PATH`.
1. Check whether the UID matches that from the DynamoDB record.
   1. If not, return the `NOT_SUBSCRIBED_PATH`.
1. If the subscriber's status is `Verified`, return the
   `ALREADY_SUBSCRIBED_PATH`.
1. Set the `SubscriberStatus` of the record to `Verified`.
1. Return the `SUBSCRIBED_PATH`.

### Responding to an unsubscribe request

1. Either an HTTP Request from the API Gateway or a mailto: event from SES comes
   in, containing a subscriber's email address and UID.
1. Check whether there is a record for the email address in DynamoDB.
   1. If not, return the `NOT_SUBSCRIBED_PATH`.
1. Check whether the UID matches that from the DynamoDB record.
   1. If not, return the `NOT_SUBSCRIBED_PATH`.
1. Delete the DynamoDB record for the email address.
1. If the request was an HTTP Request:
   1. If it uses the `POST` method, and the data contains
      `List-Unsubscribe=One-Click`, return [HTTP 204 No Content][].
   1. Otherwise return the `UNSUBSCRIBED_PATH` page.

### Expiring unused subscriber verification links

[DynamoDB's Time To Live feature][] will eventually remove expired pending subscriber records after 24 hours.

### Send rate throttling and send quota capacity limiting

EListMan calls the SES v2 `getAccount` API method once a minute to monitor
sending quotas and to adjust the send rate. Every individual message sent,
including both subscription verification messages and messages sent to the list,
will honor the current send rate.

The `MAX_BULK_SEND_CAPACITY` parameter specifies what percentage of the 24 hour
send quota may be used for sending emails to the list. This helps avoid
exceeding the daily quota before a message has been sent to all subscribers.
`elistman send` will fail, before sending an email, if sending it would result
in exceeding the percentage of the daily send quota specified by
`MAX_BULK_SEND_CAPACITY`.

The default is to use 80% of the available daily send quota for list messages,
expressed as `MAX_BULK_SEND_CAPACITY="0.8"`. The remaining 20% acts as a buffer.
For example, for a quota of 50,000 messages, up to 40,000 (50,000 * 0.8) are
available to `elistman send` within a 24 hour period.

Subscription verification messages are not affected by the
`MAX_BULK_SEND_CAPACITY` constraint. The buffer defined by
`MAX_BULK_SEND_CAPACITY` can ensure that there is always daily send quota
available for such messages.

- [Managing your Amazon SES sending limits][]
- [Errors related to the sending quotas for your Amazon SES account][]
- [How to handle a "Throttling – Maximum sending rate exceeded" error][]
- [How to Automatically Prevent Email Throttling when Reaching Concurrency Limit][]

## Unimplemented/possible future features

### Sending a test message to a specific address

This would involve extending email.SendEvent, sendHandler.HandleEvent, and
agent.SubscriptionAgent. Should be easy, but it will have to come after the
initial production launch.

### Automated End-to-End tests

This is something I _really_ want to pull off, but without blocking the first
release.

Here is what I anticipate the implementation will involve (beyond some of the existing cases in [bin/smoke-test.sh](./bin/smoke-test.sh)):

#### Setup

- Create a new random test username.
- Create a S3 bucket for the emails received by the random test user.
- Create a receipt rule for the active rule set on the domain to send emails to
  the new random test username to the S3 bucket.
  - Add the random test username to the recipient conditions.
  - Add an action to write to the S3 bucket
- Bring up a CloudFormation/Serverless Application Model stack defining these
  resources.

#### Execution

For each permutation described below:

- Send a request to subscribe the valid random username.
- Read the S3 bucket to get the validation URL.
- Request the validation URL.
- Form an unsubscribe request (either URL or mailto) from the validation URL.

#### Permutations

- Subscribe via urlencoded params, unsubscribe via urlencoded params
- Subscribe via form-data, unsubscribe via form-data params
- Subscribe via urlencoded params, unsubscribe via email
- Try to resubscribe to expect an `ALREADY_SUBSCRIBED_PATH` response.
- Modify the UID in the verification URL to expect an `INVALID_REQUEST_PATH`
  response.

#### Teardown

- Tear down the stack, which will:
  - Tear down the receipt rules
  - Tear down the test bucket

## Open Source License

This software is made available as [Open Source software][oss-def] under the
[Mozilla Public License 2.0][]. For the text of the license, see the
[LICENSE.txt](LICENSE.txt) file.

## References

- [Building Lambda functions with Go][]
- [Using AWS Lambda with other services][]
- [Using AWS Lambda with Amazon API Gateway][]
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
- [The Use of URLs as Meta-Syntax for Core Mail List Commands and their
  Transport through Message Header Fields - RFC 2369][RFC 2369]
- [Signaling One-Click Functionality for List Email Headers - RFC 8058][RFC
  8058]
- [List-Unsubscribe header critical for sustained email delivery][]
- [The Email Marketers Guide to Using List-Unsubscribe][]
- [List Unsubscribe Header in Email][]
- [Prevent mail to Gmail users from being blocked or sent to spam][]
- [Stack Overflow: Post parameter in path or in body][]
- [How to Verify Email Address Without Sending an Email][]
- [25, 2525, 465, 587, and Other Numbers: All About SMTP Ports][]
- [How to Choose the Right SMTP Port (Port 25, 587, 465, or 2525)][]
- [Which SMTP port should I use? Understanding ports 25, 465 & 587][]
- [Using curl to send email][]
- [Email Sender Reputation Made Simple][]
- [Why Go: Command-line Interfaces (CLIs)][]
- [spf13/cobra][]
- [AWS Lambda function logging in Go][]
- [Working with stages for HTTP APIs][]
- [AWS Lambda function errors in Go][]
- [RFC 9110: HTTP Semantics][]
- [Golang Auto Build Versioning][]
- [jasonmf/go-embed-version][]
- [Using ldflags to Set Version Information for Go Applications][]
- [go tool link][] (also `go tool link -help`)
- [A better way than “ldflags” to add a build version to your Go binaries][]
- [AWS Lambda function versions][]
- [Sending test emails in Amazon SES with the simulator][]
- [Setting up event notification for Amazon SES][]
- [Receiving Amazon SES notifications using Amazon SNS][]
- [Contents of event data that Amazon SES publishes to Amazon SNS][]
- [How email sending works in Amazon SES][]
- [Specifying a configuration set when you send email][]
- [RFC 2782: A DNS RR for specifying the location of services (DNS SRV)][]
- [RFC 4409: Message Submission for Mail][]
- [RFC 6186: Use of SRV Records for Locating Email Submission/Access Services][]

[Go]: https://go.dev/
[Amazon Web Services]: https://aws.amazon.com
[API Gateway]: https://aws.amazon.com/api-gateway/
[Lambda]: https://aws.amazon.com/lambda/
[DynamoDB]: https://aws.amazon.com/dynamodb/
[Simple Email Service]: https://aws.amazon.com/ses/
[Simple Notification Service]: https://aws.amazon.com/sns/
[CloudFormation]: https://aws.amazon.com/cloudformation/
[AWS Serverless Application Model (SAM)]: https://aws.amazon.com/serverless/sam/
[victoriadrake/simple-subscribe]: https://github.com/victoriadrake/simple-subscribe/
[GnuWin32.Make]: https://winget.run/pkg/GnuWin32/Make
[Amazon Simple Email Service endpoints and quotas]: https://docs.aws.amazon.com/general/latest/gr/ses.html
[AWS Command Line Interface: Quick Setup]: https://docs.aws.amazon.com/cli/latest/userguide/getting-started-quickstart.html
[Email notifications]: https://docs.aws.amazon.com/sns/latest/dg/sns-email-notifications.html
[publish an MX record for Amazon SES email receiving]: https://docs.aws.amazon.com/ses/latest/dg/receiving-email-mx-record.html
[account-level suppression list]: https://docs.aws.amazon.com/ses/latest/dg/sending-email-suppression-list.html
[Verifying your domain for Amazon SES email receiving]: https://docs.aws.amazon.com/ses/latest/dg/receiving-email-verification.html
[Receipt Rule Set]: https://docs.aws.amazon.com/ses/latest/dg/receiving-email-receipt-rules-console-walkthrough.html
[Setting up custom domain names for HTTP APIs]: https://docs.aws.amazon.com/apigateway/latest/developerguide/http-api-custom-domain-names.html
[Go Doc Comments]: https://go.dev/doc/comment
[godoc]: https://pkg.go.dev/golang.org/x/tools/cmd/godoc
[pkgsite]: https://pkg.go.dev/golang.org/x/pkgsite/cmd/pkgsite
[HTTP 303 See Other]: https://developer.mozilla.org/en-US/docs/Web/HTTP/Status/303
[Location HTTP Header]: https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Location
[RFC 5322 Section 3.2.3]: https://datatracker.ietf.org/doc/html/rfc5322#section-3.2.3
[net/mail.ParseAddress]: https://pkg.go.dev/net/mail#ParseAddress
[DynamoDB's Time To Live feature]: https://docs.aws.amazon.com/amazondynamodb/latest/developerguide/TTL.html
[Managing your Amazon SES sending limits]: https://docs.aws.amazon.com/ses/latest/dg/manage-sending-quotas.html
[Errors related to the sending quotas for your Amazon SES account]: https://docs.aws.amazon.com/ses/latest/dg/manage-sending-quotas-errors.html
[How to handle a "Throttling – Maximum sending rate exceeded" error]: https://aws.amazon.com/blogs/messaging-and-targeting/how-to-handle-a-throttling-maximum-sending-rate-exceeded-error/
[How to Automatically Prevent Email Throttling when Reaching Concurrency Limit]: https://aws.amazon.com/blogs/messaging-and-targeting/prevent-email-throttling-concurrency-limit/
[oss-def]:     https://opensource.org/osd-annotated
[HTTP 204 No Content]: https://developer.mozilla.org/en-US/docs/Web/HTTP/Status/204
[Mozilla Public License 2.0]: https://www.mozilla.org/en-US/MPL/
[Building Lambda functions with Go]: https://docs.aws.amazon.com/lambda/latest/dg/lambda-golang.html
[Using AWS Lambda with other services]: https://docs.aws.amazon.com/lambda/latest/dg/lambda-services.html
[Using AWS Lambda with Amazon API Gateway]: https://docs.aws.amazon.com/lambda/latest/dg/services-apigateway.html
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
[RFC 2369]: https://www.rfc-editor.org/rfc/rfc2369
[RFC 8058]: https://www.rfc-editor.org/rfc/rfc8058
[List-Unsubscribe header critical for sustained email delivery]: https://www.postmastery.com/list-unsubscribe-header-critical-for-sustained-email-delivery/
[The Email Marketers Guide to Using List-Unsubscribe]: https://www.litmus.com/blog/the-ultimate-guide-to-list-unsubscribe/
[List Unsubscribe Header in Email]: https://mailtrap.io/blog/list-unsubscribe-header/
[Prevent mail to Gmail users from being blocked or sent to spam]: https://support.google.com/mail/answer/81126
[Stack Overflow: Post parameter in path or in body]: https://stackoverflow.com/questions/42390564/post-parameter-in-path-or-in-body
[How to Verify Email Address Without Sending an Email]: https://mailtrap.io/blog/verify-email-address-without-sending/
[25, 2525, 465, 587, and Other Numbers: All About SMTP Ports]: https://mailtrap.io/blog/smtp-ports-25-465-587-used-for/
[How to Choose the Right SMTP Port (Port 25, 587, 465, or 2525)]: https://kinsta.com/blog/smtp-port/
[Which SMTP port should I use? Understanding ports 25, 465 & 587]: https://www.mailgun.com/blog/email/which-smtp-port-understanding-ports-25-465-587/
[Using curl to send email]: https://stackoverflow.com/questions/14722556/using-curl-to-send-email
[Email Sender Reputation Made Simple]: https://mailtrap.io/blog/email-sender-reputation/
[Why Go: Command-line Interfaces (CLIs)]: https://go.dev/solutions/clis
[spf13/cobra]: https://github.com/spf13/cobra
[AWS Lambda function logging in Go]: https://docs.aws.amazon.com/lambda/latest/dg/golang-logging.html
[Working with stages for HTTP APIs]: https://docs.aws.amazon.com/apigateway/latest/developerguide/http-api-stages.html
[AWS Lambda function errors in Go]: https://docs.aws.amazon.com/lambda/latest/dg/golang-exceptions.html
[RFC 9110: HTTP Semantics]: https://www.rfc-editor.org/rfc/rfc9110.html
[Golang Auto Build Versioning]: https://www.atatus.com/blog/golang-auto-build-versioning/
[jasonmf/go-embed-version]: https://github.com/jasonmf/go-embed-version
[Using ldflags to Set Version Information for Go Applications]: https://www.digitalocean.com/community/tutorials/using-ldflags-to-set-version-information-for-go-applications
[go tool link]: https://pkg.go.dev/cmd/link
[A better way than “ldflags” to add a build version to your Go binaries]: https://levelup.gitconnected.com/a-better-way-than-ldflags-to-add-a-build-version-to-your-go-binaries-2258ce419d2d
[AWS Lambda function versions]: https://docs.aws.amazon.com/lambda/latest/dg/configuration-versions.html
[Sending test emails in Amazon SES with the simulator]: https://docs.aws.amazon.com/ses/latest/dg/send-an-email-from-console.html
[Setting up event notification for Amazon SES]: https://docs.aws.amazon.com/ses/latest/dg/monitor-sending-activity-using-notifications.html
[Receiving Amazon SES notifications using Amazon SNS]: https://docs.aws.amazon.com/ses/latest/dg/monitor-sending-activity-using-notifications-sns.html
[Contents of event data that Amazon SES publishes to Amazon SNS]: https://docs.aws.amazon.com/ses/latest/dg/event-publishing-retrieving-sns-contents.html
[How email sending works in Amazon SES]: https://docs.aws.amazon.com/ses/latest/dg/send-email-concepts-process.html
[Specifying a configuration set when you send email]: https://docs.aws.amazon.com/ses/latest/dg/using-configuration-sets-in-email.html
[RFC 2782: A DNS RR for specifying the location of services (DNS SRV)]: https://www.rfc-editor.org/rfc/rfc2782.html
[RFC 4409: Message Submission for Mail]: https://www.rfc-editor.org/rfc/rfc4409
[RFC 6186: Use of SRV Records for Locating Email Submission/Access Services]: https://www.rfc-editor.org/rfc/rfc6186.html
