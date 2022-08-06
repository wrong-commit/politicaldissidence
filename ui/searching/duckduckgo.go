package searching

import (
	"fmt"
	"golang.org/x/net/html"
	"net/http"
	"strings"
)

// TODO: avoid resubmitting form to avoid flagging requests
type duckduck struct {
}

// Go will search DuckDuckGo's lite page and return top resuls
func (duckduck) Go(term string) ([]Link, error) {
	url := fmt.Sprintf("https://duckduckgo.com/?q=%s", term)
	var doc, err = postDuckDuckGo(url)
	if err != nil {
		return nil, err
	}
	// parse body as HTML for inspection
	node, err := html.Parse(strings.NewReader(doc))
	if err != nil {
		return nil, err
	}
	return parseSearchResults(node)
}

// hasClass returns true if the provided node has a class. only basic check of class name
func hasClass(n *html.Node, className string) bool {
	if n.Type == html.ElementNode {
		for _, attr := range n.Attr {
			if attr.Key == "class" && attr.Val == className {
				return true
			}
		}
	}
	return false
}

// shouldSubmitForm tells us if we should submit a form in the HTML
func shouldSubmitForm(node *html.Node) bool {
	return false
}

// parseSearchResults iterates the HTML to extract search results
func parseSearchResults(node *html.Node) ([]Link, error) {

	var isSponsoredLink func(*html.Node) bool
	isSponsoredLink = func(n *html.Node) bool {
		for parent := n; parent != nil; parent = parent.Parent {
			//logFn(parent)
			if parent.Data == "tr" && hasClass(parent, "result-sponsored") {
				//fmt.Println("Sponsored link found !")
				return true
			}
		}
		return false
	}

	var iterate func(*html.Node, []Link) ([]Link, error)
	iterate = func(n *html.Node, found []Link) ([]Link, error) {
		//logFn(n)

		//if n.Type == html.ElementNode && n.Data == "tr" {
		//	// break out of dodgy sponsored links
		//	if hasClass(n, "result-sponsored") {
		//		return nil, errors.New("ignore <tr>.result-sponsored")
		//	}
		//}
		// filter
		if n.Type == html.ElementNode && n.Data == "a" {
			if hasClass(n, "result-link") {
				// add href attribute and first child element as links
				if href := findHref(n); href != "" {
					description := "<unknown>"
					// assume first child is text
					if n.FirstChild != nil && n.FirstChild.Type == html.TextNode {
						description = n.FirstChild.Data
					}
					//fmt.Printf("Found link: %s\n\t%s\n", href, description)
					l := Link{href, description}
					found = append(found, l)
				}
			}
			// TODO: bug in html parser does not accept class attribute on TRs. boo ! use tokenizer API  https://zetcode.com/golang/net-html/
			//
			if isSponsoredLink(n) {
				//fmt.Println("Found sponsored link ", findHref(n))
				return nil, nil
			}
		}
		// tree iterator
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			// this is shit, checks that recursive call only append
			foundOther, _ := iterate(c, found)
			if len(foundOther) > len(found) {
				found = foundOther
			}
			if found == nil {
				// ignoring tree
				//fmt.Println("Broken out of sponsored tree ")
				//c = c.
				break
			}
		}
		return found, nil
	}

	return iterate(node, make([]Link, 0))
}

func findHref(n *html.Node) string {
	// iterate until href found
	for _, a := range n.Attr {
		if a.Key == "href" {
			return a.Val
		}
	}
	return ""
}

// search performs a GET to the provided url. returned read need to be closed
func postDuckDuckGo(term string) (string, error) {
	//body := fmt.Sprintf("q=%s", term)
	url := "https://lite.duckduckgo.com/lite/"
	values := make(map[string][]string)
	values["q"] = []string{term}
	// use PostForm sends correct Content Type
	resp, err := http.PostForm(url, values)
	if err := assertRequest(resp, err); err != nil {
		return "", err
	}
	defer resp.Body.Close()
	return bodyToString(&resp.Body), nil
}

//ignore trees of sponsored links
func logFn(n *html.Node) {
	// cleanup data that is hard to log
	nData := fmt.Sprintf("%s", n.Data)
	nData = strings.ReplaceAll(nData, "\r", "\\r")
	nData = strings.ReplaceAll(nData, "\n", "\\n")
	nData = strings.ReplaceAll(nData, "\t", "\\t")
	nType := fmt.Sprintf("%s", n.Type)
	allAtr := ""
	for _, a := range n.Attr {
		allAtr += fmt.Sprintf("%s=%s,", a.Key, a.Val)
	}
	fmt.Printf("\ttype=%s sz_a=%d data=%s attr=%s \n", nType, len(n.Attr), nData, allAtr)
}
