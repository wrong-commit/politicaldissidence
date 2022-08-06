package main

import (
	"fmt"
	"politicaldissidence/csv"
	"politicaldissidence/data"
	"politicaldissidence/db"
	"politicaldissidence/fetcher"
	"politicaldissidence/ui"
	"politicaldissidence/ui/searching"
	"politicaldissidence/whois"
	"strings"
)

const sixtynine = 69 - 1

func main() {
	mainGui()
}
func mainUrlSearcher() {
	allMps, err := db.ReadMps()
	if err != nil {
		return
	}
	fmt.Println("Loaded MPs, searching terms for MP 69", allMps[sixtynine].GoogleSearchTerm())
	searcher := searching.UrlSearcher{}
	resp, err := searcher.Search(allMps[sixtynine].GoogleSearchTerm())
	if err != nil {
		fmt.Println(err)
	}
	for _, x := range resp {
		fmt.Println(x)
	}
}

func mainGui() {
	// allMps, err := db.ReadMps()
	// if err != nil {
	// 	return
	// }
	/*
		if err := db.WriteMps(allMps); err != nil {
			return
		}
	*/
	ui.InitApp()
	// mps, err := getSenatorsAndHorMps()
	// if err != nil {
	// 	return
	// }
	// for _, mp := range mps {
	// 	fmt.Println(mp.GoogleSearchTerm())
	// }
	// fmt.Println("Found", len(mps), "Federal MPs")

	// db.WriteMps(mps)

	// mps, err := db.ReadMps()

	// if err != nil {
	// 	return
	// }

	// addFirstDomains(&mps)
	// domains := make([]data.Domain, 0)

	// for _, mp := range mps {
	// 	if mp.Domains != nil {
	// 		continue
	// 	}
	// 	fmt.Println(mp.GoogleSearchTerm())
	// 	fmt.Print("Enter domain name:")
	// 	var input string
	// 	fmt.Scanln(&input)
	// 	// remove protocal and slashes from domain, makes copying from google easier
	// 	trimmedDomain :=
	// 		strings.ReplaceAll(
	// 			strings.Replace(
	// 				strings.Replace(input, "http:", "", 1),
	// 				"https:", "", 1),
	// 			"/", "")
	// 	fmt.Println(trimmedDomain)

	// 	if input == "" {
	// 		break
	// 	}

	// 	domains = append(domains, data.Domain{
	// 		Hostname: trimmedDomain,
	// 	})
	// }

	// if err = db.WriteMps(mps); err != nil {
	// 	fmt.Println(err.Error())
	// } else {
	// 	fmt.Println("[+] Wrote MP data")
	// }
}

/**
 * TODO: Using points here seems almost wrong, is it wrong ? why did assigning mp := (*mps)[i] not allow mp.Domains to
 * be set directly ?
 */
func addFirstDomains(mps *[]data.MP) {
	for i := range *mps {
		if !(*mps)[i].NeedsDomain() {
			continue
		}

		fmt.Println((*mps)[i].GoogleSearchTerm())
		fmt.Print("Enter domain name: ")
		var input string
		fmt.Scanln(&input)
		// remove protocal and slashes from domain, makes copying from google easier
		trimmedDomain :=
			strings.ReplaceAll(
				strings.Replace(
					strings.Replace(input, "http:", "", 1),
					"https:", "", 1),
				"/", "")
		if input == "" {
			break
		}
		// lookup expiry
		expiry, err := whois.GetExpiry(trimmedDomain)
		if err != nil {
			fmt.Println("[-] Could not lookup hostname", trimmedDomain, err.Error())
			expiry = ""
		}
		(*mps)[i].Domains = make([]data.Domain, 0)
		(*mps)[i].Domains = append((*mps)[i].Domains, data.Domain{
			Hostname: trimmedDomain,
			Expiry:   expiry,
			Expired:  false,
		})
	}
}

func getSenatorsAndHorMps() ([]data.MP, error) {
	senatorsUrl, err := fetcher.FindSenatorsCsv()
	if err != nil {
		fmt.Println("[-] Could not retreive Senators CSV", err.Error())
		return nil, err
	}
	fmt.Println("[+] Senators Url", senatorsUrl)

	senators, err := csv.ParseSenatorMps(senatorsUrl)

	for i, mp := range senators {
		fmt.Printf("Senator [%d] = %s\n", i, mp.ToString())
	}

	membersUrl, err := fetcher.FindMembersCsv()
	if err != nil {
		fmt.Println("[-] Could not retreive Members CSV", err.Error())
		return nil, err
	}
	fmt.Println("[+] Members Url", membersUrl)

	hors, err := csv.ParseMemberMps(membersUrl)
	if err != nil {
		fmt.Println("[-] Could not get MP rows", err.Error())
	}

	for i, mp := range hors {
		fmt.Printf("HOR Member [%d] = %s\n", i, mp.ToString())
	}

	return append(hors, senators...), nil
}
