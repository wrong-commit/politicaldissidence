package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
)

func strip(v interface{}) interface{} {
	switch t := v.(type) {
	case map[string]interface{}:
		delete(t, "lastChecked")
		for k, child := range t {
			t[k] = strip(child)
		}
		return t
	case []interface{}:
		for i, child := range t {
			t[i] = strip(child)
		}
		return t
	default:
		return v
	}
}

func main() {
	path := os.Args[1]
	b, err := os.ReadFile(path)
	if err != nil {
		panic(err)
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	var data interface{}
	if err := dec.Decode(&data); err != nil {
		panic(err)
	}
	data = strip(data)
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent(" ", "  ")
	if err := enc.Encode(data); err != nil {
		panic(err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0644); err != nil {
		panic(err)
	}
	n := 0
	walkCount(data, &n)
	fmt.Printf("recovered first JSON value, wrote %d bytes, domains=%d\n", buf.Len(), n)
}

func walkCount(v interface{}, n *int) {
	switch t := v.(type) {
	case map[string]interface{}:
		if _, ok := t["hostname"]; ok {
			*n++
		}
		for _, child := range t {
			walkCount(child, n)
		}
	case []interface{}:
		for _, child := range t {
			walkCount(child, n)
		}
	}
}
