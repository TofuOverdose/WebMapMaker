package linkcrawler

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"

	"github.com/TofuOverdose/WebMapMaker/internal/links"
	"github.com/TofuOverdose/WebMapMaker/internal/utils"
)

// SearchConfig specifies link acceptance critereas for crawler
type SearchConfig struct {
	IncludeSubdomains     bool
	IgnoreTopLevelDomain  bool
	IncludeLinksWithQuery bool
	ExcludedPaths         []string
}

type FetchError struct {
	Code         int
	Status       string
	RequestURLs  []string
	RefererURL   string
	RequestDump  []byte
	ResponseDump []byte
}

// NewFetchError makes new fetch error instance
func NewFetchError(response *http.Response, refererURL string) *FetchError {
	reqDump, _ := httputil.DumpRequestOut(response.Request, false)
	resDump, _ := httputil.DumpResponse(response, false)
	return &FetchError{
		Code:         response.StatusCode,
		Status:       response.Status,
		RefererURL:   refererURL,
		RequestDump:  reqDump,
		ResponseDump: resDump,
	}
}

func (fe *FetchError) Error() string {
	return fmt.Sprintf("Fetch error of resource %s from referer %s: %s", fe.RequestURLs[0], fe.RefererURL, fe.Status)
}

type fetchFunc func(string) (io.ReadCloser, error)

type filterFunc func(url.URL) bool

var defaultFetchFunc fetchFunc = func(addr string) (io.ReadCloser, error) {
	res, err := http.Get(addr)
	if err != nil {
		return nil, err
	}

	if res.StatusCode >= 400 {
		return nil, NewFetchError(res, addr)
	}

	return res.Body, nil
}

type linkCrawler struct {
	initURL     *url.URL
	maxRoutines uint
	fetchFunc   fetchFunc
	filterFunc  filterFunc
	history     *history
	sem         *utils.Sema
	wg          *sync.WaitGroup
}

func makeFilterFunc(config SearchConfig, initURL url.URL) filterFunc {
	initHostname := initURL.Hostname()
	return func(u url.URL) bool {
		addr := u.String()
		// Skip crap like this
		if addr == "" || addr[0] == '.' {
			return false
		}

		// Skip anchor links
		if u.Hostname() == "" && u.EscapedPath() == "" && u.Fragment != "" {
			return false
		}

		// Pass relative links
		if u.Host == "" {
			return true
		}

		// By default, URLs with query and anchor will be ignored
		// Not sure this is a right decision but at the moment I figured that it's certainly wrong to modify the URL assuming there should be path without query.
		// If this assumption is true, such URL will probably be linked from some other place and eventually will be found some time later anyway.
		if !config.IncludeLinksWithQuery && !isCleanURL(u) {
			return false
		}

		// Check if path is excluded
		for _, p := range config.ExcludedPaths {
			if strings.Contains(addr, p) {
				return false
			}
		}

		hn := u.Hostname()
		if config.IgnoreTopLevelDomain {
			hn = trimTopLevelDomain(hn)
			initHostname = trimTopLevelDomain(initHostname)
		}

		if config.IncludeSubdomains && !isSubdomain(initHostname, hn) {
			return false
		}

		return hn == initHostname
	}
}

// SearchResult contains data about the newly found link
type SearchResult struct {
	Addr  string
	Hops  int
	Error error
}

func (crawler *linkCrawler) visit(outChan chan SearchResult, url url.URL, hopsCount int) {
	address := url.String()
	if crawler.sem != nil {
		crawler.sem.WaitToAcquire()
	}
	defer func() {
		if crawler.sem != nil {
			crawler.sem.Release()
		}
	}()
	defer crawler.wg.Done()

	pageReader, err := crawler.fetchFunc(address)
	if err != nil {
		outChan <- SearchResult{
			Addr:  address,
			Error: err,
		}
		return
	}
	defer pageReader.Close()
	// send the successful search result to the output
	outChan <- SearchResult{
		Addr: address,
		Hops: hopsCount,
	}
	// parse links on the newly received html
	linksChan, errChan, err := links.FindLinks(pageReader)
	if err != nil {
		panic(err)
	}

	for {
		select {
		case link, ok := <-linksChan:
			if !ok {
				linksChan = nil
			}
			if next := &link.Url; crawler.filterFunc(link.Url) {
				next = crawler.initURL.ResolveReference(next)
				if crawler.history.TryAdd(next.String()) {
					crawler.wg.Add(1)
					go crawler.visit(outChan, *next, hopsCount+1)
				}
			}
		case e, ok := <-errChan:
			if !ok {
				errChan = nil
			}
			outChan <- SearchResult{
				Addr:  address,
				Error: e,
			}
		}
		if linksChan == nil && errChan == nil {
			return
		}
	}
}

// CrawlOptions is a structure to set up the behavior of crawler
type CrawlOptions struct {
	MaxRoutines  uint
	SearchConfig SearchConfig
}

// Option is a function that configures the crawler
type Option func(*CrawlOptions)

// OptionMaxRoutines sets maximum number of goroutines crawler can spawn. If the value is 0, indefinite amount of goroutines will be spawned on demand
// Default value is 0
func OptionMaxRoutines(num uint) Option {
	return func(co *CrawlOptions) {
		co.MaxRoutines = num
	}
}

// OptionSearchIncludeSubdomains allows crawler to include links with subdomains
// For example, if the initial hostname is example.com, crawler with this option turned on will visit the links on domains foo.example.com and/or bar.example.com
func OptionSearchIncludeSubdomains() Option {
	return func(co *CrawlOptions) {
		co.SearchConfig.IncludeSubdomains = true
	}
}

// OptionSearchIgnoreTopLevelDomain allows crawler to include links with different top level domains, which might be useful for international websites
// For example, if the initial hostname is example.foo, crawler with this option turned on will visit the links on domains example.bar and/or example.baz
func OptionSearchIgnoreTopLevelDomain() Option {
	return func(co *CrawlOptions) {
		co.SearchConfig.IgnoreTopLevelDomain = true
	}
}

// OptionSearchAllowQuery allows crawler to include links with queries (by default, all links with query strings are ignored)
func OptionSearchAllowQuery() Option {
	return func(co *CrawlOptions) {
		co.SearchConfig.IncludeLinksWithQuery = false
	}
}

// OptionSearchIgnorePaths allows to specify patterns for link paths crawler should ignore
func OptionSearchIgnorePaths(patterns ...string) Option {
	return func(co *CrawlOptions) {
		co.SearchConfig.ExcludedPaths = patterns
	}
}

// Crawl starts crawling the website to find all external links
// initialAddr must be full URL string with protocol without path, query string or anchor
// options is a slice of functional options from this package (functions starting with Option*) to configure the behavior of the crawler
func Crawl(ctx context.Context, initialAddr string, options ...Option) (<-chan SearchResult, error) {
	// TODO take ctx into account
	opt := CrawlOptions{}
	for _, o := range options {
		o(&opt)
	}

	initURL, err := url.Parse(initialAddr)
	if err != nil {
		return nil, err
	}
	if initURL.Hostname() == "" {
		return nil, errors.New("Hostname is empty")
	}

	var sem *utils.Sema
	if opt.MaxRoutines > 0 {
		sem = utils.NewSema(opt.MaxRoutines)
	}
	crawler := &linkCrawler{
		initURL:     initURL,
		maxRoutines: opt.MaxRoutines,
		fetchFunc:   defaultFetchFunc,
		filterFunc:  makeFilterFunc(opt.SearchConfig, *initURL),
		history:     newHistory(),
		wg:          &sync.WaitGroup{},
		sem:         sem,
	}

	outChan := make(chan SearchResult)
	crawler.wg.Add(1)
	go crawler.visit(outChan, *initURL, 0)
	go func() {
		crawler.wg.Wait()
		close(outChan)
	}()
	return outChan, nil
}

// includesTail checks if the "str" has "tail" substring on the right end
func includesTail(str string, tail string) bool {
	offset := strings.Index(str, tail)
	if offset < 0 {
		return false
	}
	if len(str)-offset == len(tail) {
		return true
	}
	return false
}

// isSubdomain checks whether the domain in second string is a subdomain of the one in first string
func isSubdomain(domain string, subdomain string) bool {
	domain = strings.Replace(domain, "www.", "", 1)
	subdomain = strings.Replace(subdomain, "www.", "", 1)
	return includesTail(subdomain, domain)
}

// trimTopLevelDomain removes top level domain (".com", ".net", etc.) from domain name string
func trimTopLevelDomain(domain string) string {
	parts := strings.Split(domain, ".")
	return strings.Join(parts[:len(parts)-1], ".")
}

// makeAbsoluteURL creates a copy of base URL with new path line
func makeAbsoluteURL(base url.URL, path string) url.URL {
	return url.URL{
		Scheme:     base.Scheme,
		Opaque:     base.Opaque,
		Host:       base.Host,
		Path:       path,
		ForceQuery: base.ForceQuery,
	}
}

func isCleanURL(u url.URL) bool {
	return u.RawQuery == "" && u.Fragment == ""
}

// makeCleanURL removes query and anchor tag from URL
func makeCleanURL(u *url.URL) *url.URL {
	newURL, _ := url.Parse(u.String())
	newURL.RawQuery = ""
	newURL.Fragment = ""
	return newURL
}

type history struct {
	data map[string]bool
	mut  sync.Mutex
}

func newHistory() *history {
	return &history{
		data: make(map[string]bool),
	}
}

func (h *history) TryAdd(key string) bool {
	h.mut.Lock()
	defer h.mut.Unlock()
	if _, has := h.data[key]; has {
		return false
	}

	h.data[key] = true
	return true
}

func (h *history) Entries() []string {
	h.mut.Lock()
	defer h.mut.Unlock()
	entries := make([]string, 0, len(h.data))
	for k := range h.data {
		entries = append(entries, k)
	}
	return entries
}
