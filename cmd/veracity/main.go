package main

import (
	"fmt"
	"log"
	"os"

	"github.com/datatrails/veracity"
)

func main() {
	app := veracity.NewApp()
	veracity.AddCommands(app)
	if err := app.Run(os.Args); err != nil {
		fmt.Printf("error: %v\n", err)
		log.Fatal(err)
	}
}
