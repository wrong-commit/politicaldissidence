package data

import (
	"fmt"
	"politicaldissidence/whois"
)

type Domain struct {
	/* Domain Hostname */
	Hostname string `json:"hostname"`
	/* Empty string indicates domain expiry cannot be determined */
	Expiry  string `json:"expiry"`
	Expired bool   `json:"expired"`
}

// updateExpiry updates the expiry on a domain object
func (d *Domain) UpdateExpiry() (string, error) {
	// catch panic inside whois when domain is invalid
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in UpdateExpiry", r)
		}
	}()
	var err error
	var expiryDate string
	expiryDate, err = whois.GetExpiry(d.Hostname)
	if err != nil {
		return expiryDate, err
	}
	d.Expiry = expiryDate
	// TODO: check if date is after current date
	return d.Expiry, nil
}
