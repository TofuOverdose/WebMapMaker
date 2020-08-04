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
	"github.com/TofuOverdose/WebMapMaker/internal/utils/sema"
)

// SearchConfig specifies link acceptance critereas for crawler
type SearchConfig struct {
	IncludeSubdomains     bool
	IgnoreTopLevelDomain  bool
	IncludeLinksWithQuery bool
	ExcludedPaths         []string
}

// FetchError carries data about HTTP response with 4xx or 5xx status codes
type FetchError struct {
	Code         int
	Status       string
	RequestURLs  []string
	RequestDump  []byte
	ResponseDump []byte
}

func (fe *FetchError) Error() string {
	firstReq := fe.RequestURLs[0]
	lastReq := fe.RequestURLs[len(fe.RequestURLs)-1]
	if firstReq != lastReq {
		return fmt.Sprintf("Fetch error from %s (original request for %s): %s", lastReq, firstReq, fe.Status)
	}
	return fmt.Sprintf("Fetch error from %s: %s", lastReq, fe.Status)
}

type fetchFunc func(string) (io.ReadCloser, error)

type filterFunc func(url.URL) bool

const defaultMaxRedirects = 10

var defaultFetchFunc fetchFunc = func(addr string) (io.ReadCloser, error) {
	reCount := 0
	urls := []string{addr}
	client := http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			reCount++
			if reCount == defaultMaxRedirects {
				return fmt.Errorf("HTTP client exceeded maximum of %d redirects (initial request for %s)", defaultMaxRedirects, addr)
			}
			urls = append(urls, req.URL.String())
			return nil
		},
	}
	res, err := client.Get(addr)
	if err != nil {
		return nil, err
	}

	if res.StatusCode >= 400 {
		reqDump, _ := httputil.DumpRequestOut(res.Request, false)
		resDump, _ := httputil.DumpResponse(res, false)
		return nil, &FetchError{
			Code:         res.StatusCode,
			Status:       res.Status,
			RequestURLs:  urls,
			RequestDump:  reqDump,
			ResponseDump: resDump,
		}
	}

	return res.Body, nil
}

type linkCrawler struct {
	initURL    *url.URL
	fetchFunc  fetchFunc
	filterFunc filterFunc
	history    *history
	sem        *sema.Sema
	wg         *sync.WaitGroup
}

func makeFilterFunc(config SearchConfig, initURL url.URL) filterFunc {
	initHostname := initURL.Hostname()
	initAddr := initURL.String()
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

		if u.Host == "" {
			newurl := initURL.ResolveReference(&u)
			u := *newurl
			addr = u.String()
		}

		// TODO make this configurable
		if !strings.HasPrefix(addr, initAddr) {
			return false
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

func (crawler *linkCrawler) visit(url url.URL, hopsCount int, outChan chan SearchResult, doneChan <-chan struct{}) {
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
		case <-doneChan:
			return
		case link, ok := <-linksChan:
			if !ok {
				linksChan = nil
			}
			if next := &link.URL; crawler.filterFunc(link.URL) {
				next = crawler.initURL.ResolveReference(next)
				if crawler.history.TryAdd(next.String()) {
					crawler.wg.Add(1)
					go crawler.visit(*next, hopsCount+1, outChan, doneChan)
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

	var sem *sema.Sema
	if opt.MaxRoutines > 0 {
		sem = sema.NewSema(opt.MaxRoutines)
	}
	crawler := &linkCrawler{
		initURL:    initURL,
		fetchFunc:  defaultFetchFunc,
		filterFunc: makeFilterFunc(opt.SearchConfig, *initURL),
		history:    newHistory(),
		wg:         &sync.WaitGroup{},
		sem:        sem,
	}

	outChan := make(chan SearchResult)
	crawler.wg.Add(1)
	go crawler.visit(*initURL, 0, outChan, ctx.Done())
	go func() {
		crawler.wg.Wait()
		close(outChan)
	}()
	return outChan, nil
}
