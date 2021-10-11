package fetcher

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"golang.org/x/net/html"
)

/*
 * Get the URL to the Senate's CSV file from the APH site
 */
func FindSenatorsCsv() (string, error) {
	return findAphCsv("allsenph.csv")
}

/*
 * Get the URL to the House of Reps' Members CSV file from the APH site
 */
func FindMembersCsv() (string, error) {
	return findAphCsv("FamilynameRepsCSV.csv")
}

/*
 *
 */
func findAphCsv(expectedCsvHref string) (string, error) {
	doc, err := findAphHtml()
	if err != nil {
		fmt.Println("[-] Could not get HTML for APH site", err.Error())
		return "", err
	}

	filename, err := findAnchorWithPartialHref(doc, expectedCsvHref)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("https://www.aph.gov.au%s", filename), nil
}

/*
 * Get the HTML from the APH site containing Members and Senators list
 */
func findAphHtml() (*html.Node, error) {
	resp, err := http.Get("https://www.aph.gov.au/Senators_and_Members/Guidelines_for_Contacting_Senators_and_Members/Address_labels_and_CSV_files")

	if err != nil {
		fmt.Println("[-] Could not open request to get Senators", err.Error())
		return nil, err
	}

	defer resp.Body.Close()

	fmt.Println("[*]", resp.Request.Method, resp.StatusCode, resp.Request.URL.String())

	// find link wth "allsenph.csv"
	return html.Parse(resp.Body)
}

/*
 * Find an <a> tag in a HTML node that's href attribute contains the filename param
 */
func findAnchorWithPartialHref(doc *html.Node, filename string) (string, error) {
	var iterate func(*html.Node) (string, error)
	iterate = func(n *html.Node) (string, error) {
		if n.Type == html.ElementNode && n.Data == "a" {
			// iterate until href found
			for i := 0; i < len(n.Attr); i++ {
				if n.Attr[i].Key == "href" && strings.Index(n.Attr[i].Val, filename) > -1 {
					return n.Attr[i].Val, nil
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if filename, err := iterate(c); err == nil {
				return filename, nil
			}
		}
		return "", errors.New(fmt.Sprintf("Could not find <a/> with filename <%s>", filename))
	}

	return iterate(doc)
}
