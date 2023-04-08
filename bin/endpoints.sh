#!/usr/bin/env bash
#
# From:
# https://catalog.workshops.aws/complete-aws-sam/en-US/module-4-cicd/module-4-cicd-gh/60-inspect#inspect-the-devprod-stages

function get_endpoint() {
  local stack_name="$1"

  aws cloudformation describe-stacks --stack-name "$stack_name" |
    jq -r '.Stacks[].Outputs[].OutputValue | select(startswith("https://"))'
}

export DEV_ENDPOINT="$(get_endpoint elistman-dev)"
export PROD_ENDPOINT="$(get_endpoint elistman-prod)"

echo "Dev endpoint: $DEV_ENDPOINT"
echo "Prod endpoint: $PROD_ENDPOINT"
