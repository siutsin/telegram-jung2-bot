#!/usr/bin/env bash

git flow feature start housekeeping

npm i
npm run lint-fix
doctoc README.md
