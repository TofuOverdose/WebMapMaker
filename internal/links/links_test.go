package links

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
)

func TestFindLinksToCreateWithoutErrors(t *testing.T) {
	r := bytes.NewReader(make([]byte, 0))
	_, _, err := FindLinks(r)
	if err != nil {
		t.Fatal("FindLinks returned error on creation: ", err)
	}
}

func genReaderWithLinks(links map[string]string) *strings.Reader {
	output := "<div>"
	for name, href := range links {
		aNode := fmt.Sprintf("<a href=\"%s\">%s</a>", href, name)
		output += aNode
	}
	output += "</div>"
	return strings.NewReader(output)
}

func TestFindLinksToReadCorrectLinks(t *testing.T) {
	wanted := map[string]string{
		"link_number_one":   "/link/number/one/",
		"link_number_two":   "https://www.link.two/foo/bar",
		"link_number_three": "#somewhere",
	}
	results := make(map[string]string)
	r := genReaderWithLinks(wanted)
	outChan, errChan, err := FindLinks(r)
	if err != nil {
		t.Fatal("FindLinks returned error on creation: ", err)
	}

Loop:
	for {
		select {
		case link, ok := <-outChan:
			if !ok {
				break Loop
			} else {
				results[link.Name] = link.Url.String()
			}
		case err, ok := <-errChan:
			if ok && err != nil {
				t.Log(err)
				t.Fatal("Received error from error channel")
			}
		}
	}

	want := len(wanted)
	got := len(results)
	if got != want {
		t.Fatalf("Expected %v results, got %v\n", want, got)
	}
	for wKey, wVal := range wanted {
		gotVal, has := results[wKey]
		if !has {
			t.Logf("Link with name %s missing in the results\n", wKey)
			t.Fail()
		}
		if gotVal != wVal {
			t.Logf("Link %s: expected href to be %s, got %s\n", wKey, wVal, gotVal)
			t.Fail()
		}
	}
}
