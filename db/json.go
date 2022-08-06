/**
 * Package responsible for reading and writing application data to a persistent storage medium.
 */
package db

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"politicaldissidence/data"
	"strings"
)

const mpJsonFilename = "mp_data.json"

// const domainsJsonFilename = "mp_domains.json"

func ReadMps() ([]data.MP, error) {
	data, err := readFile(mpJsonFilename)

	if err != nil {
		fmt.Println("[-] Could not read MP data")
		return nil, err
	}

	r := strings.NewReader(data)
	return deserializeMps(r)
}

// func ReadDomains() ([]data.Domain, error) {
// 	data, err := readFile(domainsJsonFilename)

// 	if err != nil {
// 		fmt.Println("[-] Could not read Domain data")
// 		return nil, err
// 	}

// 	r := strings.NewReader(data)
// 	return deserializeDomains(r)
// }

func WriteMps(mps []data.MP) error {
	return write(mps, mpJsonFilename)
}

// func WriteDomains(domains []data.Domain) error {
// 	return write(domains, domainsJsonFilename)
// }

func write(v interface{}, filename string) error {
	var file *os.File

	// create/open file
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		fmt.Println("[+] Creating file", filename)
		if file, err = os.Create(filename); err != nil {
			fmt.Println("[-] Could not create file", filename)
			return err
		}
	} else {
		file, err = os.OpenFile(filename, os.O_RDWR, os.ModeExclusive)
		if err != nil {
			fmt.Println("[-] Could not open file", filename)
			return err
		}
	}

	// convert v to json bytes[]
	var buf bytes.Buffer

	encoder := json.NewEncoder(&buf)
	encoder.SetIndent(" ", "  ")
	err := encoder.Encode(v)
	if err != nil {
		fmt.Println("[-] Could serialize JSON before writing to", filename)
		return err
	}

	// write json to file
	_, err = file.Write(buf.Bytes())
	if err != nil {
		fmt.Println("[-] Could not write", buf.Len(), "bytes of JSON to", filename)
	}
	return err
}

func readFile(filename string) (string, error) {
	buf, err := os.ReadFile(filename)
	if err != nil {
		return "", err
	}

	return string(buf), nil
}

// TODO: make deserializeX functions a single function
/**
 * Deserialize a JSON string into []data.MP
 */
func deserializeMps(r io.Reader) ([]data.MP, error) {
	var pResp []data.MP

	if err := json.NewDecoder(r).Decode(&pResp); err != nil {
		fmt.Printf("Could not unmarshal JSON - %s", err.Error())
		return nil, err
	}

	return pResp, nil
}

/**
 * Deserialize a JSON string into []data.Domain
 */
func deserializeDomains(r io.Reader) ([]data.Domain, error) {
	var pResp []data.Domain

	if err := json.NewDecoder(r).Decode(&pResp); err != nil {
		fmt.Printf("Could not unmarshal JSON - %s", err.Error())
		return nil, err
	}

	return pResp, nil
}
