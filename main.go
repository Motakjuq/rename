package main

import (
	"flag"
	"log"
	"os"
)

var (
	source = flag.String("in", "", "Define the input directory")
	dest   = flag.String("out", "", "Define the output format")
	keep   = flag.Bool("keep", true, "Keep source files")
	digits = flag.Uint("digits", 0, "Define the min amount of digits used by {count}")
)

func main() {

	fi, err := os.Stat(*source)
	if err != nil || !fi.IsDir() {
		log.Printf("Invalid in path. %v", err)
		return
	}

	err = Process(Config{
		OriginPath:    *source,
		DestinyPath:   *dest,
		KeepOriginals: *keep,
		Digits:        int(*digits),
	})

	if err != nil {
		log.Printf("Error found. %v", err)
	}

}
