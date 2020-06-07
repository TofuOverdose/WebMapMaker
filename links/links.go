package links

import (
	"io"
	"net/url"

	"golang.org/x/net/html"
)

type LinkType uint8

const (
	UndefinedTypeLink = iota
	AbsolutePathLink
	RelativePathLink
	AnchorLink
)

type Link struct {
	Name string
	Type LinkType
	Url  url.URL
}

func (link *Link) String() string {
	return link.Name + ": " + link.Url.String()
}

func getLinkType(url *url.URL) LinkType {
	if url.Hostname() == "" {
		if url.EscapedPath() == "" && url.Fragment != "" {
			return AnchorLink
		} else {
			return RelativePathLink
		}
	} else {
		return AbsolutePathLink
	}
}

func parseHref(linkNode *html.Node) (string, bool) {
	for _, attr := range linkNode.Attr {
		if attr.Key == "href" {
			return attr.Val, true
		}
	}
	return "", false
}

func ParseLinksChannel(reader io.Reader) (<-chan Link, <-chan error) {
	outChan := make(chan Link)
	errChan := make(chan error)

	node, err := html.Parse(reader)
	if err != nil {
		errChan <- err
		close(outChan)
		close(errChan)
		return outChan, errChan
	}

	var seekFunc func(node *html.Node)
	seekFunc = func(node *html.Node) {
		defer func() {
			for c := node.FirstChild; c != nil; c = c.NextSibling {
				seekFunc(c)
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
				Type: getLinkType(url),
			}
		}
	}

	go func() {
		seekFunc(node)
		close(outChan)
		close(errChan)
	}()

	return outChan, errChan
}
