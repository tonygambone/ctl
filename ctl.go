package main

import (
	"log"
)

func main() {
	ReadMedia(func(trackChannel TrackChannel) {
			for track := range trackChannel {
				log.Printf("%s - %s - %s", track.artist, track.album, track.title)
			}
		}, "/media/e/Music/The Black Keys", "/media/e/Music/Radiohead", "/media/e/Music/Metric")
}