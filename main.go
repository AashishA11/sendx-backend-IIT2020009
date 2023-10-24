package main

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
)

type CrawlSettings struct {
	WorkerCount int
	CrawlSpeed  int
}

var (
	// This map stores the last crawl time for each URL.
	// Mutex to protect concurrent access to lastCrawlTime.
	lastCrawlTimeMutex sync.RWMutex

	// These maps store worker counts and crawl speeds for paying and non-paying customers.
	workerCounts map[string]int
	crawlSpeeds  map[string]int
	// Mutex to protect concurrent access to workerCounts and crawlSpeeds.
	adminDataMutex     sync.RWMutex
	crawlSettings      CrawlSettings
	crawlSettingsMutex sync.RWMutex

	muRateLimit       sync.Mutex
	maxWorkersPaid    = 5
	maxWorkersFree    = 2
	maxPagesPerWorker = 100
	// Track the number of pages crawled per worker per hour
	pageCountPerWorker = make(map[string]int)
	lastCrawlTime      = make(map[string]time.Time)
	CrawlWorkers       = make(map[string][]string)
	shutdown           = make(chan struct{})
)
var cache map[string]string
var cacheMutex sync.RWMutex

func init() {
	cache = make(map[string]string)
}

func getCachedContent(url string) (string, bool) {
	cacheMutex.RLock()
	content, exists := cache[url]
	cacheMutex.RUnlock()
	return content, exists
}

func removeCachedContent(url string) {
	cacheMutex.Lock()
	delete(cache, url)
	cacheMutex.Unlock()
}

func cacheContent(url string, content string) {
	cacheMutex.Lock()
	cache[url] = content
	cacheMutex.Unlock()
}

// Crawler struct to hold the crawling logic.
type Crawler struct {
	PayingCustomers    chan string
	NonPayingCustomers chan string
	wg                 sync.WaitGroup
	shutdown           chan struct{}
}

func NewCrawler() *Crawler {
	return &Crawler{
		PayingCustomers:    make(chan string, 5), // For paying customers (concurrency limit: 5)
		NonPayingCustomers: make(chan string, 2), // For non-paying customers (concurrency limit: 2)
	}
}
func (c *Crawler) CrawlPage(url string, depth int) {
	defer c.wg.Done()
	fmt.Printf("Crawling: %s\n", url)

	if depth <= 0 {
		return
	}

	// Fetch the web page
	resp, err := http.Get(url)
	if err != nil {
		fmt.Printf("Error fetching URL: %v\n", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Printf("Non-200 status code received: %d\n", resp.StatusCode)
		return
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		fmt.Printf("Error parsing the page: %v\n", err)
		return
	}

	var links []string
	var images []string

	// Find and store all links in the page.
	doc.Find("a").Each(func(i int, s *goquery.Selection) {
		link, exists := s.Attr("href")
		if exists {
			links = append(links, link)
		}
	})

	// Find and store all image sources in the page.
	doc.Find("img").Each(func(i int, s *goquery.Selection) {
		src, exists := s.Attr("src")
		if exists {
			images = append(images, src)
		}
	})

	// Print the links and images
	fmt.Printf("Links on the page:\n%s\n", strings.Join(links, "\n"))
	fmt.Printf("Images on the page:\n%s\n", strings.Join(images, "\n"))

	// Recursively crawl the links on the page (up to a specified depth)
	if depth > 1 {
		for _, link := range links {
			c.wg.Add(1)
			go c.CrawlPage(link, depth-1)
		}
	}
}

// StartCrawl starts the crawling process for paying and non-paying customers.
func (c *Crawler) StartCrawl() {
	for {
		select {
		case url, ok := <-c.PayingCustomers:
			if !ok {
				return
			}
			c.wg.Add(1)
			go c.CrawlPage(url, 3)
		case url, ok := <-c.NonPayingCustomers:
			if !ok {
				return
			}
			c.wg.Add(1)
			go c.CrawlPage(url, 3)
		case <-c.shutdown:
			return
		}
	}
}

func main() {
	// Create an instance of the Crawler
	crawler := NewCrawler()

	// Start the crawler
	go crawler.StartCrawl()

	// Handle HTTP requests
	http.HandleFunc("/crawl", func(w http.ResponseWriter, r *http.Request) {
		url := r.FormValue("url")
		customerType := r.FormValue("customerType")

		// Determine whether the request is for paid or free customers
		if customerType == "paid" {
			// Send the URL to the paying customers channel
			crawler.PayingCustomers <- url
		} else if customerType == "free" {
			// Send the URL to the non-paying customers channel
			crawler.NonPayingCustomers <- url
		} else {
			// Handle invalid customer types or errors
			http.Error(w, "Invalid customer type", http.StatusBadRequest)
			return
		}

		// Respond to the user's request
		fmt.Fprintf(w, "Crawling URL: %s\n", url)
	})

	// Set up other HTTP handlers
	http.HandleFunc("/admin/set-workers", setWorkersHandler)
	http.HandleFunc("/admin/set-crawl-speed", setCrawlSpeedHandler)
	http.HandleFunc("/admin.html", adminPageHandler)
	http.Handle("/", http.FileServer(http.Dir("."))) // Serves static files

	// Start the HTTP server
	http.ListenAndServe(":8080", nil)

}

func crawlHandler(w http.ResponseWriter, r *http.Request) {
	url := r.FormValue("url")
	customerType := r.FormValue("customerType")

	// Determine the number of workers and crawl speed based on customer type.

	// Function to simulate crawling and content retrieval.
	rateLimited, erra := checkCrawlRateLimit(customerType, url, r.RemoteAddr)
	if rateLimited {
		http.Error(w, erra, http.StatusTooManyRequests)
		return
	}

	limitReached, limitMessage := checkCrawlRateLimit(customerType, url, "someWorkerKey")

	if limitReached {
		// Handle the rate limit error
		fmt.Fprintf(w, "Error: %s", limitMessage)
		return
	}

	crawlAndRetrieveContent := func(url string) (string, error) {
		resp, err := http.Get(url)
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return "", fmt.Errorf("Non-200 status code received")
		}

		doc, err := goquery.NewDocumentFromReader(resp.Body)
		if err != nil {
			return "", err
		}

		var links []string
		var images []string

		// Find and store all links in the page.
		doc.Find("a").Each(func(i int, s *goquery.Selection) {
			link, exists := s.Attr("href")
			if exists {
				links = append(links, link)
			}
		})

		// Find and store all image sources in the page.
		doc.Find("img").Each(func(i int, s *goquery.Selection) {
			src, exists := s.Attr("src")
			if exists {
				images = append(images, src)
			}
		})

		result := "Crawling URL: " + url + "\n"
		result += "Links on the page:\n"
		result += strings.Join(links, "\n") + "\n\n"
		result += "Images on the page:\n"
		result += strings.Join(images, "\n")

		return result, nil
	}

	var crawlResult string
	var err error
	// Retry crawling if the initial attempt fails.
	for retry := 0; retry < 3; retry++ {
		crawlResult, err = crawlAndRetrieveContent(url)
		if err == nil {
			break
		}
		// Add a delay or handle retries according to your requirements.
		time.Sleep(2 * time.Second)
	}

	if err != nil {
		fmt.Fprintf(w, "Error: Failed to crawl URL: %s")
		return
	}

	// Update the last crawl time and cache the content.
	lastCrawlTimeMutex.Lock()
	lastCrawlTime[url] = time.Now()
	lastCrawlTimeMutex.Unlock()
	cacheContent(url, crawlResult)

	fmt.Fprintf(w, crawlResult)
}

func crawl(url string) (string, bool) {
	resp, err := http.Get(url)
	if err != nil {
		return "Error: Unable to fetch the URL.", false
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "Error: Non-200 status code received.", false
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return "Error: Unable to parse the page.", false
	}

	var links []string
	var images []string

	doc.Find("a").Each(func(i int, s *goquery.Selection) {
		link, exists := s.Attr("href")
		if exists {
			links = append(links, link)
		}
	})

	doc.Find("img").Each(func(i int, s *goquery.Selection) {
		src, exists := s.Attr("src")
		if exists {
			images = append(images, src)
		}
	})

	result := "Crawling URL: " + url + "\n"
	result += "Links on the page:\n"
	result += strings.Join(links, "\n") + "\n\n"
	result += "Images on the page:\n"
	result += strings.Join(images, "\n")

	return result, true
}

// Function to check the crawl rate limit
func checkCrawlRateLimit(customerType, url, workerKey string) (bool, string) {
	muRateLimit.Lock()
	defer muRateLimit.Unlock()

	if workerCount := len(CrawlWorkers[customerType]); workerCount >= maxWorkersPaid && customerType == "Paid" {
		return true, "Hourly worker limit for paid customers reached"
	}
	if workerCount := len(CrawlWorkers[customerType]); workerCount >= maxWorkersFree && customerType == "Free" {
		return true, "Hourly worker limit for free customers reached"
	}

	// Check the crawl rate for the given URL and worker
	key := customerType + url
	lastTime, exists := lastCrawlTime[key]
	if exists && time.Since(lastTime).Minutes() < 60 {
		pageCount := pageCountPerWorker[workerKey]
		if pageCount >= maxPagesPerWorker {
			return true, "Hourly crawl limit for the worker exceeded"
		}
		pageCountPerWorker[workerKey]++
	} else {
		pageCountPerWorker[workerKey] = 1
		lastCrawlTime[key] = time.Now()
	}

	return false, ""
}

func setWorkersHandler(w http.ResponseWriter, r *http.Request) {
	customerType := r.PostFormValue("customerType")
	workerCountStr := r.PostFormValue("workerCount")

	if workerCountStr == "" || customerType == "" {
		http.Error(w, "Both customer type and worker count are required.", http.StatusBadRequest)
		return
	}

	workerCount, err := strconv.Atoi(workerCountStr)
	if err != nil {
		http.Error(w, "Invalid worker count.", http.StatusBadRequest)
		return
	}

	muRateLimit.Lock()
	defer muRateLimit.Unlock()

	if customerType == "paid" {
		maxWorkersPaid = workerCount
	} else if customerType == "free" {
		maxWorkersFree = workerCount
	} else {
		http.Error(w, "Invalid customer type.", http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusNoContent)

	fmt.Fprintf(w, "Worker count for %s set to %d", customerType, workerCount)
}

func setCrawlSpeedHandler(w http.ResponseWriter, r *http.Request) {
	crawlSpeedStr := r.PostFormValue("crawlSpeed")

	if crawlSpeedStr == "" {
		http.Error(w, "Crawling speed is required.", http.StatusBadRequest)
		return
	}

	crawlSpeed, err := strconv.Atoi(crawlSpeedStr)
	if err != nil {
		http.Error(w, "Invalid crawling speed.", http.StatusBadRequest)
		return
	}

	muRateLimit.Lock()
	defer muRateLimit.Unlock()

	maxPagesPerWorker = crawlSpeed

	w.WriteHeader(http.StatusNoContent)
	// fmt.Fprintf(w, "Crawl speed for %s set to %d pages/hour per worker", customerType, speed)
}

// Admin Control API to set the number of workers and crawl rate per hour per worker
func setWorkerAndCrawlRate(w http.ResponseWriter, r *http.Request) {
	workerCountStr := r.URL.Query().Get("workers")
	crawlRateStr := r.URL.Query().Get("rate")

	if workerCountStr == "" && crawlRateStr == "" {
		http.Error(w, "No parameters provided", http.StatusBadRequest)
		return
	}

	var workerCount, crawlRate int
	var err error

	if workerCountStr != "" {
		workerCount, err = strconv.Atoi(workerCountStr)
		if err != nil {
			http.Error(w, "Invalid 'workers' value", http.StatusBadRequest)
			return
		}
	}

	if crawlRateStr != "" {
		crawlRate, err = strconv.Atoi(crawlRateStr)
		if err != nil {
			http.Error(w, "Invalid 'rate' value", http.StatusBadRequest)
			return
		}
	}

	muRateLimit.Lock()
	defer muRateLimit.Unlock()

	if workerCount > 0 {
		if r.URL.Query().Get("type") == "Paid" {
			maxWorkersPaid = workerCount
		} else if r.URL.Query().Get("type") == "Free" {
			maxWorkersFree = workerCount
		}
	}

	if crawlRate > 0 {
		maxPagesPerWorker = crawlRate
	}

	// Optionally, you can store these values in a configuration file or database for persistence.
}

func adminPageHandler(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "admin.html")
}
