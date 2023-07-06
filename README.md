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
- [Web Application Firewall][]

Uses [CloudFormation][] and the [AWS Serverless Application Model (SAM)][] for
deploying the Lambda function, binding to the API Gateway, managing permissions,
and other configuration parameters.

Originally implemented to support <https://mike-bland.com/subscribe/>.

The very earliest stages of the implementation were based on hints from
[victoriadrake/simple-subscribe][], but all the code is original.

## Open Source License

This software is made available as [Open Source software][oss-def] under the
[Mozilla Public License 2.0][]. For the text of the license, see the
[LICENSE.txt](LICENSE.txt) file.

## Setup

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

Create a [Receipt Rule Set][] and set it as Active. EListMan will add a Receipt Rule for an unsubscribe email address to this Receipt Rule Set.

Create a Receipt Rule to receive [Email notifications][] for the `postmaster`
and `abuse` accounts, along with any other accounts that you'd like.

- You can add these recipient conditions manually, which would require creating
  a Simple Notification Service (SNS) topic manually as well.
- Alternatively, consider using [mbland/ses-forwarder][] to automate configuring
  the Receipt Rule Set with an appropriate Receipt Rule.

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
status, send quotas, and send rates, via:

```json
$ aws sesv2 get-account

{
   ...
    "ProductionAccessEnabled": true,
    "SendQuota": {
        "Max24HourSend": ...,
        "MaxSendRate": ...,
        "SentLast24Hours": ...
    },
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

### Configure API Gateway to write CloudWatch logs

Next, [set up an IAM role to allow the API to write CloudWatch logs][]. You only
need to execute the steps in the **Create an IAM role for logging to
CloudWatch** section. One possible name for the new IAM role would be
`ApiGatewayCloudWatchLogging`.

The last step from the above instructions is to

```sh
$ ARN="arn:aws:iam::...:role/ApiGatewayCloudWatchLogging"
$ aws apigateway update-account --patch-operations \
    op='replace',path='/cloudwatchRoleArn',value='$ARN'
```

If successful, the output should resemble the following, where `<ARN>` is the value of `$ARN` from above:

```sh
{
    "cloudwatchRoleArn": "<ARN>",
    "throttleSettings": {
        "burstLimit": ...,
        "rateLimit": ...
    },
    "features": []
}
```

_Note:_ Per the documentation for the [AWS::ApiGateway::Account CloudFormation
entity][], "you should only have one `AWS::ApiGateway::Account` resource per
region per account." This is why it's not included in `template.yml` in favor of
the one-time-per-account instructions above.

However, if you want to try using SAM/CloudFormation to manage it, see:

- [Stack Overflow: Configuring logging of AWS API Gateway - Using a SAM
  template][]

### Run tests

To make sure the local environment is in good shape, and your AWS services are
properly configured, run the main test suite via `make test`. (Note that the
example output below is slightly edited for clarity.)

```sh
$ make test

go vet -tags=all_tests ./...
go run honnef.co/go/tools/cmd/staticcheck -tags=all_tests ./...
go build -tags=all_tests ./...

go test -tags=small_tests ./...
ok      github.com/mbland/elistman/agent        0.110s
ok      github.com/mbland/elistman/db   0.392s
ok      github.com/mbland/elistman/email        0.187s
ok      github.com/mbland/elistman/handler      0.260s
ok      github.com/mbland/elistman/ops  0.461s
ok      github.com/mbland/elistman/types        0.523s

go test -tags=medium_tests -count=1 ./...
ok      github.com/mbland/elistman/db   2.970s
ok      github.com/mbland/elistman/email        1.150s

go test -tags=contract_tests -count=1 ./db -args -awsdb
ok      github.com/mbland/elistman/db   44.264s
```

If you're using [Visual Studio Code][], you can run all but the last test via
the **Test: Run All Tests** command (`testing.runAll`). The default keyboard shortcut is **⌘; A**.

- The project VS Code configuration is in
  [.vscode/settings.json](.vscode/settings.json).
- For other helpful testing-related keyboard shortcuts, press **⌘K ⌘S**, then
  search for `testing`.

#### Test sizes

The tests are divided into suites of varying [test sizes][], described below,
using [Go build constraints][] (a.k.a. "build tags"). These constraints are
specified on the first line of every test file:

```sh
$ head -n1 */*_test.go

==> agent/agent_test.go <==
//go:build small_tests || all_tests

# ...snip...

# See "Test coverage" section below for an explanation of the
# dynamodb_contract_test build constraints.
==> db/dynamodb_contract_test.go <==
//go:build ((medium_tests || contract_tests) && !no_coverage_tests) || coverage_tests || all_tests

# ...snip...

==> email/mailer_contract_test.go <==
//go:build medium_tests || contract_tests || all_tests

# ...etc...
```

##### Small tests

The `small_tests` all run locally, with no external dependencies. These tests
cover all fine details and error conditions.

##### Medium/contract tests

Each of the `medium_tests` exercises integration with specific dependencies.
Most of these dependencies are actual, live AWS services that require a network
connection.

These tests are designed to set up required state and clean up any side effects.
Other than ensuring the network is available, and the required resources are
running and accessible, no external intervention is necessary.

`medium_tests` validate high level use cases and fundamental assumptions, _not_
exhaustive details and error conditions. That's what the `small_tests` are for,
resulting in fewer, less complicated, faster, and more stable `medium_tests`.

Each of the `contract_tests` are also `medium_tests`. In fact, it's arguable
that these tags are redundant, but I want the reader to contemplate both
concepts and their equivalence.

The medium/contract tests in `db/dynamodb_contract_test.go` run against:

- a local [Docker][] container running the [amazon/dynamodb-local][] image when
  run without the `-awsdb` flag
  - e.g. When run via `go test -tags=medium_tests -count=1 ./...`, in VS Code
    via **⌘; A**, or in CI via `-tags=coverage_tests`, described below.
- the actual DynamoDB for your AWS account when run with the `-awsdb` flag
  - e.g. When run via `go test -tags=contract_tests -count=1 ./db -args -awsdb`

_Note:_ `-count=1` is the Go idiom to ensure tests are run with caching
disabled, per `go help testflag`.

##### Large tests and smoke tests

There are no end-to-end `large_tests` yet, outside of `bin/smoke-tests.sh`. The
smoke tests are described below, as are the plans for adding end-to-end tests
one day.

#### Test coverage

To check code coverage, you can run:

```sh
$ make coverage

go test -covermode=count -coverprofile=coverage.out \
          -tags=small_tests,coverage_tests ./...

ok  github.com/mbland/elistman/agent    0.351s  coverage: 100.0% of statements
ok  github.com/mbland/elistman/db       3.214s  coverage: 100.0% of statements
ok  github.com/mbland/elistman/email    0.539s  coverage: 100.0% of statements
ok  github.com/mbland/elistman/handler  0.613s  coverage: 100.0% of statements
ok  github.com/mbland/elistman/ops      0.457s  coverage: 100.0% of statements
ok  github.com/mbland/elistman/types    0.675s  coverage: 100.0% of statements

go tool cover -html=coverage.out
[ ...opens default browser with HTML coverage results... ]
```

You can also check coverage in VS Code by searching for the **Go: Toggle Test
Coverage in Current Package** command via **Show All Commands** (⇧⌘P).

Note that `db/dynamodb_contract_test.go` is the one and only `medium_test` that
we need for test coverage purposes. It contains the `coverage_tests` build
constraint, enabling the CI pipeline to collect its coverage data without
running other `medium_tests`.

### Build the `elistman` CLI

Build the `elistman` command line interface program in the root directory via:

```sh
go build
```

Run the command and check the output to see if it was successful:

```sh
$ ./elistman -h

Mailing list system providing address validation and unsubscribe URIs

See the https://github.com/mbland/elistman README for details.

To create a table:
  elistman create-subscribers-table TABLE_NAME

To see an example of the message input JSON structure:
  elistman preview --help

To preview a raw message before sending, where `generate-email` is any
program that creates message input JSON:
  generate-email | elistman preview

To send an email to the list, given the STACK_NAME of the EListMan instance:
  generate-email | elistman send -s STACK_NAME

Usage:
  elistman [command]

Available Commands:
  [...commands snipped...]

Flags:
  -h, --help      help for elistman
  -v, --version   version for elistman

Use "elistman [command] --help" for more information about a command.
```

### Create the DynamoDB table

Run `elistman create-subscribers-table <TABLE_NAME>` to create the DynamoDB
table, replacing `<TABLE_NAME>` with a table name of your choice. Then run `aws
dynamodb list-tables` to confirm that the new table is present.

### Create the configuration file

Create the `deploy.env` configuration file in the root directory containing the
following environment variables (replacing each value with your own as
appropriate):

```sh
# This will be the name of the CloudFormation stack. The `--stack-name` flag of
# `elistman` CLI commands will require this value.
STACK_NAME="mike-blands-blog-example"

# This is the domain name configured in the "Configure AWS API Gateway" step.
API_DOMAIN_NAME="api.mike-bland.com"

# This will be the first component of the EListMan API endpoints after the
# hostname, e.g., api.mike-bland.com/email/subscribe.
API_MAPPING_KEY="email"

# The domain from which emails will be sent. This should likely match the
# website on which the subscription form appears.
EMAIL_DOMAIN_NAME="mike-bland.com"

# The proper name of the website from which emails will appear to be sent. It
# need not match to the site's <title> exactly, but should clearly describe what
# subscribers expect.
EMAIL_SITE_TITLE="Mike Bland's blog"

# The proper name of the email sender. It need not match EMAIL_SITE_TITLE, but
# again, should not surprise subscribers.
SENDER_NAME="Mike Bland's blog"

# The username of the email sender. The full address will be of the form:
# SENDER_USER_NAME@EMAIL_DOMAIN_NAME, e.g., posts@mike-bland.com.
SENDER_USER_NAME="posts"

# The username of the unsubscribe email recipient. The full address will be of
# the form: UNSUBSCRIBE_USER_NAME@EMAIL_DOMAIN_NAME, e.g.,
# unsubscribe@mike-bland.com.
UNSUBSCRIBE_USER_NAME="unsubscribe"

# The name of the Receipt Rule Set created in the "Configure AWS Simple Email
# Service (SES)" step.
RECEIPT_RULE_SET_NAME="mike-bland.com"

# The name of the DynamoDB table created via `elistman create-subscribers-table`
# in the "Create the DynamoDB table" step.
SUBSCRIBERS_TABLE_NAME="<TABLE_NAME>"

# Percentage of daily quota to consume before self-limiting bulk sends via
# `elistman send -s STACK_NAME`.  See the "Send rate throttling and send quota
# capacity limiting" step for a detailed description. (Does not apply when
# running `elistman send` with specific subscriber addresses specified on the
# command line.)
MAX_BULK_SEND_CAPACITY="0.8"

# EListMan will redirect API requests to the following URLs according to the 
# "Algorithms" described below.
INVALID_REQUEST_PATH="/subscribe/malformed.html"
ALREADY_SUBSCRIBED_PATH="/subscribe/already-subscribed.html"
VERIFY_LINK_SENT_PATH="/subscribe/confirm.html"
SUBSCRIBED_PATH="/subscribe/hello.html"
NOT_SUBSCRIBED_PATH="/unsubscribe/not-subscribed.html"
UNSUBSCRIBED_PATH="/unsubscribe/goodbye.html"
```

### Run smoke tests locally

`bin/smoke-test.sh` invokes `curl` to send HTTP requests to the running Lambda,
all of which expect an error response without any side effects (save for
logging).

To check that your configuration works locally, you'll need two separate
terminal windows to run `bin/smoke-test.sh`. In the first, run:

```sh
$ make run-local

[ ...validates template.yml, builds lambda, etc... ]
bin/sam-with-env.sh deploy.env local start-api --port 8080
[ ...more output... ]
You can now browse to the above endpoints to invoke your functions....
2023-05-29 16:08:04 WARNING: This is a development server....
 * Running on http://127.0.0.1:8080
2023-05-29 16:08:04 Press CTRL+C to quit
```

In the next terminal, run:

```sh
$ ./bin/smoke-test ./deploy.env --local

INFO: SUITE: Not found (403 locally, 404 in prod)
INFO: TEST: 1 — invalid endpoint not found
Expect 403 from: POST http://127.0.0.1:8080/foobar/mbland%40acm.org

curl -isS -X POST http://127.0.0.1:8080/foobar/mbland%40acm.org

HTTP/1.1 403 FORBIDDEN
Server: Werkzeug/2.3.4 Python/3.8.16
Date: Mon, 29 May 2023 20:19:57 GMT
Content-Type: application/json
Content-Length: 43
Connection: close

{"message":"Missing Authentication Token"}

PASSED: 1 — invalid endpoint not found:
    status: 403

INFO: TEST: 2 — /subscribe with trailing component not found
Expect 403 from: POST http://127.0.0.1:8080/subscribe/foobar

[ ...more test output/results... ]

PASSED: 6 — invalid UID for /unsubscribe:
    status: 400

PASSED: All 6 smoke tests passed!
```

Then enter CTRL-C in the first window to stop the local SAM Lambda server.

### Understand the danger of spam bots and the need for a CAPTCHA

Before deploying to production, we need to talk about spam.

The EListMan system tries to validate email addresses through its own up front
analysis and by sending validation links to subscribers. However, opportunistic
spam bots can still—and will—submit many valid email addresses without either
the knowledge or consent of the actual owner.

Fortunately, the validation link mechanism prevents most bogus subscriptions,
and [DynamoDB's Time To Live feature][] cleans them from the database
automatically. A bounce or complaint also notifies the EListMan Lambda to remove
the address and add it to the [account-level suppression list][]. The
suppression list ensures the system won't send to that address again, even if
someone attempts to resubmit it.

This means most bogus subscriptions will not pollute the verified subscriber
list, and such recipients will not receive further emails. However, generating
these bogus subscriptions still consumes resources, and their verification
emails can yield bounces and complaints that will harm your [SES reputation
metrics][].

Having learned this the hard (naïve) way, I recommend using a [CAPTCHA][] to
prevent spam bot abuse:

- When I first published my EListMan subscription form, my instance received
  dozens of bogus subscription requests a day—before I'd even announced it on my
  blog. (The form had been available before, but used a different subscription
  system.)
- After deploying a CAPTCHA, the number of bogus subscriptions dropped to zero.
  (I hope I hadn't inadvertently been allowing subscription verification spam
  all those years before....)

### Decide whether or not to use the AWS WAF CAPTCHA

EListMan's CloudFormation/SAM template configures an AWS Web Application
Firewall (WAF) CAPTCHA, creating one Web ACL and one Rule associated with it.
If you choose to use it, note that it does incur additional charges. See [AWS
WAF Pricing][] for details.

If you choose not to use it, comment out or delete the `WebAcl` and
`WebAclAssociation` resources in [template.yml](./template.yml).

### Generate an AWS Web Application Firewall CAPTCHA API KEY (optional)

To use EListMan's Web ACL configuration, you'll need to [generate an API key for
the CAPTCHA API][]. Include whichever domain will serve the submission form in
the list of domains used to generate the API key.

The default EListMan configuration expects this domain to be the same as
`EMAIL_DOMAIN_NAME`, described above.  If you use a different domain, set
`WebAcl > Properties > TokenDomains` in [template.yml](./template.yml)
appropriately.

## Deployment

### Deploy to AWS

If the smoke tests pass, deploy the EListMan system via:

```sh
make deploy
```

Once the deployment is running, run the smoke tests without the `--local` flag
to ensure your instance is reachable:

```sh
./bin/smoke-tests.sh ./deploy.env
```

### Publish your HTML subscription form

You'll need to publish a subscription [&lt;form&gt;][] similar to the following,
substituting `API_DOMAIN_NAME` with the custom domain name from the **Configure
AWS API Gateway** step:

```html
<!-- subscribe.html -->

<form method="post" action="https://API_DOMAIN_NAME/email/subscribe">
  <input name="email" type="email"
   placeholder="Please enter your email address."/>
  <button type="submit">Subscribe</button>
</form>
```

However, as mentioned above, spam bots are a thing, even for the humblest of
sites publicly sporting a [&lt;form&gt;][] element.

### Generate your email submission form programmatically (optional)

You may gain extra protection from spam bots by generating the subscription form
using JavaScript instead of embedding a [&lt;form&gt;][] element directly in
your HTML.

In other words, instead of embedding the [&lt;form&gt;][] directly in your
subscription page as shown above, use something like this:

```html
<!-- subscribe.html -->

<div class="subscribe-form">
  <button>Show subscribe form</button>
</div>
```

```js
// subscribe.js

"use strict";

document.addEventListener("DOMContentLoaded", () => {
  var container = document.querySelector(".subscribe-form")

  var showForm = () => {
    var f = document.createElement("form")
    // The following should generate the value for API_DOMAIN_NAME.
    var api_domain_name = ["my", "api", "com"].join(".")
    f.action = ["https:", "", api_domain_name, "email", "subscribe"].join("/")
    f.method = "post"

    var i = document.createElement("input")
    i.name = "email"
    i.type = "email"
    i.placeholder = "Please enter your email address."
    f.appendChild(i)

    var s = document.createElement("button")
    s.type = "submit"
    s.appendChild(document.createTextNode("Subscribe"))
    f.appendChild(s)

    container.parentNode.replaceChild(f, container)
  }

  container.querySelector("button").addEventListener('click', showForm)
})
```

### Integrate the CAPTCHA into your subscription form (optional)

Of course, the ultimate protection would be to use an AWS WAF CAPTCHA to protect
the `/subscribe` API endpoint.

Using the same HTML from above, the code below will [render the AWS WAF CAPTCHA
puzzle][] when the subscriber clicks the button.  When they solve the puzzle, it
will then reveal the submission form.

Remember to substitute `YOUR_AWS_WAF_CAPTCHA_API_KEY` with your own API key:

```js
// subscribe.js

"use strict";

document.addEventListener("DOMContentLoaded", () => {
  var container = document.querySelector(".subscribe-form")

  var showForm = () => {
    // Same implementation as above
  }

  container.querySelector("button").addEventListener('click', () => {
    AwsWafCaptcha.renderCaptcha(container, {
      apiKey: YOUR_AWS_WAF_CAPTCHA_API_KEY,
      onSuccess: showForm,
      dynamicWidth: true,
      skipTitle: true
    });
  })
})
```

### Subscribe and send a test email to yourself

After deploying EListMan and publishing your subscription form, use the form to
subscribe to the list. Then you can run the following command to send a test
email to yourself (replacing `STACK_NAME` and `MY_EMAIL_ADDRESS` as
appropriate):

```sh
$ ./bin/generate-test-message.sh ./deploy.env |
    ./elistman send -s STACK_NAME MY_EMAIL_ADDRESS
```

### Send a production email to the list

Run `./elistman send -h` to see an example email:

```sh
$ ./elistman send -h 

Reads a JSON object from standard input describing a message:

  {
    "From": "Foo Bar <foobar@example.com>",
    "Subject": "Test object",
    "TextBody": "Hello, World!",
    "TextFooter": "Unsubscribe: {{UnsubscribeUrl}}",
    "HtmlBody": "<!DOCTYPE html><html><head></head><body>Hello, World!<br/>",
    "HtmlFooter": "<a href='{{UnsubscribeUrl}}'>Unsubscribe</a></body></html>"
  }
```

You will need to generate a similar JSON object to feed into the standard input
of `./elistman send`:

- `From`, `Subject`, `TextBody`, and `TextFooter` are required.
- If `HtmlBody` is present, `HtmlFooter` must also be present.
- `TextFooter`, and `HtmlFooter` if present, must contain one and only one
  instance of the `{{UnsubscribeUrl}}` template. The EListMan Lambda will
  replace this template with the unsubscribe URL unique to each subscriber.
- `TextFooter` and `HtmlFooter` will appear on a new line immediately after
  `TextBody` and `HtmlBody`, respectively.

Provided you have a program to generate the JSON object above called
`generate-email`, you can then send an email to the list via:

```sh
generate-email | ./elistman send -s STACK_NAME
```

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

## URI Schema

- `https://<api_hostname>/<route_key>/<operation>`
- `mailto:<unsubscribe_user_name>@<email_domain_name>?subject=<email>%20<uid>`

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
- `<unsubscribe_user_name>`: The username receiving unsubscribe emails,
  typically `unsubscribe`, set via `UNSUBSCRIBE_USER_NAME`.
- `<email_domain_name>`: Hostname serving as an SES verified identity for
  sending and receiving email, set via `EMAIL_DOMAIN_NAME`

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
`elistman send` will fail, before sending an email, if the percentage of the
daily send quota specified by `MAX_BULK_SEND_CAPACITY` has already been
consumed.

The default is to use 80% of the available daily send quota for list messages,
expressed as `MAX_BULK_SEND_CAPACITY="0.8"`. The remaining 20% acts as a buffer.
For example, for a quota of 50,000 messages, up to 40,000 (50,000 * 0.8) are
available to `elistman send` within a 24 hour period.

Note that this mechanism tries to prevent the operator from accidentally
exceeding the 24 hour quota, but it's not foolproof. The operator is ultimately
responsible for ensuring that `elistman send` won't exceed the quota if
`MAX_BULK_SEND_CAPACITY` hasn't yet been reached, or for tuning it accordingly.

Building on the previous example, if there are 17,000 subscribers:

- The first `elistman send` consumes 17,000 of the quota.
- The second `elistman send` consumes the next 17,000 of the quota, for a total
  of 34,000.
- The third `elistman send` will proceed, since 34,000 is less than the 40,000
  calculated by `MAX_BULK_SEND_CAPACITY="0.8"`. However, it will consume 17,000 more of the quota, for 51,000 total, exceeding the 50,000 quota.

Subscription verification messages are not affected by the
`MAX_BULK_SEND_CAPACITY` constraint. The buffer defined by
`MAX_BULK_SEND_CAPACITY` can ensure that there is always daily send quota
available for such messages.

- [Managing your Amazon SES sending limits][]
- [Errors related to the sending quotas for your Amazon SES account][]
- [How to handle a "Throttling – Maximum sending rate exceeded" error][]
- [How to Automatically Prevent Email Throttling when Reaching Concurrency Limit][]

## Unimplemented/possible future features

### Automated End-to-End tests

This is something I _really_ want to pull off, but without blocking the first
release.

Here is what I anticipate the implementation will involve (beyond some of the existing cases in [bin/smoke-test.sh](./bin/smoke-test.sh)):

#### Test Setup

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
  - _Note:_ The `/subscribe` endpoint is CAPTCHA-protected on the dev and prod
    instances. We may need to bring up an alternate API Gateway instance
    for the test without CAPTCHA protection, or call `ProdAgent.Subscribe()`
    through another method.
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
- [Multipurpose Internet Mail Extensions (MIME)][MIME]
- [Does the presence of a Content-ID header in an email MIME mean that the attachment must be embedded?][]
- [RFC 2183: Communicating Presentation Information in Internet Messages: The Content-Disposition Header Field][]
- [RFC 2392: Content-ID and Message-ID Uniform Resource Locators][]
- [The precise format of Content-Id header][]
- [RFC 7103: Advice for Safe Handling of Malformed Messages][]
- [RFC 3986: Uniform Resource Identifier (URI): Generic Syntax][]
- [MDN: encodeURIComponent()][]

[Go]: https://go.dev/
[Amazon Web Services]: https://aws.amazon.com
[API Gateway]: https://aws.amazon.com/api-gateway/
[Lambda]: https://aws.amazon.com/lambda/
[DynamoDB]: https://aws.amazon.com/dynamodb/
[Simple Email Service]: https://aws.amazon.com/ses/
[Simple Notification Service]: https://aws.amazon.com/sns/
[Web Application Firewall]: https://aws.amazon.com/waf/
[CloudFormation]: https://aws.amazon.com/cloudformation/
[AWS Serverless Application Model (SAM)]: https://aws.amazon.com/serverless/sam/
[victoriadrake/simple-subscribe]: https://github.com/victoriadrake/simple-subscribe/
[GnuWin32.Make]: https://winget.run/pkg/GnuWin32/Make
[Amazon Simple Email Service endpoints and quotas]: https://docs.aws.amazon.com/general/latest/gr/ses.html
[AWS Command Line Interface: Quick Setup]: https://docs.aws.amazon.com/cli/latest/userguide/getting-started-quickstart.html
[Email notifications]: https://docs.aws.amazon.com/sns/latest/dg/sns-email-notifications.html
[mbland/ses-forwarder]: https://github.com/mbland/ses-forwarder
[publish an MX record for Amazon SES email receiving]: https://docs.aws.amazon.com/ses/latest/dg/receiving-email-mx-record.html
[account-level suppression list]: https://docs.aws.amazon.com/ses/latest/dg/sending-email-suppression-list.html
[Verifying your domain for Amazon SES email receiving]: https://docs.aws.amazon.com/ses/latest/dg/receiving-email-verification.html
[Receipt Rule Set]: https://docs.aws.amazon.com/ses/latest/dg/receiving-email-receipt-rules-console-walkthrough.html
[Setting up custom domain names for HTTP APIs]: https://docs.aws.amazon.com/apigateway/latest/developerguide/http-api-custom-domain-names.html
[set up an IAM role to allow the API to write CloudWatch logs]: https://repost.aws/knowledge-center/api-gateway-cloudwatch-logs
[AWS::ApiGateway::Account CloudFormation entity]: https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-apigateway-account.html
[Stack Overflow: Configuring logging of AWS API Gateway - Using a SAM template]: https://stackoverflow.com/a/74985768
[test sizes]: https://mike-bland.com/making-software-quality-visible#the-test-pyramid
[Go build constraints]: https://pkg.go.dev/cmd/go#hdr-Build_constraints
[DynamoDB's Time To Live feature]: https://docs.aws.amazon.com/amazondynamodb/latest/developerguide/TTL.html
[SES reputation metrics]: https://docs.aws.amazon.com/ses/latest/dg/monitor-sender-reputation.html
[CAPTCHA]: https://docs.aws.amazon.com/waf/latest/developerguide/waf-captcha-puzzle.html
[AWS WAF Pricing]: https://aws.amazon.com/waf/pricing/
[generate an API key for the CAPTCHA API]: https://docs.aws.amazon.com/waf/latest/developerguide/waf-js-captcha-api-key.html
[&lt;form&gt;]: https://developer.mozilla.org/en-US/docs/Web/HTML/Element/form
[Render the AWS WAF CAPTCHA puzzle]: https://docs.aws.amazon.com/waf/latest/developerguide/waf-js-captcha-api-render.html
[Docker]: https://www.docker.com
[amazon/dynamodb-local]: https://hub.docker.com/r/amazon/dynamodb-local
[Visual Studio Code]: https://code.visualstudio.com
[Go Doc Comments]: https://go.dev/doc/comment
[godoc]: https://pkg.go.dev/golang.org/x/tools/cmd/godoc
[pkgsite]: https://pkg.go.dev/golang.org/x/pkgsite/cmd/pkgsite
[HTTP 303 See Other]: https://developer.mozilla.org/en-US/docs/Web/HTTP/Status/303
[Location HTTP Header]: https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Location
[RFC 5322 Section 3.2.3]: https://datatracker.ietf.org/doc/html/rfc5322#section-3.2.3
[net/mail.ParseAddress]: https://pkg.go.dev/net/mail#ParseAddress
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
[MIME]: https://en.wikipedia.org/wiki/MIME
[Does the presence of a Content-ID header in an email MIME mean that the attachment must be embedded?]: https://serverfault.com/a/489752
[RFC 2183: Communicating Presentation Information in Internet Messages: The Content-Disposition Header Field]: https://www.rfc-editor.org/rfc/rfc2183
[RFC 2392: Content-ID and Message-ID Uniform Resource Locators]: https://www.rfc-editor.org/rfc/rfc2392
[The precise format of Content-Id header]: https://stackoverflow.com/questions/39577386/the-precise-format-of-content-id-header
[RFC 7103: Advice for Safe Handling of Malformed Messages]: https://www.rfc-editor.org/rfc/rfc7103
[RFC 3986: Uniform Resource Identifier (URI): Generic Syntax]: https://www.rfc-editor.org/rfc/rfc3986.html
[MDN: encodeURIComponent()]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/encodeURIComponent
