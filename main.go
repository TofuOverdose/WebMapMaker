package main

import (
	"fmt"

	"github.com/TofuOverdose/WebMapMaker/scrapper"
)

func main() {
	config := scrapper.Config{
		IgnoreTopLevelDomain: false,
		IncludeSubdomains:    false,
	}
	scr := scrapper.NewLinkScrapper(config)
	output, err := scr.GetInnerLinks("https://gobyexample.com")
	if err != nil {
		fmt.Println(err)
		return
	}

	results := make([]scrapper.SearchResult, 0)
	maxHops := 0
	for o := range output {
		if o.Error != nil {
			fmt.Printf("ERROR on address %s: %s\n", o.Url, o.Error)
		} else {
			results = append(results, o)
			if o.Hops > maxHops {
				maxHops = o.Hops
			}
		}
	}
}
