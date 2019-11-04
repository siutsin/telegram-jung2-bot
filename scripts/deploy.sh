#!/usr/bin/env bash

set -e

if [[ "${TRAVIS_BRANCH}" == "develop" ]]; then
  echo "Deploying for ${TRAVIS_BRANCH}..."
  {
    echo "STAGE=$STAGE_DEV"
    echo "REGION=$REGION_DEV"
    echo "DOMAIN=$DOMAIN_DEV"
    echo "LOG_LEVEL=$LOG_LEVEL_DEV"
  } >>.env
elif [[ "${TRAVIS_BRANCH}" == "master" ]]; then
  echo "Deploying for ${TRAVIS_BRANCH}..."
  {
    echo "STAGE=$STAGE_PROD"
    echo "REGION=$REGION_PROD"
    echo "DOMAIN=$DOMAIN_PROD"
    echo "LOG_LEVEL=$LOG_LEVEL_PROD"
  } >>.env
else
  echo "Neither in develop nor master branch - ${TRAVIS_BRANCH}"
  exit 1
fi

sls deploy --conceal --force &>/dev/null

echo "Successful deployment for ${TRAVIS_BRANCH}"

rm -f .env

exit 0
