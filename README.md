[![CI](https://github.com/siutsin/telegram-jung2-bot/actions/workflows/ci.yaml/badge.svg)](https://github.com/siutsin/telegram-jung2-bot/actions/workflows/ci.yaml)
[![codecov](https://codecov.io/gh/siutsin/telegram-jung2-bot/branch/develop/graph/badge.svg?token=0bIxFvEufG)](https://codecov.io/gh/siutsin/telegram-jung2-bot)
[![Known Vulnerabilities](https://snyk.io/test/github/siutsin/telegram-jung2-bot/badge.svg?targetFile=package.json)](https://snyk.io/test/github/siutsin/telegram-jung2-bot?targetFile=package.json)

# telegram-jung2-bot

Add the bot to your group at [@jung2_bot](https://bit.ly/github-jung2bot)

<b>冗員</b>[jung2jyun4] Excess personnel in Cantonese

This bot is created for counting the number of messages per participant in a chat group.

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

## License

See the LICENSE file for more info.
