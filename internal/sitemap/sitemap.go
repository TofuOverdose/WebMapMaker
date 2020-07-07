package sitemap

import (
	"encoding/xml"
	"io"
	"strings"
	"time"
)

type Url struct {
	XMLName    xml.Name `xml:"url"`
	Loc        string   `xml:"loc"`
	Lastmod    string   `xml:"lastmod,omitempty"`
	Changefreq string   `xml:"changefreq,omitempty"`
	Priority   float64  `xml:"priority,omitempty"`
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
	XMLName   xml.Name `xml:"urlset"`
	Namespace string   `xml:"xmlns,attr"`
	Urls      []Url    `xml:"url"`
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

func (us *UrlSet) WriteXml(w io.Writer) error {
	enc := xml.NewEncoder(w)
	enc.Indent("", strings.Repeat(" ", 4))

	if err := enc.Encode(us); err != nil {
		return err
	}
	return nil
}

func (us *UrlSet) WritePlain(w io.Writer) error {
	tc := len(us.Urls)
	for i := 0; i < tc-1; i++ {
		ubytes := []byte(us.Urls[i].Loc + " ")
		if _, err := w.Write(ubytes); err != nil {
			return err
		}
	}
	ubytes := []byte(us.Urls[tc-1].Loc)
	if _, err := w.Write(ubytes); err != nil {
		return err
	}
	return nil
}

type sitemap struct {
	XMLName xml.Name `xml:"sitemap"`
	Loc     string   `xml:"loc"`
}

type Index struct {
	XMLName  xml.Name  `xml:"sitemapindex"`
	Sitemaps []sitemap `xml:"sitemap"`
}

func NewIndex() *Index {
	return &Index{}
}

func (index *Index) AddSitemap(location string) {
	sm := sitemap{
		Loc: location,
	}
	index.Sitemaps = append(index.Sitemaps, sm)
}
