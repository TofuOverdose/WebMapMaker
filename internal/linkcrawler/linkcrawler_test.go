package linkcrawler

import (
	"fmt"
	"math/rand"
	"net/url"
	"testing"
)

func genURL(scheme, host string, pathSegments, queryParams int, includeAnchor bool) url.URL {
	var path string
	for i := 0; i < pathSegments; i++ {
		path = fmt.Sprintf("%s/path%d", path, i)
	}

	var query string
	for i := 0; i < queryParams; i++ {
		query = query + fmt.Sprintf("query%d=value%d&", i, i)
	}
	if len(query) > 0 {
		query = query[:len(query)-1]
	}

	var anchor string
	if includeAnchor {
		anchor = "some_anchor_on_page"
	}

	return url.URL{
		Scheme:   scheme,
		Host:     host,
		Path:     path,
		RawQuery: query,
		Fragment: anchor,
	}
}

func TestMakeCleanUrl(t *testing.T) {
	r := rand.New(rand.NewSource(99))
	dirtyURL := genURL("https", "testdomain.test", r.Intn(5), r.Intn(5), r.Intn(1) > 1)
	cleanURL := dirtyURL
	cleanURL.RawQuery = ""
	cleanURL.Fragment = ""
	want := cleanURL.String()
	got := makeCleanURL(&dirtyURL).String()
	if got != want {
		t.Errorf("Wanted %s, got %s", want, got)
	}
}

func TestMakeAbsoluteUrl(t *testing.T) {
	r := rand.New(rand.NewSource(99))
	host := "test.domain.test"
	base := genURL("https", host, 0, 0, false)
	want := genURL("https", host, r.Intn(5), 0, false)
	got := makeAbsoluteURL(&base, want.Path)
	if got.String() != want.String() {
		t.Errorf("Wanted %s, got %s", want.String(), got.String())
	}
}
