package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"

	"github.com/aluttik/go-crossplane"
)

func main() {
	file, err := os.Open(os.Args[1])
	if err != nil {
		log.Fatal(err)
	}

	input, err := ioutil.ReadAll(file)
	if err != nil {
		log.Fatal(err)
	}

	var payload crossplane.Payload
	if err = json.Unmarshal(input, &payload); err != nil {
		log.Fatal(err)
	}

	combined, err := payload.Combined()
	if err != nil {
		log.Fatal(err)
	}
	config := combined.Config[0]
	options := &crossplane.BuildOptions{}

	var output bytes.Buffer
	if err = crossplane.Build(&output, config, options); err != nil {
		log.Fatal(err)
	}

	fmt.Println(output.String())
}
