package linkcrawler

import (
	"errors"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"

	"github.com/TofuOverdose/WebMapMaker/internal/links"
	"github.com/TofuOverdose/WebMapMaker/internal/utils"
)

type SearchConfig struct {
	IncludeSubdomains     bool
	IgnoreTopLevelDomain  bool
	IncludeLinksWithQuery bool
	ExcludedPaths         []regexp.Regexp
}

type FetchFunc func(string) (io.ReadCloser, error)

var defaultFetchFunc FetchFunc = func(addr string) (io.ReadCloser, error) {
	res, err := http.Get(addr)
	if err != nil {
		return nil, err
	}
	return res.Body, nil
}

type LinkCrawler struct {
	config      SearchConfig
	maxRoutines uint
	fetchFunc   FetchFunc
}

func (ls *LinkCrawler) SetFetchFunc(fetchFunc FetchFunc) {
	ls.fetchFunc = fetchFunc
}

func NewLinkCrawler(config SearchConfig, maxRoutines uint) *LinkCrawler {
	return &LinkCrawler{
		config:      config,
		fetchFunc:   defaultFetchFunc,
		maxRoutines: maxRoutines,
	}
}

type SearchResult struct {
	Url   string
	Hops  int
	Error error
}

func (ls *LinkCrawler) GetInnerLinks(initialAddr string) (<-chan SearchResult, error) {
	baseURL, err := url.Parse(initialAddr)
	if err != nil {
		return nil, err
	}
	baseHost := baseURL.Hostname()
	if baseHost == "" {
		return nil, errors.New("Hostname not found")
	}

	filter := func(link *links.Link) (string, bool) {
		for _, p := range ls.config.ExcludedPaths {
			if p.MatchString(link.Url.String()) {
				return "", false
			}
		}
		var nextUrl *url.URL
		switch link.Type {
		case links.AbsolutePathLink:
			hostLink := link.Url.Hostname()
			hostBase := baseHost
			if ls.config.IgnoreTopLevelDomain {
				hostLink = trimTopLevelDomain(hostLink)
				hostBase = trimTopLevelDomain(hostBase)
			}
			pass := isSubdomain(hostBase, hostLink) && ls.config.IncludeSubdomains
			if pass {
				nextUrl = &link.Url
			}
			return "", false
		case links.RelativePathLink:
			nextUrl = makeAbsoluteURL(baseURL, link.Url.Path)
		default:
			return "", false
		}

		// By default, URLs with query and anchor will be ignored
		// Not sure this is a right decision but at the moment I figured that it's certainly wrong to modify the URL assuming there should be path without query.
		// If this assumption is true, such URL will probably be linked from some other place and eventually will be found some time later anyway.
		if !ls.config.IncludeLinksWithQuery && !isCleanURL(nextUrl) {
			return "", false
		}

		return nextUrl.String(), true
	}

	return travel(initialAddr, ls.fetchFunc, filter, ls.maxRoutines), nil
}

func travel(
	initialAddr string,
	fetchFunc func(string) (io.ReadCloser, error),
	filterFunc func(*links.Link) (string, bool),
	maxRoutines uint,
) <-chan SearchResult {
	var sema *utils.Sema
	if maxRoutines > 0 {
		sema = utils.NewSema(maxRoutines)
	}
	outChan := make(chan SearchResult)
	history := make(map[string]bool)
	var mut sync.Mutex
	var wg sync.WaitGroup

	var visitFunc func(addr string, hopsCount int)
	visitFunc = func(addr string, hopsCount int) {
		if sema != nil {
			sema.WaitToAcquire()
		}

		defer func() {
			if sema != nil {
				sema.Release()
			}
		}()
		defer wg.Done()

		mut.Lock()
		if _, found := history[addr]; found {
			mut.Unlock()
			return
		}
		history[addr] = true
		mut.Unlock()

		pageReader, err := fetchFunc(addr)
		if err != nil {
			outChan <- SearchResult{
				Url:   addr,
				Error: err,
			}
			return
		}
		outChan <- SearchResult{
			Url:  addr,
			Hops: hopsCount,
		}

		dataChan, errChan := links.ParseLinksChannel(pageReader)
		defer pageReader.Close()

		for {
			select {
			case link, ok := <-dataChan:
				if !ok {
					return
				}
				nextAddr, pass := filterFunc(&link)
				if pass {
					wg.Add(1)
					go visitFunc(nextAddr, hopsCount+1)
				}
			case err := <-errChan:
				outChan <- SearchResult{
					Url:   addr,
					Error: err,
				}
			}
		}
	}

	wg.Add(1)
	go visitFunc(initialAddr, 0)
	go func() {
		wg.Wait()
		close(outChan)
	}()

	return outChan
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
func makeAbsoluteURL(base *url.URL, path string) *url.URL {
	return &url.URL{
		Scheme:     base.Scheme,
		Opaque:     base.Opaque,
		Host:       base.Host,
		Path:       path,
		ForceQuery: base.ForceQuery,
	}
}

func isCleanURL(u *url.URL) bool {
	return u.RawQuery == "" && u.Fragment == ""
}

// makeCleanURL removes query and anchor tag from URL
func makeCleanURL(u *url.URL) *url.URL {
	newURL, _ := url.Parse(u.String())
	newURL.RawQuery = ""
	newURL.Fragment = ""
	return newURL
}
