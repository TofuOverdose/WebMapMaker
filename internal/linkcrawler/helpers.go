package linkcrawler

import (
	"net/url"
	"strings"
	"sync"
)

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

func isCleanURL(u url.URL) bool {
	return u.RawQuery == "" && u.Fragment == ""
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
