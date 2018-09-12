package main

import (
	"log"
	"os"

	"github.com/yale-mgt-656-fall-2018/js-hw-grading/grading"
)

func main() {
	if len(os.Args) != 2 {
		log.Println("Error. You must supply a profile URL.")
		return
	}
	grading.TestAll(os.Args[1], true)
}
