// Package opml parses Feedly-compatible OPML exports into a flat list of feeds.
//
// Security: parsing uses encoding/xml, whose decoder does not resolve external
// entities or fetch DTDs, so hostile OPML cannot trigger XXE. We additionally
// reject any DOCTYPE to avoid entity-expansion surprises.
package opml

import (
	"encoding/xml"
	"fmt"
	"io"
	"strings"
)

// Feed is one subscription extracted from an OPML document.
type Feed struct {
	Title    string
	XMLURL   string
	HTMLURL  string
	Category string // derived from the enclosing outline folder
}

// outline is the recursive OPML node. Feedly nests feed outlines under a folder
// outline whose text/title names the category.
type outline struct {
	Text     string    `xml:"text,attr"`
	Title    string    `xml:"title,attr"`
	Type     string    `xml:"type,attr"`
	XMLURL   string    `xml:"xmlUrl,attr"`
	HTMLURL  string    `xml:"htmlUrl,attr"`
	Outlines []outline `xml:"outline"`
}

type document struct {
	XMLName xml.Name  `xml:"opml"`
	Body    []outline `xml:"body>outline"`
}

// Parse reads an OPML document and returns its feeds with categories resolved
// from the folder hierarchy.
func Parse(r io.Reader) ([]Feed, error) {
	data, err := io.ReadAll(io.LimitReader(r, 16<<20)) // 16 MiB cap on OPML
	if err != nil {
		return nil, fmt.Errorf("read opml: %w", err)
	}
	if hasDoctype(data) {
		return nil, fmt.Errorf("opml: DOCTYPE declarations are not allowed")
	}

	var doc document
	dec := xml.NewDecoder(strings.NewReader(string(data)))
	dec.Strict = false // Feedly exports occasionally contain loose markup
	if err := dec.Decode(&doc); err != nil {
		return nil, fmt.Errorf("parse opml: %w", err)
	}

	var feeds []Feed
	for _, o := range doc.Body {
		collect(o, "", &feeds)
	}
	return feeds, nil
}

// collect walks the outline tree. A node with an xmlUrl is a feed; otherwise it
// is a folder whose name becomes the category for its descendants.
func collect(o outline, category string, out *[]Feed) {
	name := firstNonEmpty(o.Title, o.Text)
	if o.XMLURL != "" {
		*out = append(*out, Feed{
			Title:    firstNonEmpty(name, o.XMLURL),
			XMLURL:   o.XMLURL,
			HTMLURL:  o.HTMLURL,
			Category: category,
		})
		return
	}
	// Folder: use its name as the category for children (top-level only, to
	// match Feedly's single-level folders; nested folders keep the outermost).
	childCategory := category
	if childCategory == "" {
		childCategory = name
	}
	for _, c := range o.Outlines {
		collect(c, childCategory, out)
	}
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func hasDoctype(data []byte) bool {
	return strings.Contains(strings.ToUpper(string(data)), "<!DOCTYPE")
}
