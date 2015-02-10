Compact Track Loader
====================

Compact Track Loader is a program for reading a local media library and adding the tracks it finds
to your Spotify "Your Music" area.

Once authorized with Spotify, it will allow you to specify your media library directories, and load
each track it finds into your [Spotify account](https://play.spotify.com/collection/songs) by searching
Spotify's library for the track.

Usage
-----

Scan some directories, and load each track found into Spotify (these are equivalent):

`ctl /path/to/dir1 /path/to/dir2`
`ctl --load=track /path/to/dir1 /path/to/dir2`

Scan some directories, and load all tracks from each album found into Spotify:

`ctl --load=album /path/to/dir1 /path/to/dir2`

This will help fill out albums with missing tracks. The album mode is not implemented yet.

Authorization
-------------

CTL needs to get access to your account, and to do so it needs to follow Spotify's web-based authorization
procedure.  Once it starts, you need to connect to it via a web browser, log in to Spotify, and authorize it
to access your account.  CTL will briefly turn itself into a web server to accomplish this. Once it's finished
authorizing, the action takes place in the console.

You'll also need a Spotify API key. You can create one [here](https://developer.spotify.com/my-applications/).
Make sure to add a redirect URI that matches how you will connect to CTL. If you are using CTL on your local
machine, add:

* http://127.0.0.1:64055/spotifyCallback
* http://localhost:64055/spotifyCallback

And if you are using CTL on a remote machine, add:

* http://<ip or hostname>:64055/spotifyCallback

Rename the file `SpotifyCredentials.go.dist` to `SpotifyCredentials.go` and put the API client ID and secret in
the spaces provided.

This is all perhaps a bit awkward.

Matching
--------

CTL will do its best to find matching tracks in Spotify. Sometimes it will load a track with the correct title,
but in a different album (compilation, remaster, live recording, etc.).

H/T [texttheater/golang-levenshtein](https://github.com/texttheater/golang-levenshtein)

TODO
----

* Support MusicBrainz or other ID tagging systems via [EchoNest](http://developer.echonest.com/docs/v4#project-rosetta-stone) for more precise matching
* Support Spotify's /me/albums API if/when it comes out (seems like they are working on it)
* Keep track of scanned tracks so we can continually add just the new tracks to Spotify
* Fix awkward authorization setup
* Implement album load mode