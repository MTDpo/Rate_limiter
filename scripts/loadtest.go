package main

import (
	"flag"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

func main() {
	url := flag.String("url", "http://localhost:8080/", "base URL")
	n := flag.Int("n", 1000, "total requests")
	c := flag.Int("c", 50, "concurrent goroutines")
	ips := flag.Int("ips", 10, "number of distinct IPs to simulate")
	flag.Parse()

	if *c <= 0 {
		*c = 1
	}
	if *ips <= 0 {
		*ips = 1
	}

	var allowed, rejected, errors int64
	start := time.Now()

	var wg sync.WaitGroup
	perWorker := (*n + *c - 1) / *c

	for g := 0; g < *c; g++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			client := &http.Client{Timeout: 5 * time.Second}
			ip := fmt.Sprintf("192.168.1.%d", workerID%*ips)
			for i := 0; i < perWorker; i++ {
				req, err := http.NewRequest(http.MethodGet, *url, nil)
				if err != nil {
					atomic.AddInt64(&errors, 1)
					continue
				}
				req.Header.Set("X-Forwarded-For", ip)
				resp, doErr := client.Do(req)
				if doErr != nil {
					atomic.AddInt64(&errors, 1)
					continue
				}
				resp.Body.Close()
				switch resp.StatusCode {
				case 200:
					atomic.AddInt64(&allowed, 1)
				case 429:
					atomic.AddInt64(&rejected, 1)
				default:
					atomic.AddInt64(&errors, 1)
				}
			}
		}(g)
	}
	wg.Wait()

	elapsed := time.Since(start)
	total := allowed + rejected + errors
	rps := float64(total) / elapsed.Seconds()

	fmt.Printf("\n=== Load Test Results ===\n")
	fmt.Printf("URL: %s\n", *url)
	fmt.Printf("Total Requests: %d (connections: %d, IPs: %d)\n", total, *c, *ips)
	fmt.Printf("Duration: %v\n", elapsed)
	fmt.Printf("Throughput: %.1f req/s\n", rps)
	fmt.Printf("Allowed (200): %d\n", allowed)
	fmt.Printf("Rejected (429): %d\n", rejected)
	fmt.Printf("Errors: %d\n", errors)
}
