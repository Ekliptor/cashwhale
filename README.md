# CashWahle
A twitter bot to tweet about large amounts of BitcoinCash (BCH) funds
moving on-chain. It is using [BCHD](https://github.com/gcash/bchd) as a full node.

You can follow [@WhaleAlertBch](https://twitter.com/WhaleAlertBch) to see tweets of this bot.

## Building from source

### Requirements
```
Go >= 1.13
```

### Installation
1. Rename the `config.example.yaml` file in the root to `config.yaml` and
customize your settings (BCHD address, Twitter API, ...)

2. In the project root directory, just run:
```
./scripts/build.sh
./bin/cashwhale
```

### Running tests
In the project root directory, just run:
```
./scripts/test.sh
```

## ToDo
- wait for [GoSlp](https://github.com/simpleledgerinc/GoSlp) and BCHD to support it so we can
tweet about SLP transactions

## Contact
Follow me on [Twitter](https://twitter.com/ekliptor) and [Memo](https://memo.cash/profile/1JFKA1CabVyX98qPRAUQBL9NhoTnXZr5Zm).