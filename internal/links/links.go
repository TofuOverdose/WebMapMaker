package links

import (
	"bytes"
	"fmt"
	"io"
	"net/url"

	"golang.org/x/net/html"
)

type Link struct {
	Name string
	Url  url.URL
}

func (link *Link) String() string {
	return link.Name + " " + link.Url.String()
}

func parseHref(linkNode *html.Node) string {
	for _, attr := range linkNode.Attr {
		if attr.Key == "href" {
			return attr.Val
		}
	}
	return ""
}

type LinkParseError struct {
	Node html.Node
	Href string
}

func (err LinkParseError) Error() string {
	var render bytes.Buffer
	html.Render(&render, &err.Node)
	return fmt.Sprintf("Failed to parse href of hyperlink node %s", render.String())
}

func seekLinkNodes(node *html.Node, outChan chan Link, errChan chan LinkParseError) {
	if node.Data == "a" {
		href := parseHref(node)
		if href != "" {
			url, err := url.Parse(href)
			if err != nil {
				var render bytes.Buffer
				html.Render(&render, node)
				errChan <- LinkParseError{
					Node: *node,
					Href: href,
				}
			} else {
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
	}

	for c := node.FirstChild; c != nil && c.Type != html.ErrorNode; c = c.NextSibling {
		seekLinkNodes(c, outChan, errChan)
	}
}

func FindLinks(reader io.Reader) (<-chan Link, <-chan LinkParseError, error) {
	node, err := html.Parse(reader)
	if err != nil {
		return nil, nil, err
	}

	oc := make(chan Link)
	ec := make(chan LinkParseError)
	go func() {
		seekLinkNodes(node, oc, ec)
		close(oc)
		close(ec)
	}()

	return oc, ec, nil
}
