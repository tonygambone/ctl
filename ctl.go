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
// TODO: MusicBrainz / Echo Nest http://developer.echonest.com/docs/v4#project-rosetta-stone
// TODO: generally handle errors
// TODO: keep track of progress
// TODO: maybe host an auth server to give out tokens?

// track load loads a single track that matches title, album, and artist
// album load loads an entire album that matches album and artist for any track found

type Options struct {
	load  string   // track, album
	paths []string // media paths to scan
}

const (
	LOAD_TRACK = "track"
	LOAD_ALBUM = "album"
)

var options Options

func init() {
	flag.StringVar(&options.load, "load", LOAD_TRACK, "how to load tracks to Spotify (track, album)")
}

func main() {
	if !populateOptions() {
		return
	}
	var spotify Spotify
	spotify.tracksToAdd = make([]string, 0, 50) // maximum 50 track IDs per put
	if options.load == LOAD_ALBUM {
		spotify.albumsAdded = make(map[string]bool)
	}
	// this will return an error - "use of closed network connection", this is normal
	_ = spotify.Authorize()

	ReadMedia(func(trackChannel TrackChannel) {
		for track := range trackChannel {
			log.Printf("%s - %s - %s", track.artist, track.album, track.title)
			spotify.findAndAdd(track.artist, track.album, track.title, options.load)
		}
	}, options.paths...)

	spotify.flushTracks()
}

func populateOptions() (ret bool) {
	flag.Parse()
	options.paths = flag.Args()
	ret = true

	res, _ := regexp.MatchString("^(track|album)$", options.load)
	if !res {
		fmt.Println("Invalid option for 'load' (must be 'track' or 'album')")
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
