[![serverless](http://public.serverless.com/badges/v3.svg)](https://www.serverless.com)
[![license](https://img.shields.io/badge/license-MIT-blue.svg)](https://img.shields.io/badge/license-MIT-blue.svg)
[![dependency](https://david-dm.org/siutsin/telegram-jung2-bot.svg)](https://david-dm.org/siutsin/telegram-jung2-bot.svg)
[![devDependency Status](https://david-dm.org/siutsin/telegram-jung2-bot/dev-status.svg)](https://david-dm.org/siutsin/telegram-jung2-bot#info=devDependencies)
[![Build Status](https://travis-ci.org/siutsin/telegram-jung2-bot.svg?branch=master)](https://travis-ci.org/siutsin/telegram-jung2-bot)
[![Coverage Status](https://coveralls.io/repos/github/siutsin/telegram-jung2-bot/badge.svg)](https://coveralls.io/github/siutsin/telegram-jung2-bot)

# telegram-jung2-bot

Add the bot to your group at [@jung2_bot](https://bit.ly/github-jung2bot)

<b>冗員</b>[jung2jyun4] Excess personnel in Cantonese

This bot is created for counting the number of messages per participant in a chat group.

<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->
**Table of Contents**

- [Setup](#setup)
  - [AWS Credential](#aws-credential)
  - [Telegram API Token](#telegram-api-token)
  - [Create `.env` files](#create-env-files)
  - [Deploy! 🚀](#deploy-)
- [Usage](#usage)
  - [Admin Only](#admin-only)
- [Development](#development)
  - [Test API and DB locally](#test-api-and-db-locally)
- [Sponsor](#sponsor)
- [Author](#author)
- [Code Style](#code-style)
- [License](#license)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

## Setup

### AWS Credential

Refer to [AWS Documentation](https://docs.aws.amazon.com/cli/latest/userguide/cli-configure-files.html).

### Telegram API Token

Serverless will retrieve the Telegram API Token via [SSM](https://docs.aws.amazon.com/systems-manager/latest/userguide/systems-manager-paramstore.html) from a SecureString.

Naming convention - `{service}-{stage}-telegram-api-token`. E.g.:

- `jung2bot-dev-telegram-api-token`

### Create `.env` files

Copy `.env.example` and rename the file to `.env.{stage}`. E.g.:

- `.env.development`
- `.env.production`

Load orders are defined at `serverless-dotenv-plugin`'s [doc](https://github.com/colynb/serverless-dotenv-plugin#automatic-env-file-name-resolution).

### Deploy! 🚀

```bash
$ sls deploy
```

## Usage

|command|info|
|---|---|
|`/topten`|Show the percentage of top ten participants for the past seven days|
|`/topdiver`|Show the percentage of top ten divers for the past seven days (Requires at least one message from the user to be counted)|
|`/alljung`|Show the percentage of all participants for the past seven days|
|`/junghelp`|Show help message|

### Admin Only
|command|info|
|---|---|
|`/enablealljung`|Enable /alljung command|
|`/disablealljung`|Disable /alljung command|

## Development

### Test API and DB locally

```bash
$ npm run offline
```

## Sponsor

[RisingStack](https://trace.risingstack.com?utm_source=github&utm_medium=sponsored&utm_content=siutsin/telegram-jung2-bot)

## Author

[@Simon__Li](https://bit.ly/github-twitter)

## Code Style

[![JavaScript Style Guide](https://cdn.rawgit.com/standard/standard/master/badge.svg)](https://github.com/standard/standard)

## License

`telegram-jung2-bot` is available under the [MIT license](https://siutsin.mit-license.org). See the LICENSE file for more info.
