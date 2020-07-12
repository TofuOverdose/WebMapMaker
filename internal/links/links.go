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

type OperationPipe struct {
	dataChan chan Link
	errChan  chan error
	doneChan chan struct{}
}

func NewOperationPipe(dataChan chan Link, errChan chan error) *OperationPipe {
	return &OperationPipe{
		dataChan: dataChan,
		errChan:  errChan,
	}
}

func (op *OperationPipe) Close() {
	close(op.dataChan)
	close(op.errChan)
}

func seekLinkNodes(node *html.Node, op *OperationPipe, dc DoneChan) {
	defer func() {
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			seekLinkNodes(c, op, dc)
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
			op.errChan <- err
			return
		}

		var name string
		if child := node.FirstChild; child != nil {
			name = child.Data
		}
		op.dataChan <- Link{
			Name: name,
			Url:  *url,
			Type: getLinkType(url),
		}
	}
}

// DoneChan is a channel that will be closed when some operation is done executing
type DoneChan chan struct{}

func FindsLinks(reader io.Reader, op *OperationPipe) (DoneChan, error) {
	node, err := html.Parse(reader)
	if err != nil {
		return nil, err
	}

	dc := make(DoneChan)
	go func() {
		seekLinkNodes(node, op, dc)
	}()
	return dc, nil
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
