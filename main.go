package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"

	"github.com/PuerkitoBio/goquery"
)

// Web Crawler by Luke Ryan
// Usage: go run .\main.go <URL>

var crawledLinks = make(map[string]bool)
var maxDepth = 5 // sets maximum child URLs crawled per URL
var mu sync.Mutex

func getUrlHtml(url string) (*goquery.Document, error) {
	// GET url page contents
	res, err := http.Get(url)
	if err != nil {
		log.Println(err)
		return nil, err
	}

	defer res.Body.Close()
	if res.StatusCode != 200 {
		log.Printf("Status Code Error: %d %s For URL: %s\n", res.StatusCode, res.Status, url)
		return nil, fmt.Errorf("Status Code Error: %d %s For URL: %s\n", res.StatusCode, res.Status, url)
	}

	// Load the HTML document
	doc, err := goquery.NewDocumentFromReader(res.Body)
	if err != nil {
		log.Println(err)
		return nil, err
	}
	return doc, nil
}

// crawlWithRecursion - recursively crawls a URL and its children URLs
func crawlWithRecursion(inputUrl string, baseUrl *url.URL, depth int, channel chan bool) {
	defer func() { channel <- true }() // sends to channel after execution is finished, then unblocks

	if depth == 0 {
		return
	}

	// Convert to absolute URL
	urlAbsolute, err := baseUrl.Parse(inputUrl)
	if err != nil {
		log.Println(err)
		return
	}
	urlAbsString := urlAbsolute.String()

	// Print each link not belonging to the start URL domain
	if !strings.Contains(urlAbsString, baseUrl.Host) {
		fmt.Printf("Not Storing External URL: %s\n", urlAbsString)
		return
	}

	mu.Lock()                        // Lock map to prevent race conditions
	if !crawledLinks[urlAbsString] { // Check if URL has been crawled
		// Store the URL in the map
		crawledLinks[urlAbsString] = true
	}
	mu.Unlock() // Unlock map

	// Get HTML doc for URL
	doc, err := getUrlHtml(urlAbsString)
	if err != nil {
		log.Println(err)
		return
	}

	// Search the HTML contents of that page for all <a> tags/links
	var pageUrls []string
	doc.Find("body a").Each(func(_ int, tag *goquery.Selection) {
		url, _ := tag.Attr("href")
		pageUrls = append(pageUrls, url)
	})

	childChannel := make(chan bool)

	for _, url := range pageUrls {
		go crawlWithRecursion(url, baseUrl, depth-1, childChannel)
	}

	for range pageUrls {
		// wait for all the child URLs to finish executing their go routines here
		// we use the channel receive syntax to do this as it blocks until each channel has something to send
		<-childChannel
	}

	return
}

func main() {
	urlArg := os.Args[1] // take URL argument from command line
	var baseUrl, _ = url.Parse(urlArg)

	parentChannel := make(chan bool)
	go crawlWithRecursion(baseUrl.String(), baseUrl, maxDepth, parentChannel)
	<-parentChannel // parentChannel blocks until main caller terminates

	// Convert the map of crawled URLs to a JSON output
	jsonOutput, err := json.Marshal(crawledLinks)
	if err != nil {
		fmt.Printf("Error: %s", err.Error())
	} else {
		fmt.Printf("\n\nCrawled URLs:\n %s", string(jsonOutput))
	}
}
