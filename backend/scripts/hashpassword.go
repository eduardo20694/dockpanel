//go:build ignore

package main

import (
	"fmt"
	"os"

	"golang.org/x/crypto/bcrypt"
)

func main() {
	password := "TROQUE_A_SENHA_DO_LOGIN"
	if len(os.Args) > 1 {
		password = os.Args[1]
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		panic(err)
	}
	fmt.Println(string(hash))
}
