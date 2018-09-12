package main

import (
	"fmt"

	"github.com/yale-mgt-656-fall-2018/js-hw-grading/grading"
)

func main() {
	fmt.Println("hello world")
	url := "https://www.freecodecamp.org/fccc861a26c-236c-480a-9ada-ca754f20c767"
	grading.TestAll(url, true)
}
