package scrapper

import (
	"errors"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"

	"github.com/TofuOverdose/WebMapMaker/links"
)

type Config struct {
	IncludeSubdomains    bool
	IgnoreTopLevelDomain bool
	IgnoreQuery          bool
	ExcludedPaths        []regexp.Regexp
}

type FetchFunc func(string) (io.ReadCloser, error)

var defaultFetchFunc FetchFunc = func(addr string) (io.ReadCloser, error) {
	res, err := http.Get(addr)
	if err != nil {
		return nil, err
	}
	return res.Body, nil
}

type LinkScrapper struct {
	config    Config
	fetchFunc FetchFunc
}

func (ls *LinkScrapper) SetFetchFunc(fetchFunc FetchFunc) {
	ls.fetchFunc = fetchFunc
}

func NewLinkScrapper(config Config) *LinkScrapper {
	return &LinkScrapper{
		config:    config,
		fetchFunc: defaultFetchFunc,
	}
}

type SearchResult struct {
	Url   string
	Error error
}

func (ls *LinkScrapper) GetInnerLinks(initialAddr string) (<-chan SearchResult, error) {
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
				return link.Url.String(), true
			}
			return "", false
		case links.RelativePathLink:
			return restoreFullUrl(baseURL, link.Url.String()), true
		default:
			return "", false
		}
	}

	return travel(initialAddr, ls.fetchFunc, filter), nil
}

func travel(
	initialAddr string,
	fetchFunc func(string) (io.ReadCloser, error),
	filterFunc func(*links.Link) (string, bool),
) <-chan SearchResult {
	outChan := make(chan SearchResult)
	history := make(map[string]bool)
	var mut sync.Mutex
	var wg sync.WaitGroup

	var visitFunc func(addr string)
	visitFunc = func(addr string) {
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
			Url: addr,
		}

		dataChan, errChan := links.ParseLinksChannel(pageReader)
		defer pageReader.Close()

		for {
			select {
			case link, ok := <-dataChan:
				if !ok {
					return
				} else {
					nextAddr, pass := filterFunc(&link)
					if pass {
						wg.Add(1)
						go visitFunc(nextAddr)
					}
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
	go visitFunc(initialAddr)
	go func() {
		wg.Wait()
		close(outChan)
	}()

	return outChan
}

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

func isSubdomain(domain string, subdomain string) bool {
	domain = strings.Replace(domain, "www.", "", 1)
	subdomain = strings.Replace(subdomain, "www.", "", 1)
	return includesTail(subdomain, domain)
}

func trimTopLevelDomain(domain string) string {
	parts := strings.Split(domain, ".")
	return strings.Join(parts[:len(parts)-1], ".")
}

func restoreFullUrl(base *url.URL, path string) string {
	var hostname string
	if h := base.Hostname(); h == "" {
		return path
	} else {
		hostname = h
	}
	var scheme string
	if s := base.Scheme; s == "" {
		scheme = "https"
	} else {
		scheme = s
	}
	return scheme + "://" + hostname + path
}
