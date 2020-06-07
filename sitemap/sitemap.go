package sitemap

type Url struct {
	Loc        string  `xml:"loc"`
	Lastmod    string  `xml:"lastmod"`
	Changefreq string  `xml:"changefreq"`
	Priority   float32 `xml:"priority"`
}

type Sitemap struct {
	Urls []Url `xml:"url"`
}

func (s *Sitemap) AddUrl(url Url) {
	s.Urls = append(s.Urls, url)
}

func (s *Sitemap) encode() string {

}
