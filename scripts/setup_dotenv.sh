#!/usr/bin/env bash

set -e

{
  echo "STAGE=$STAGE"
  echo "REGION=$REGION"
  echo "LOG_LEVEL=$LOG_LEVEL"
} >>.env
