#!/bin/bash

SUBSCRIBERS_TABLE_NAME="${1}"

if [[ -z "$SUBSCRIBERS_TABLE_NAME" ]]; then
  printf 'Usage: %s [build_table_name]\n' "${BASH_SOURCE[0]}" >&2
  exit 1
fi

aws dynamodb create-table \
  --table-name ${SUBSCRIBERS_TABLE_NAME} \
  --attribute-definitions "AttributeName=email,AttributeType=S" \
  --key-schema "AttributeName=email,KeyType=HASH" \
  --billing-mode "PAY_PER_REQUEST"
