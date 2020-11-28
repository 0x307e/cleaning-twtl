# cleaning-twtl

🚫 Say goodbye to trashy Twitter users

## Requirements

Golang 1.11+ (build from source)

## Usage

### Configration

```sh
cp sample.config.toml
vim config.toml
```

### Block from filter stream

```sh
cleaning-twtl
```

### Block from CSV file

```sh
cleaning-twtl -import [blocked.csv]
```

## Installation

Download flom [Relases](https://github.com/makotia/cleaning-twtl/releases)  
Create config file.  
Run it.

## Web API

Search Words List API
```sh
curl http://localhost:1323/words.csv
```

Blocked User ID List API
```sh
curl http://localhost:1323/blocked.csv
```

## License

MIT

## Author

[@makotia](https://github.com/makotia)
