package data

import "fmt"

type MP struct {
	Honorific     string
	FirstName     string
	Surname       string
	OtherName     string
	PreferredName string

	Party string
	State string
}

func (mp MP) Name() string {
	return fmt.Sprintf("%s %s %s %s", mp.Honorific, mp.FirstName, mp.OtherName, mp.Surname)
}

func (mp MP) ToString() string {
	return fmt.Sprintf("(%s) %s - %s", mp.Party, mp.Name(), mp.State)
}
