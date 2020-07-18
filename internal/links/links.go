package links

import (
	"io"
	"net/url"
	"strings"

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
	return link.Name + " " + link.Url.String()
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
			Type: getLinkType(url),
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

// TODO: test FindsLinks function above and remove this one
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
