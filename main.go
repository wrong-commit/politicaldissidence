package main

import (
	"fmt"
	"politicaldissonance/fetcher"
)

func main() {
	senatorsUrl, err := fetcher.FindSenatorsCsv()
	if err != nil {
		fmt.Println("[-] Could not retreive Senators CSV", err.Error())
	}
	fmt.Println("[+] Senators Url", senatorsUrl)

	membersUrl, err := fetcher.FindMembersCsv()
	if err != nil {
		fmt.Println("[-] Could not retreive Members CSV", err.Error())
	}
	fmt.Println("[+] Members Url", membersUrl)

}
