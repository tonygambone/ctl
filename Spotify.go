package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/url"
	"time"
)

// we need to set the Spotify application credentials in another file
// this is to prevent it getting checked in to source control
// check SpotifyCredentials.go.dist for details

type Spotify struct {
	listener net.Listener

	clientId string
	clientSecret string
	tokenData spotifyTokenData
}

type spotifyTokenData struct {
	AccessToken  string `json:"access_token"`
	// TODO: handle refreshing
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
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
					$('#auth').html('<p>Logged in as <a href="' + me.external_urls.spotify + '" target="_blank">' + me.id + '</a></p>');
				}).fail(function(xhr) {
					$('#auth').html('<a href="/authorize">Log in with Spotify</a>');
				});
			});
			</script>`)
		})

	// spotify api proxy
	http.HandleFunc("/me", func(w http.ResponseWriter, r *http.Request) {
		// TODO: check for "error" parameter and also handle other errors

		if (len(s.tokenData.AccessToken) == 0) {
			http.Error(w, `{"error":"No Spotify access token present"}`, http.StatusForbidden)
			return
		}

		//uri := "https://api.spotify.com/v1/" + r.URL.Path[len("/api/"):] + "?" + r.URL.RawQuery
		uri := "https://api.spotify.com/v1/me"
		log.Printf("API %s %s", r.Method, uri)
		client := &http.Client{}
		req, err := http.NewRequest(r.Method, uri, r.Body)
		if (err != nil) {
			panic(err)
		}
		req.Header.Set("Authorization", "Bearer " + s.tokenData.AccessToken)
		response, err := client.Do(req)
		if (err != nil) {
			panic(err)
		}
		defer response.Body.Close()
		// close listener only if successful
		if (s.tokenData.AccessToken != "") {
			defer s.listener.Close()
		}
		w.Header().Set("Content-Type", "application/json")
		io.Copy(w, response.Body)
		})

	http.HandleFunc("/authorize", func(w http.ResponseWriter, r *http.Request) {
		q := url.Values{
			"client_id": {s.clientId},
			"response_type": {"code"},
			"redirect_uri": {"http://" + r.Host + "/spotifyCallback"},
			"scope": {"user-library-modify"},
			"show_dialog": {"true"}, // TODO: can remove this
		}
		http.Redirect(w, r, "https://accounts.spotify.com/authorize?" + q.Encode(), http.StatusFound)
		})

	http.HandleFunc("/spotifyCallback", func(w http.ResponseWriter, r *http.Request) {
		// TODO: check for "error" parameter and also handle other errors

		response, _ := http.PostForm("https://accounts.spotify.com/api/token",
			url.Values{
				"grant_type": {"authorization_code"},
				"code": {r.URL.Query().Get("code")},
				"redirect_uri": {"http://" + r.Host + "/spotifyCallback"},
				"client_id": {s.clientId},
				"client_secret": {s.clientSecret},
				})
		defer response.Body.Close()

		body, err := ioutil.ReadAll(response.Body)
		if (err != nil) {
			panic(err)
		}

		err = json.Unmarshal(body, &s.tokenData)
		if (err != nil) {
			panic(err)
		}
		http.Redirect(w, r, "/", http.StatusFound)
		})
	listen := ":64055" // TODO: localhost
	log.Printf("Starting up HTTP server on %s ...", listen)
	//log.Fatalln(http.ListenAndServe(listen, nil))
	s.listener, err = net.Listen("tcp", listen)
	if (err != nil) {
		return
	}
	err = http.Serve(tcpKeepAliveListener{s.listener.(*net.TCPListener)}, nil)
	return
}