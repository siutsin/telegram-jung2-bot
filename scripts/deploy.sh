#!/usr/bin/env bash

if [[ "${TRAVIS_BRANCH}" == "develop" ]]; then
   echo "STAGE=$STAGE_DEV" >> .env
   echo "REGION=$REGION_DEV" >> .env
   echo "MESSAGE_TABLE=$MESSAGE_TABLE_DEV" >> .env
   echo "MESSAGE_TABLE_GSI=$MESSAGE_TABLE_GSI_DEV" >> .env
   echo "DOMAIN=$DOMAIN_DEV" >> .env
   echo "LOG_LEVEL=$LOG_LEVEL_DEV" >> .env
elif [[ "${TRAVIS_BRANCH}" == "master" ]]; then
   echo "STAGE=$STAGE_PROD" >> .env
   echo "REGION=$REGION_PROD" >> .env
   echo "MESSAGE_TABLE=$MESSAGE_TABLE_PROD" >> .env
   echo "MESSAGE_TABLE_GSI=$MESSAGE_TABLE_GSI_PROD" >> .env
   echo "DOMAIN=$DOMAIN_PROD" >> .env
   echo "LOG_LEVEL=$LOG_LEVEL_PROD" >> .env
else
   echo "Neither in develop or master branch - ${TRAVIS_BRANCH}"
   exit 1
fi

sls deploy --conceal &>/dev/null

echo "Successful deployment for ${TRAVIS_BRANCH}"

rm -f .env

exit 0
