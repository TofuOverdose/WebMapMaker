package main

import (
	"log"

	"github.com/TofuOverdose/WebMapMaker/internal/scrapper"
	"github.com/TofuOverdose/WebMapMaker/internal/sitemap"
)

// simple demonstration of how this thing's supposed to work
func main() {
	config := scrapper.Config{
		IgnoreTopLevelDomain: false,
		IncludeSubdomains:    false,
	}
	scr := scrapper.NewLinkScrapper(config)
	output, err := scr.GetInnerLinks("https://gobyexample.com")
	if err != nil {
		log.Fatal(err)
	}

	results := make([]scrapper.SearchResult, 0)
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
	log.Println(string(data))
}
