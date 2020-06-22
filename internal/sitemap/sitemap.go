package sitemap

import (
	"encoding/xml"
	"time"
)

type Url struct {
	Loc        string  `xml:"loc"`
	Lastmod    string  `xml:"lastmod,omitempty"`
	Changefreq string  `xml:"changefreq,omitempty"`
	Priority   float64 `xml:"priority,omitempty"`
}

const timeFormat string = "2006-01-02T15:04:05-0700"

// NewUrl creates new Url struct instance
func NewUrl(location, lastmod, changefreq string, priority float64) *Url {
	if lastmod == "" {
		lastmod = time.Now().Format(timeFormat)
	}
	return &Url{
		Loc:        location,
		Lastmod:    lastmod,
		Changefreq: changefreq,
		Priority:   priority,
	}
}

type UrlSet struct {
	Namespace string `xml:"xmlns,attr"`
	Urls      []Url  `xml:"url"`
}

const defaultSitemapNamespace string = "https://www.sitemaps.org/schemas/sitemap/0.9/"

func NewUrlSet() *UrlSet {
	return &UrlSet{
		Namespace: defaultSitemapNamespace,
		Urls:      make([]Url, 0),
	}
}

func (us *UrlSet) AddUrl(urls ...Url) {
	us.Urls = append(us.Urls, urls...)
}

func (us *UrlSet) ToXML() ([]byte, error) {
	return xml.MarshalIndent(us, "", " ")
}

func (us *UrlSet) Locations() []string {
	urls := make([]string, 0)
	for _, u := range us.Urls {
		urls = append(urls, u.Loc)
	}
	return urls
}

type Sitemap struct {
	Loc string `xml:"loc"`
}

type Index struct {
	Sitemaps []Sitemap `xml:sitemap`
}

func NewIndex() *Index {
	return &Index{}
}

func (index *Index) AddSitemap(location string) {
	sm := Sitemap{
		Loc: location,
	}
	index.Sitemaps = append(index.Sitemaps, sm)
}
