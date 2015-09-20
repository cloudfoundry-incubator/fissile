package main

import (
	"github.com/opesun/copyrecur"
	"log"
)

func main() {
	err = copyrecur.CopyDir("/home/jaybill/data", "/home/jaybill/backup")
	if err != nil {
		log.Fatal(err)
	} else {
		log.Print("Files copied.")
	}
}
