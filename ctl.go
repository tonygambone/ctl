package main

import (

)

// TODO: option to listen locally or on any interface
// TODO: options to add by album or track (update when spotify album API is out)
// TODO: cache tokens

func main() {
	/*
	ReadMedia(func(trackChannel TrackChannel) {
			for track := range trackChannel {
				log.Printf("%s - %s - %s", track.artist, track.album, track.title)
			}
		}, "/media/e/Music/The Black Keys", "/media/e/Music/Radiohead", "/media/e/Music/Metric")
	*/
	var spotify Spotify
	// this will return an error - "use of closed network connection", this is normal
	_ := spotify.Authorize()

}