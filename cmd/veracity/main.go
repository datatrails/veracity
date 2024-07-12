package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/datatrails/veracity"
)

func main() {

	ikwid := false
	envikwid := os.Getenv("VERACITY_IKWID")
	if envikwid == "1" || strings.ToLower(envikwid) == "true" {
		ikwid = true
	}
	app := veracity.NewApp(ikwid)
	veracity.AddCommands(app, ikwid)
	if err := app.Run(os.Args); err != nil {
		fmt.Printf("error: %v\n", err)
		log.Fatal(err)
	}
}
