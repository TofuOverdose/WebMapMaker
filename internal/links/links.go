package links

import (
	"io"
	"net/url"
	"strings"

	"golang.org/x/net/html"
)

type Link struct {
	Name string
	Url  url.URL
}

func (link *Link) String() string {
	return link.Name + " " + link.Url.String()
}

func parseHref(linkNode *html.Node) (string, bool) {
	for _, attr := range linkNode.Attr {
		if attr.Key == "href" {
			return attr.Val, true
		}
	}
	return "", false
}

func seekLinkNodes(node *html.Node, outChan chan Link, errChan chan error) {
	defer func() {
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			seekLinkNodes(c, outChan, errChan)
		}
	}()

	if node == nil || node.Type == html.ErrorNode {
		return
	}

	if node.Data == "a" {
		href, found := parseHref(node)
		if !found {
			return
		}

		href = strings.Trim(href, " ")
		url, err := url.Parse(href)
		if err != nil {
			errChan <- err
			return
		}

		var name string
		if child := node.FirstChild; child != nil {
			name = child.Data
		}
		outChan <- Link{
			Name: name,
			Url:  *url,
		}
	}
}

func FindLinks(reader io.Reader) (<-chan Link, <-chan error, error) {
	node, err := html.Parse(reader)
	if err != nil {
		return nil, nil, err
	}

	oc := make(chan Link)
	ec := make(chan error)
	go func() {
		seekLinkNodes(node, oc, ec)
		close(oc)
		close(ec)
	}()

	return oc, ec, nil
}
