package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/texttheater/golang-levenshtein/levenshtein"
	"io"
	"io/ioutil"
	"log"
	"math"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// we need to set the Spotify application credentials in another file
// this is to prevent it getting checked in to source control
// check SpotifyCredentials.go.dist for details

type Spotify struct {
	listener     net.Listener
	refreshTimer *time.Timer

	clientId     string
	clientSecret string
	tokenData    spotifyTokenData

	tracksToAdd []string
}

// API data structs

type spotifyTokenData struct {
	AccessToken  string `json:"access_token"`
	ExpiresIn    int    `json:"expires_in"` // seconds
	RefreshToken string `json:"refresh_token"`
}

type spotifyErrorResponse struct {
	Error spotifyError `json:"error"`
}

type spotifyError struct {
	Status  int    `json:"status"`
	Message string `json:"message"`
}

type spotifyId struct {
	Id string `json:"id"`
}

type spotifyName struct {
	*spotifyId
	Name string `json:"name"`
}

type spotifySearchResult struct {
	TrackResult spotifyTrackSearchResult `json:"tracks"`
}

type spotifyTrackSearchResult struct {
	Tracks []spotifyTrack `json:"items"`
}

type spotifyTrack struct {
	*spotifyName
	Album   spotifyName   `json:"album"`
	Artists []spotifyName `json:"artists"`
}

func (tr spotifyTrack) ArtistName() string {
	if len(tr.Artists) > 0 {
		return tr.Artists[0].Name
	}
	return ""
}

// copied from net/http/server.go
type tcpKeepAliveListener struct {
	*net.TCPListener
}

func (ln tcpKeepAliveListener) Accept() (c net.Conn, err error) {
	tc, err := ln.AcceptTCP()
	if err != nil {
		return
	}
	tc.SetKeepAlive(true)
	tc.SetKeepAlivePeriod(3 * time.Minute)
	return tc, nil
}

func (ln tcpKeepAliveListener) Close() (err error) {
	log.Println("Shutting down HTTP server")
	err = ln.TCPListener.Close()
	if err != nil {
		return
	}
	return nil
}

func (s *Spotify) Authorize() (err error) {
	s.credentialsInit()

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `
			<script src="https://code.jquery.com/jquery-2.1.3.min.js"></script>
			<h1>Compact Track Loader</h1>
			<div id="error"></div>
			<div id="auth"></div>
			<script>
			$(function() {
				$.ajax('/me').done(function(me) {
					window.me = me;
					$('#auth').html('<p>Logged in as ' + me.id + '.</p>' +
						'<p>You can go back to the console window now.</p>');
				}).fail(function(xhr) {
					$('#auth').html('<a href="/authorize">Log in with Spotify</a>');
				});
			});
			</script>`)
	})

	// spotify api proxy
	http.HandleFunc("/me", func(w http.ResponseWriter, r *http.Request) {
		// TODO: check for "error" parameter and also handle other errors

		if len(s.tokenData.AccessToken) == 0 {
			http.Error(w, `{"error":"No Spotify access token present"}`, http.StatusForbidden)
			return
		}

		user := spotifyId{}

		err := s.doApiRequest("GET", "/me", nil, &user)
		if err != nil {
			panic(err)
		}

		// close listener only if successful
		if s.tokenData.AccessToken != "" {
			defer s.listener.Close()
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("{\"id\":\"" + user.Id + "\"}"))
	})

	http.HandleFunc("/authorize", func(w http.ResponseWriter, r *http.Request) {
		q := url.Values{
			"client_id":     {s.clientId},
			"response_type": {"code"},
			"redirect_uri":  {"http://" + r.Host + "/spotifyCallback"},
			// scopes needed:
			// user-library-modify to add tracks/albums
			// user-read-private is so we can use market=from_token which limits search results to
			//   tracks available in the user's market
			"scope": {"user-library-modify user-read-private"},
			//"show_dialog": {"true"}, // uncomment to force dialog even when already authorized
		}
		http.Redirect(w, r, "https://accounts.spotify.com/authorize?"+q.Encode(), http.StatusFound)
	})

	http.HandleFunc("/spotifyCallback", func(w http.ResponseWriter, r *http.Request) {
		// TODO: check for "error" URL parameter
		err = s.requestToken(r.URL.Query().Get("code"), r.Host)
		if err != nil {
			panic(err)
		}
		http.Redirect(w, r, "/", http.StatusFound)
	})

	listen := ":64055" // TODO: localhost
	log.Printf("Starting up HTTP server on %s ...", listen)
	log.Printf("Please go to http://localhost" + listen + " in your browser to log in to Spotify. I'll wait.")
	s.listener, err = net.Listen("tcp", listen)
	if err != nil {
		return
	}
	err = http.Serve(tcpKeepAliveListener{s.listener.(*net.TCPListener)}, nil)
	return
}

func (s *Spotify) requestToken(code string, host string) (err error) {
	response, err := http.PostForm("https://accounts.spotify.com/api/token",
		url.Values{
			"grant_type":    {"authorization_code"},
			"code":          {code},
			"redirect_uri":  {"http://" + host + "/spotifyCallback"},
			"client_id":     {s.clientId},
			"client_secret": {s.clientSecret},
		})
	defer response.Body.Close()
	if err != nil {
		return
	}

	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return
	}

	err = json.Unmarshal(body, &s.tokenData)
	if err != nil {
		return
	}

	// start refresh timer
	if s.tokenData.ExpiresIn > 0 {
		d := time.Duration(s.tokenData.ExpiresIn-30) * time.Second
		if s.refreshTimer == nil {
			s.refreshTimer = time.NewTimer(d)
		} else {
			s.refreshTimer.Reset(d)
		}
		go func() {
			<-s.refreshTimer.C
			log.Println("Refreshing auth token")
			s.requestToken(s.tokenData.RefreshToken, "localhost")
		}()
	}
	return
}

func (s *Spotify) doApiRequest(method string, pathAndQuery string, body io.Reader, responseData interface{}) (err error) {
	responseReader, err := s.doApiRequestReader(method, pathAndQuery, body)
	defer responseReader.Close()
	if err != nil {
		return
	}

	if responseData != nil {
		responseBody, err := ioutil.ReadAll(responseReader)
		if err != nil {
			return err
		}

		// check for error
		spErr := spotifyErrorResponse{}
		err = json.Unmarshal(responseBody, &spErr)
		if len(spErr.Error.Message) > 0 {
			return errors.New(spErr.Error.Message)
		}

		err = json.Unmarshal(responseBody, &responseData)
		if err != nil {
			return err
		}
	}

	return
}

func (s *Spotify) doApiRequestReader(method string, pathAndQuery string, body io.Reader) (responseReader io.ReadCloser, err error) {
	if len(s.tokenData.AccessToken) == 0 {
		return nil, errors.New("Missing Spotify access token")
	}

	uri := "https://api.spotify.com/v1" + pathAndQuery
	log.Printf("API %s %s", method, uri)
	client := &http.Client{}
	req, err := http.NewRequest(method, uri, body)
	if err != nil {
		return
	}
	req.Header.Set("Authorization", "Bearer "+s.tokenData.AccessToken)
	response, err := client.Do(req)
	if err != nil {
		return
	}

	responseReader = response.Body
	return
}

func (s *Spotify) findAndAdd(artist string, album string, title string, loadMode string) (err error) {
	// TODO: loadMode
	result := spotifySearchResult{}
	query := url.Values{
		"q":      {"artist:" + artist + " album:" + album + " track:" + title},
		"type":   {"track"},
		"market": {"from_token"},
	}
	err = s.doApiRequest("GET", "/search?"+query.Encode(), nil, &result)
	if err != nil {
		return
	}

	if len(result.TrackResult.Tracks) > 0 {
		lowScore := math.MaxInt32
		bestTrack := spotifyTrack{}
		for _, t := range result.TrackResult.Tracks {
			// score matches
			score := levenshtein.DistanceForStrings([]rune(artist), []rune(t.ArtistName()), levenshtein.DefaultOptions) +
				levenshtein.DistanceForStrings([]rune(album), []rune(t.Album.Name), levenshtein.DefaultOptions) +
				levenshtein.DistanceForStrings([]rune(title), []rune(t.Name), levenshtein.DefaultOptions)
			if score < lowScore {
				lowScore = score
				bestTrack = t
			}
		}
		log.Printf("Adding %s - %s - %s", bestTrack.ArtistName(), bestTrack.Album.Name, bestTrack.Name)
		s.tracksToAdd = append(s.tracksToAdd, bestTrack.Id)
		if len(s.tracksToAdd) == 50 {
			err = s.flushTracks()
			if err != nil {
				return
			}
		}
	} else {
		log.Println("No match")
	}
	return
}

func (s *Spotify) flushTracks() (err error) {
	if len(s.tracksToAdd) > 0 {
		log.Printf("Sending %d tracks", len(s.tracksToAdd))
		err = s.doApiRequest("PUT", "/me/tracks?ids="+strings.Join(s.tracksToAdd, ","), nil, nil)
		if err != nil {
			log.Println(err)
			return
		}
		// reset slice length to 0
		s.tracksToAdd = s.tracksToAdd[:0]
	}
	return
}
