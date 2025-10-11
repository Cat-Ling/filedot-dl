# filedot-dl

A command-line tool for downloading files from filedot.to

## 📦 Dependencies

-   [Go](https://golang.org/)
-   [aria2c](https://aria2.github.io/)

## ⚙️ Installation

1.  Install Go and aria2c.
2.  Build the script:
    ```bash
    go build -o filedot-dl main.go
    ```

## 🚀 Usage

```bash
./filedot-dl [options] <URL>
```

### Options

-   `-d`, `--dir`: Specify the download directory. Defaults to the current directory.
-   `-list`: Specify a file containing a list of URLs to download, one per line.
-   `-N`, `-concurrent`: Number of concurrent file downloads. (default 3) (for link lists and folders)

### 📂 Custom Directory

```bash
./filedot-dl -d /path/to/downloads https://filedot.to/abcdefgh
```

### 🔗 Link List

```bash
./filedot-dl -list /path/to/links.txt
```

### 🧩 Download Multiple Files Simultaneously

```bash
./filedot-dl -N <n> https://filedot.to/abcdefgh
```


## Disclaimer

This script is for educational purposes only. Please do not use it for any illegal activities.
