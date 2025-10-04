package main

import (
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

func main() {
	var dir string
	flag.StringVar(&dir, "d", ".", "Download directory")
	flag.StringVar(&dir, "dir", ".", "Download directory")

	flag.Usage = func() {
		fmt.Printf("Usage: %s [options] <URL>\n", filepath.Base(os.Args[0]))
		flag.PrintDefaults()
	}

	flag.Parse()

	if len(flag.Args()) < 1 {
		flag.Usage()
		return
	}
	baseURL := flag.Args()[0]

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

func getAttr(n *html.Node, key string) string {
	for _, attr := range n.Attr {
		if attr.Key == key {
			return attr.Val
		}
	}
	return ""
}
