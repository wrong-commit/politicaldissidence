package data

import "fmt"

type MP struct {
	Honorific     string `json:"honorific"`
	FirstName     string `json:"firstName"`
	Surname       string `json:"surnname"`
	OtherName     string `json:"otherName"`
	PreferredName string `json:"preferredName"`

	Electorate string `json:"electorate"`
	Party      string `json:"party"`
	State      string `json:"state"`
	Level      string `json:"level"`

	Domains []Domain `json:"domains"`
}

func (mp MP) NeedsDomain() bool {
	return mp.Domains == nil || len(mp.Domains) == 0
}

func (mp MP) Name() string {
	return fmt.Sprintf("%s %s %s", mp.Honorific, mp.FirstName, mp.Surname)
}

func (mp MP) ToString() string {
	return fmt.Sprintf("(%s) %s - %s", mp.Party, mp.Name(), mp.Electorate)
}

func (mp MP) GoogleSearchTerm() string {
	return fmt.Sprintf("%s member for %s %s ", mp.Name(), mp.Electorate, mp.Party)
}
