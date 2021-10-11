package main

import (
	"fmt"
	"politicaldissonance/csv"
	"politicaldissonance/fetcher"
)

func main() {
	senatorsUrl, err := fetcher.FindSenatorsCsv()
	if err != nil {
		fmt.Println("[-] Could not retreive Senators CSV", err.Error())
		return
	}
	fmt.Println("[+] Senators Url", senatorsUrl)

	mps, err := csv.ParseSenatorMps(senatorsUrl)

	for i, mp := range mps {
		fmt.Printf("Senator [%d] = %s\n", i, mp.ToString())
	}

	membersUrl, err := fetcher.FindMembersCsv()
	if err != nil {
		fmt.Println("[-] Could not retreive Members CSV", err.Error())
		return
	}
	fmt.Println("[+] Members Url", membersUrl)

	mps, err = csv.ParseMemberMps(membersUrl)
	if err != nil {
		fmt.Println("[-] Could not get MP rows", err.Error())
	}

	for i, mp := range mps {
		fmt.Printf("HOR Member [%d] = %s\n", i, mp.ToString())
	}

}
