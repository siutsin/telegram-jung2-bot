#!/usr/bin/env bash

set -e

echo "Deploying for ${GITHUB_REF}..."

# Hide sls deploy output from CI
sls deploy --conceal --force &>/dev/null

echo "Successful deployment for ${GITHUB_REF}"
