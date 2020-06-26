package main

import (
	"fmt"
	"log"
	"time"

	"github.com/TofuOverdose/WebMapMaker/internal/scrapper"
	"github.com/TofuOverdose/WebMapMaker/internal/sitemap"
)

// simple demonstration of how this thing's supposed to work
func main() {
	config := scrapper.SearchConfig{
		IgnoreTopLevelDomain:  true,
		IncludeSubdomains:     true,
		IncludeLinksWithQuery: true,
	}
	scr := scrapper.NewLinkScrapper(config, 4)
	timeStart := time.Now()
	output, err := scr.GetInnerLinks("https://gobyexample.com")
	if err != nil {
		log.Fatal(err)
	}

	results := make([]scrapper.SearchResult, 1)
	maxHops := 0
	for o := range output {
		if o.Error != nil {
			log.Printf("ERROR on address %s: %s\n", o.Url, o.Error)
		} else {
			results = append(results, o)
			if o.Hops > maxHops {
				maxHops = o.Hops
			}
		}
	}

	fmt.Printf("Scrapping finished in %f seconds", time.Since(timeStart).Seconds())

	sm := sitemap.NewUrlSet()
	for _, res := range results {
		priority := 1.0
		if res.Hops > 0 {
			priority = float64(res.Hops) / priority
		}
		sm.AddUrl(*sitemap.NewUrl(res.Url, "", "never", priority))
	}

	data, err := sm.ToXML()
	if err != nil {
		log.Fatal(err)
	}
	_ = string(data)
}
