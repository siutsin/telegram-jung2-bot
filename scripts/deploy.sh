#!/usr/bin/env bash

if [[ "${TRAVIS_BRANCH}" == "develop" ]]; then
   export STAGE=STAGE_DEV
   export REGION=REGION_DEV
   export PROFILE=PROFILE_DEV
   export MESSAGE_TABLE=MESSAGE_TABLE_DEV
   export MESSAGE_TABLE_GSI=MESSAGE_TABLE_GSI_DEV
   export DOMAIN=DOMAIN_DEV
   export LOG_LEVEL=LOG_LEVEL_DEV
elif [[ "${TRAVIS_BRANCH}" == "master" ]]; then
   export STAGE=STAGE_PROD
   export REGION=REGION_PROD
   export PROFILE=PROFILE_PROD
   export MESSAGE_TABLE=MESSAGE_TABLE_PROD
   export MESSAGE_TABLE_GSI=MESSAGE_TABLE_GSI_PROD
   export DOMAIN=DOMAIN_PROD
   export LOG_LEVEL=LOG_LEVEL_PROD
else
   echo "Neither in develop or master branch - ${TRAVIS_BRANCH}"
   exit 1
fi

sls deploy

echo "Successful deployment for ${TRAVIS_BRANCH}"

exit 0
