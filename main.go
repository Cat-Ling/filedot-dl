package main

import (
	"bufio"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/html"
)

const (
	userAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/109.0.0.0 Safari/537.36"
)

// CaptchaDigit represents a single digit of the captcha with its position.
type CaptchaDigit struct {
	Text        string
	PaddingLeft float64
}


func downloadFile(baseURL, dir string) {
	
	fmt.Println("Fetching file page...")
	jar, _ := cookiejar.New(nil)
	client := &http.Client{
		Jar: jar,
	}

	re := regexp.MustCompile(`filedot.to/([a-zA-Z0-9]+)`)
	matches := re.FindStringSubmatch(baseURL)
	if len(matches) < 2 {
		fmt.Println("Could not extract file ID from URL.")
		return
	}
	fileID := matches[1]

	req, err := http.NewRequest("GET", baseURL, nil)
	if err != nil {
		fmt.Printf("Error creating GET request: %v\n", err)
		return
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("Error fetching initial page: %v\n", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Printf("Initial page request failed with status: %s\n", resp.Status)
		return
	}

	initialFormData := make(url.Values)
	initialFormData.Set("op", "download1")
	initialFormData.Set("id", fileID)
	initialFormData.Set("referer", "https://www.google.com/")
	initialFormData.Set("method_free", "Free Download")

	fmt.Println("Solving captcha...")

	req, err = http.NewRequest("POST", baseURL, strings.NewReader(initialFormData.Encode()))
	if err != nil {
		fmt.Printf("Error creating initial POST request: %v\n", err)
		return
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Referer", baseURL)

	resp, err = client.Do(req)
	if err != nil {
		fmt.Printf("Error submitting initial form: %v\n", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Printf("Initial form submission failed with status: %s\n", resp.Status)
		return
	}

	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("Error reading response body after initial POST: %v\n", err)
		return
	}
	bodyString := string(bodyBytes)

	doc, err := html.Parse(strings.NewReader(bodyString))
	if err != nil {
		fmt.Printf("Error parsing HTML after initial POST: %v\n", err)
		return
	}

	captchaFormData := make(url.Values)
	var captchaSolution string

	var parseCaptchaPage func(*html.Node)
	parseCaptchaPage = func(n *html.Node) {
		if n.Type == html.ElementNode {
			if n.Data == "input" {
				name := getAttr(n, "name")
				value := getAttr(n, "value")
				if name != "" {
					captchaFormData.Set(name, value)
				}
			}

			if n.Data == "div" && getAttr(n, "style") == "width:80px;height:26px;font:bold 13px Arial;background:#ccc;text-align:left;direction:ltr;" {
				var digits []CaptchaDigit
				for c := n.FirstChild; c != nil; c = c.NextSibling {
					if c.Type == html.ElementNode && c.Data == "span" {
						text := strings.TrimSpace(c.FirstChild.Data)
						style := getAttr(c, "style")
						re := regexp.MustCompile(`padding-left:\s*(\d+)\s*px;`)
						matches := re.FindStringSubmatch(style)
						if len(matches) > 1 {
							paddingLeft, _ := strconv.ParseFloat(matches[1], 64)
							digits = append(digits, CaptchaDigit{Text: text, PaddingLeft: paddingLeft})
						}
					}
				}
				sort.Slice(digits, func(i, j int) bool {
					return digits[i].PaddingLeft < digits[j].PaddingLeft
				})
				for _, d := range digits {
					captchaSolution += d.Text
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			parseCaptchaPage(c)
		}
	}
	parseCaptchaPage(doc)

	if captchaSolution != "" {
		captchaFormData.Set("code", captchaSolution)
	}

	time.Sleep(6 * time.Second)

	req, err = http.NewRequest("POST", baseURL, strings.NewReader(captchaFormData.Encode()))
	if err != nil {
		fmt.Printf("Error creating POST request: %v\n", err)
		return
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Referer", baseURL)

	resp, err = client.Do(req)
	if err != nil {
		fmt.Printf("Error submitting form: %v\n", err)
		return
	}
	defer resp.Body.Close()

	bodyBytes, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("Error reading response body: %v\n", err)
		return
	}
	bodyString = string(bodyBytes)

	doc, err = html.Parse(strings.NewReader(bodyString))
	if err != nil {
		fmt.Printf("Error parsing HTML after form submission: %v\n", err)
		return
	}

	var finalDownloadLink string
	var findDownloadLink func(*html.Node)
	findDownloadLink = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "a" {
			href := getAttr(n, "href")
			if strings.Contains(href, "fs") {
				finalDownloadLink = href
				return
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			findDownloadLink(c)
		}
	}
	findDownloadLink(doc)

	var findFileName func(*html.Node) string
	findFileName = func(n *html.Node) string {
		if n.Type == html.ElementNode && n.Data == "nobr" {
			return n.FirstChild.Data
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if result := findFileName(c); result != "" {
				return result
			}
		}
		return ""
	}
	fileName := findFileName(doc)
	if fileName != "" {
		fmt.Printf("Downloading: %s\n", fileName)
	}

	if finalDownloadLink == "" {
		fmt.Println("Could not find the final download link.")
		return
	}

	fmt.Println("Downloading file...")

	err = os.MkdirAll(dir, 0755)
	if err != nil {
		fmt.Printf("Error creating directory: %v\n", err)
		return
	}

	cookies := client.Jar.Cookies(req.URL)
	cookieHeader := ""
	for _, cookie := range cookies {
		cookieHeader += cookie.Name + "=" + cookie.Value + "; "
	}
	cookieHeader = strings.TrimSuffix(cookieHeader, "; ")

	aria2cArgs := []string{
		finalDownloadLink,
		"-c",
		"--header=User-Agent: " + userAgent,
		"--check-certificate=false",
		"--dir=" + dir,
	}

	if cookieHeader != "" {
		aria2cArgs = append(aria2cArgs, "--header=Cookie: "+cookieHeader)
	}

	aria2cArgs = append(aria2cArgs, "--header=Referer: "+baseURL)

	cmd := exec.Command("aria2c", aria2cArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		fmt.Printf("Error running aria2c: %v\n", err)
		return
	}
}

func downloadFolder(folderURL, downloadDir string, numConcurrentDownloads int) {
	fmt.Printf("Fetching folder page: %s\n", folderURL)

	jar, _ := cookiejar.New(nil)
	client := &http.Client{
		Jar: jar,
	}

	req, err := http.NewRequest("GET", folderURL, nil)
	if err != nil {
		fmt.Printf("Error creating GET request for folder: %v\n", err)
		return
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("Error fetching folder page: %v\n", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Printf("Folder page request failed with status: %s\n", resp.Status)
		return
	}

	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("Error reading folder page response body: %v\n", err)
		return
	}
	bodyString := string(bodyBytes)

	doc, err := html.Parse(strings.NewReader(bodyString))
	if err != nil {
		fmt.Printf("Error parsing folder page HTML: %v\n", err)
		return
	}

	var folderName string
	var findFolderName func(*html.Node)
	findFolderName = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "h1" {
			var h1Content string
			var collectText func(*html.Node)
			collectText = func(n *html.Node) {
				if n.Type == html.TextNode {
					h1Content += n.Data
				}
				for c := n.FirstChild; c != nil; c = c.NextSibling {
					collectText(c)
				}
			}
			collectText(n)
			folderName = strings.TrimSpace(h1Content)
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			findFolderName(c)
		}
	}
	findFolderName(doc)

	if folderName == "" {
		fmt.Println("Could not determine folder name. Using default 'downloads'.")
		folderName = "downloads"
	}

	fmt.Printf("Downloading files from folder: %s\n", folderName)

	// Create a subdirectory for the folder
	folderPath := filepath.Join(downloadDir, folderName)
	err = os.MkdirAll(folderPath, 0755)
	if err != nil {
		fmt.Printf("Error creating folder directory %s: %v\n", folderPath, err)
		return
	}

	var fileLinks []string
	var inTable bool
	var findFileLinks func(*html.Node)
	findFileLinks = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "table" {
			inTable = true
		}
		if inTable && n.Type == html.ElementNode && n.Data == "a" {
			href := getAttr(n, "href")
			if strings.Contains(href, "filedot.to/") && !strings.Contains(href, "/folder/") {
				fileLinks = append(fileLinks, href)
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			findFileLinks(c)
		}
		if n.Type == html.ElementNode && n.Data == "table" {
			inTable = false
		}
	}
	findFileLinks(doc)

	if len(fileLinks) == 0 {
		fmt.Println("No file links found in the folder.")
		return
	}

	fmt.Printf("Found %d files in the folder. Starting download with %d concurrent downloads...\n", len(fileLinks), numConcurrentDownloads)

	semaphore := make(chan struct{}, numConcurrentDownloads)
	var wg sync.WaitGroup

	for _, link := range fileLinks {
		semaphore <- struct{}{}
		wg.Add(1)
		go func(link string) {
			defer wg.Done()
			time.Sleep(2 * time.Second)
			downloadFile(link, folderPath)
			<-semaphore
		}(link)
	}
	wg.Wait()
}



func main() {
	var dir string
	var numConcurrentDownloads int
	var listFile string
	flag.StringVar(&dir, "d", ".", "Download directory")
	flag.StringVar(&dir, "dir", ".", "Download directory")
	flag.IntVar(&numConcurrentDownloads, "N", 3, "Number of concurrent downloads")
	flag.IntVar(&numConcurrentDownloads, "concurrent", 3, "Number of concurrent downloads (alias for -N)")
	flag.StringVar(&listFile, "list", "", "File containing a list of URLs to download, one per line")

	flag.Usage = func() {
		fmt.Printf("Usage: %s [options] <URL>\n", filepath.Base(os.Args[0]))
		flag.PrintDefaults()
	}

	flag.Parse()

	if listFile != "" {
		downloadFromList(listFile, dir, numConcurrentDownloads)
		return
	}

	if len(flag.Args()) < 1 {
		flag.Usage()
		return
	}
	baseURL := flag.Args()[0]

	if isFolderURL(baseURL) {
		downloadFolder(baseURL, dir, numConcurrentDownloads)
		return
	}

	fmt.Println("Fetching file page...")

	jar, _ := cookiejar.New(nil)
	client := &http.Client{
		Jar: jar,
	}

	re := regexp.MustCompile(`filedot.to/([a-zA-Z0-9]+)`)
	matches := re.FindStringSubmatch(baseURL)
	if len(matches) < 2 {
		fmt.Println("Could not extract file ID from URL.")
		return
	}
	fileID := matches[1]

	req, err := http.NewRequest("GET", baseURL, nil)
	if err != nil {
		fmt.Printf("Error creating GET request: %v\n", err)
		return
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("Error fetching initial page: %v\n", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Printf("Initial page request failed with status: %s\n", resp.Status)
		return
	}

	initialFormData := make(url.Values)
	initialFormData.Set("op", "download1")
	initialFormData.Set("id", fileID)
	initialFormData.Set("referer", "https://www.google.com/")
	initialFormData.Set("method_free", "Free Download")

	fmt.Println("Solving captcha...")

	req, err = http.NewRequest("POST", baseURL, strings.NewReader(initialFormData.Encode()))
	if err != nil {
		fmt.Printf("Error creating initial POST request: %v\n", err)
		return
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Referer", baseURL)

	resp, err = client.Do(req)
	if err != nil {
		fmt.Printf("Error submitting initial form: %v\n", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Printf("Initial form submission failed with status: %s\n", resp.Status)
		return
	}

	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("Error reading response body after initial POST: %v\n", err)
		return
	}
	bodyString := string(bodyBytes)

	doc, err := html.Parse(strings.NewReader(bodyString))
	if err != nil {
		fmt.Printf("Error parsing HTML after initial POST: %v\n", err)
		return
	}

	captchaFormData := make(url.Values)
	var captchaSolution string

	var parseCaptchaPage func(*html.Node)
	parseCaptchaPage = func(n *html.Node) {
		if n.Type == html.ElementNode {
			if n.Data == "input" {
				name := getAttr(n, "name")
				value := getAttr(n, "value")
				if name != "" {
					captchaFormData.Set(name, value)
				}
			}

			if n.Data == "div" && getAttr(n, "style") == "width:80px;height:26px;font:bold 13px Arial;background:#ccc;text-align:left;direction:ltr;" {
				var digits []CaptchaDigit
				for c := n.FirstChild; c != nil; c = c.NextSibling {
					if c.Type == html.ElementNode && c.Data == "span" {
						text := strings.TrimSpace(c.FirstChild.Data)
						style := getAttr(c, "style")
						re := regexp.MustCompile(`padding-left:\s*(\d+)\s*px;`)
						matches := re.FindStringSubmatch(style)
						if len(matches) > 1 {
							paddingLeft, _ := strconv.ParseFloat(matches[1], 64)
							digits = append(digits, CaptchaDigit{Text: text, PaddingLeft: paddingLeft})
						}
					}
				}
				sort.Slice(digits, func(i, j int) bool {
					return digits[i].PaddingLeft < digits[j].PaddingLeft
				})
				for _, d := range digits {
					captchaSolution += d.Text
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			parseCaptchaPage(c)
		}
	}
	parseCaptchaPage(doc)

	if captchaSolution != "" {
		captchaFormData.Set("code", captchaSolution)
	}

	time.Sleep(6 * time.Second)

	req, err = http.NewRequest("POST", baseURL, strings.NewReader(captchaFormData.Encode()))
	if err != nil {
		fmt.Printf("Error creating POST request: %v\n", err)
		return
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Referer", baseURL)

	resp, err = client.Do(req)
	if err != nil {
		fmt.Printf("Error submitting form: %v\n", err)
		return
	}
	defer resp.Body.Close()

	bodyBytes, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("Error reading response body: %v\n", err)
		return
	}
	bodyString = string(bodyBytes)

	doc, err = html.Parse(strings.NewReader(bodyString))
	if err != nil {
		fmt.Printf("Error parsing HTML after form submission: %v\n", err)
		return
	}

	var finalDownloadLink string
	var findDownloadLink func(*html.Node)
	findDownloadLink = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "a" {
			href := getAttr(n, "href")
			if strings.Contains(href, "fs") {
				finalDownloadLink = href
				return
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			findDownloadLink(c)
		}
	}
	findDownloadLink(doc)

	var findFileName func(*html.Node) string
	findFileName = func(n *html.Node) string {
		if n.Type == html.ElementNode && n.Data == "nobr" {
			return n.FirstChild.Data
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if result := findFileName(c); result != "" {
				return result
			}
		}
		return ""
	}
	fileName := findFileName(doc)
	if fileName != "" {
		fmt.Printf("Downloading: %s\n", fileName)
	}

	if finalDownloadLink == "" {
		fmt.Println("Could not find the final download link.")
		return
	}

	fmt.Println("Downloading file...")

	err = os.MkdirAll(dir, 0755)
	if err != nil {
		fmt.Printf("Error creating directory: %v\n", err)
		return
	}

	cookies := client.Jar.Cookies(req.URL)
	cookieHeader := ""
	for _, cookie := range cookies {
		cookieHeader += cookie.Name + "=" + cookie.Value + "; "
	}
	cookieHeader = strings.TrimSuffix(cookieHeader, "; ")

	aria2cArgs := []string{
		finalDownloadLink,
		"-c",
		"--header=User-Agent: " + userAgent,
		"--check-certificate=false",
		"--dir=" + dir,
	}

	if cookieHeader != "" {
		aria2cArgs = append(aria2cArgs, "--header=Cookie: "+cookieHeader)
	}

	aria2cArgs = append(aria2cArgs, "--header=Referer: "+baseURL)

	cmd := exec.Command("aria2c", aria2cArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		fmt.Printf("Error running aria2c: %v\n", err)
		return
	}
}

func isFolderURL(urlStr string) bool {
	return strings.Contains(urlStr, "/folder/")
}

func getAttr(n *html.Node, key string) string {
	for _, attr := range n.Attr {
		if attr.Key == key {
			return attr.Val
		}
	}
	return ""
}

func downloadFromList(listFile, downloadDir string, numConcurrentDownloads int) {
	file, err := os.Open(listFile)
	if err != nil {
		fmt.Printf("Error opening list file: %v\n", err)
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		link := scanner.Text()
		if isFolderURL(link) {
			downloadFolder(link, downloadDir, numConcurrentDownloads)
		} else {
			downloadFile(link, downloadDir)
		}
	}

	if err := scanner.Err(); err != nil {
		fmt.Printf("Error reading list file: %v\n", err)
	}
}
