[![serverless](assets/badges/serverless-v3.svg)](https://www.serverless.com)
[![license](https://img.shields.io/badge/license-MIT-blue.svg)](https://siutsin.mit-license.org/)
[![JavaScript Style Guide](https://img.shields.io/badge/code_style-standard-brightgreen.svg)](https://standardjs.com)
[![mergify](https://img.shields.io/endpoint.svg?url=https://gh.mergify.io/badges/siutsin/telegram-jung2-bot&style=flat)](https://mergify.io)
<br>
[![dependency](https://david-dm.org/siutsin/telegram-jung2-bot.svg)](https://david-dm.org/siutsin/telegram-jung2-bot)
[![devDependency Status](https://david-dm.org/siutsin/telegram-jung2-bot/dev-status.svg)](https://david-dm.org/siutsin/telegram-jung2-bot#info=devDependencies)
[![Build Status](https://travis-ci.com/siutsin/telegram-jung2-bot.svg?branch=master)](https://travis-ci.com/siutsin/telegram-jung2-bot)
[![Coverage Status](https://coveralls.io/repos/github/siutsin/telegram-jung2-bot/badge.svg)](https://coveralls.io/github/siutsin/telegram-jung2-bot)
[![FOSSA Status](https://app.fossa.io/api/projects/git%2Bgithub.com%2Fsiutsin%2Ftelegram-jung2-bot.svg?type=shield)](https://app.fossa.io/projects/git%2Bgithub.com%2Fsiutsin%2Ftelegram-jung2-bot?ref=badge_shield)
[![Known Vulnerabilities](https://snyk.io/test/github/siutsin/telegram-jung2-bot/badge.svg?targetFile=package.json)](https://snyk.io/test/github/siutsin/telegram-jung2-bot?targetFile=package.json)

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
  - [Test API and Database locally](#test-api-and-database-locally)
- [Sponsor](#sponsor)
- [Author](#author)
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
|`/enablealljung`|Enable `/alljung` command|
|`/disablealljung`|Disable `/alljung` command|

## Development

### Test API and Database locally

```bash
$ npm run offline
```

## Sponsor

[RisingStack](https://trace.risingstack.com?utm_source=github&utm_medium=sponsored&utm_content=siutsin/telegram-jung2-bot)

## Author

[@Simon__Li](https://bit.ly/github-twitter)

## License

`telegram-jung2-bot` is available under the [MIT license](https://siutsin.mit-license.org). See the LICENSE file for more info.


[![FOSSA Status](https://app.fossa.io/api/projects/git%2Bgithub.com%2Fsiutsin%2Ftelegram-jung2-bot.svg?type=large)](https://app.fossa.io/projects/git%2Bgithub.com%2Fsiutsin%2Ftelegram-jung2-bot?ref=badge_large)
