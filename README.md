# sendx-backend-IIT2020009

Welcome to the Sendex Backend project, IIT2020009. This backend server is designed to handle web crawling requests and prioritize paying customers while providing various features and controls.

## Table of Contents
- [Features](#features)
- [Getting Started](#getting-started)
  - [Prerequisites](#prerequisites)
  - [Installation](#installation)
- [Usage](#usage)
- [API Endpoints](#api-endpoints)
  - [Crawl](#Web-Crawler)
  - [Admin Control Panel](#admin-control-panel)
  - [Set Workers](#Set-the-number-of-workers (Admin API))
  - [Set Crawl Speed](#Set-the-crawling-speed (Admin API))
- [Crawling Workflow](#crawling-workflow)
- [Rate Limiting](#rate-limiting)
- [Installation](#Installing)
## Features

### Core Features

1. **Single HTML Page:** A user-friendly web page provides a search bar where users can enter the URL they want to crawl and click the "Crawl" button.

2. **Caching:** The server checks if the URL has been crawled in the last 60 minutes. If the data is available in the cache, it is returned; otherwise, it performs real-time crawling.

3. **Retry Mechanism:** Considering that web pages may not always be available, the system incorporates a retry mechanism to ensure successful crawling.

4. **Prioritization:** The application caters to both paying and non-paying customers. Paying customers receive priority during crawling. This differentiation is achieved by a query parameter passed from the frontend API.

### Advanced Features

1. **Concurrent Crawling:** Multiple workers are available to crawl pages concurrently, ensuring maximum throughput. The system assumes 5 workers for paying customers and 2 for non-paying customers.

2. **Rate Limiting:** The application implements rate limiting to prevent excessive crawling. Paid customers are allowed more workers to crawl more pages.

3. **Admin Control:** The system exposes two APIs to control the number of crawler workers and the crawling speed per hour per worker. Admins can dynamically adjust these settings to manage crawling resources effectively.

## Getting Started

### Prerequisites

- Go (Golang)
- Dependencies listed in `go.mod`
- [Colly](http://go-colly.org/) for web scraping


## API Endpoints

## Web Crawler
- Access the web crawler via the web interface by visiting your server's address (e.g., http://localhost:8080).
- Enter the URL you want to crawl, select the customer type (Paid or Free), and click "Crawl."
- The server checks if the page has been crawled within the last 60 minutes. If it's in the cache, it serves the cached data; otherwise, it crawls the page in real-time and returns the data.

## Admin Control Panel
- Access the Admin Control Panel at http://localhost:8080/admin.
- Use the Admin Panel to set the number of workers for Paid and Free customers.
- Adjust the crawling speed per hour per worker.

 ## Set the number of workers (Admin API)

 - Endpoint: /admin/set-workers
 - Parameters: customerType (Paid or Free), workerCount (number of workers)
 - Example: http://localhost:8080/admin/set-workers?customerType=Paid&workerCount=10

 ## Set the crawling speed (Admin API)

 - Endpoint: /admin/set-crawl-speed
 - Parameters: crawlSpeed (pages per hour per worker)
-  Example: http://localhost:8080/admin/set-crawl-speed?crawlSpeed=100


## Caching
The server caches crawled data for 60 minutes to reduce redundant crawling.

## Rate Limiting
Rate limiting is implemented to prevent excessive crawling. Paid customers have more workers (5) compared to Free customers (2).

### Installing

1. Clone the repository to your local machine.

   ```bash
   git clone  (https://github.com/AashishA11/sendx-backend-iit2020009)
   cd sendx-backend-IIT2020009
   
2. Start the Go server. Open a terminal and navigate to the project directory, then run:

   ```bash
   go run main.go
   
   
3. The server should now be running on port 8080.
   
4. Open a web browser and enter the following URL to access the web interface:
   
  ```bash
   http://localhost:8080
  ```

