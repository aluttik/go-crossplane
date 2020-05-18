package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"

	crossplane "github.com/aluttik/go-crossplane"
)

func main() {
	// rdr := bytes.NewReader([]byte("events {\n whatever;\n okay 'then';\n} main;"))
	// for token := range crossplane.Lex(rdr) {
	// 	if token.Error != nil {
	// 		log.Fatal(token.Error)
	// 	}
	// 	fmt.Printf("%#v\n", token)
	// }

	path := os.Args[1]
	payload, err := crossplane.Parse(path, &crossplane.ParseOptions{})
	if err != nil {
		log.Fatal(err)
	}
	b, _ := json.Marshal(payload)
	fmt.Println(string(b))
}
