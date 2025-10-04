# filedot-dl

This is a command-line tool for downloading files from filedot.to. It automates the process of navigating the website, solving the captcha, and downloading the file.

## Features

-   Downloads files from filedot.to
-   Automatically solves the captcha
-   Supports specifying the download directory
-   Uses `aria2c` for fast and reliable downloads

## Dependencies

-   [Go](https://golang.org/)
-   [aria2c](https://aria2.github.io/)

## Installation

1.  Install Go and aria2c.
2.  Build the script:
    ```bash
    go build -o filedot-dl main.go
    ```

## Usage

```bash
./filedot-dl [options] <URL>
```

### Options

-   `-d`, `--dir`: Specify the download directory. Defaults to the current directory.

### Example

```bash
./filedot-dl -d /path/to/downloads https://filedot.to/abcdefgh
```

## Disclaimer

This script is for educational purposes only. Please do not use it for any illegal activities.
