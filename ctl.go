package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"regexp"
)

// TODO: maybe a set of port numbers to choose from
// TODO: update when spotify album API is out

type Options struct {
	match string // title, album TODO: artist?
	// TODO: option to specify token info to bypass the auth?
	paths []string // media paths to scan
}

var options Options

func init() {
	flag.StringVar(&options.match, "match", "title", "how to match tracks to add on Spotify (title, album)")
}

func main() {
	if !populateOptions() {
		return
	}
	var spotify Spotify
	// this will return an error - "use of closed network connection", this is normal
	_ = spotify.Authorize()

	ReadMedia(func(trackChannel TrackChannel) {
		for track := range trackChannel {
			log.Printf("%s - %s - %s", track.artist, track.album, track.title)
		}
	}, options.paths...)
}

func populateOptions() (ret bool) {
	flag.Parse()
	options.paths = flag.Args()
	ret = true

	res, _ := regexp.MatchString("^(title|album)$", options.match)
	if !res {
		fmt.Println("Invalid option for 'match' (must be 'title' or 'album')")
		ret = false
	}

	res = len(options.paths) > 0
	if !res {
		fmt.Println("No files or directories specified")
		ret = false
	} else {
		for _, el := range options.paths {
			if _, err := os.Stat(el); err != nil {
				fmt.Printf("Cannot %s\n", err)
				ret = false
			}
		}
	}
	return
}
