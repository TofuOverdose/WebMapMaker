package main

import (
	"fmt"

	"github.com/TofuOverdose/WebMapMaker/scrapper"
)

func main() {
	config := scrapper.Config{
		IgnoreTopLevelDomain: true,
		IncludeSubdomains:    true,
	}
	scrapper := scrapper.NewLinkScrapper(config)
	output, err := scrapper.GetInnerLinks("https://www.google.com")
	if err != nil {
		fmt.Println(err)
		return
	}
	count := 0
	for o := range output {
		if o.Error != nil {
			fmt.Printf("ERROR on address %s: %s\n", o.Url, o.Error)
		} else {
			count++
			fmt.Println(count)
		}

	}
	fmt.Printf("Found %d inner links\n", count)
}
