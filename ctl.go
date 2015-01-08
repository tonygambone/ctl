package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"html/template"
)

// we need to set the Spotify application credentials in another file
// this is to prevent it getting checked in to source control
// check SpotifyCredentials.go.dist for details

var session struct {
	SpotifyClientId string
	SpotifyClientSecret string
	AccessToken  string `json:"access_token"`
	// TODO: handle refreshing
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
}

func createTemplate(base *template.Template, content string) *template.Template {
	return template.Must(template.Must(base.Clone()).Parse(content))
}

func main() {
	/*
	ReadMedia(func(trackChannel TrackChannel) {
			for track := range trackChannel {
				log.Printf("%s - %s - %s", track.artist, track.album, track.title)
			}
		}, "/media/e/Music/The Black Keys", "/media/e/Music/Radiohead", "/media/e/Music/Metric")
	*/
	spotifyCredentialsInit()

	baseTemplate := template.Must(template.New("base").Parse(`
		<h1>Compact Track Loader</h1>
		<p>{{ template "content" . }}</p>`))
	templates := map[string]*template.Template{
		"index": createTemplate(baseTemplate, `{{ define "content" }}index{{ end }}`),
		"about": createTemplate(baseTemplate, `{{ define "content" }}about{{ end }}`),
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		templates["index"].Execute(w, nil)
		})
	http.HandleFunc("/about", func(w http.ResponseWriter, r *http.Request) {
		templates["about"].Execute(w, nil)
		})
	http.HandleFunc("/authorize", func(w http.ResponseWriter, r *http.Request) {
		q := url.Values{
			"client_id": {session.SpotifyClientId},
			"response_type": {"code"},
			"redirect_uri": {"http://" + r.Host + "/spotifyCallback"},
			"scope": {"user-library-modify"},
			"show_dialog": {"true"}, // TODO: can remove this
		}
		http.Redirect(w, r, "https://accounts.spotify.com/authorize?" + q.Encode(), 302)
		})
	http.HandleFunc("/spotifyCallback", func(w http.ResponseWriter, r *http.Request) {
		// TODO: check for "error" parameter and also handle other errors

		response, _ := http.PostForm("https://accounts.spotify.com/api/token",
			url.Values{
				"grant_type": {"authorization_code"},
				"code": {r.URL.Query().Get("code")},
				"redirect_uri": {"http://" + r.Host + "/spotifyCallback"},
				"client_id": {session.SpotifyClientId},
				"client_secret": {session.SpotifyClientSecret},
				})
		defer response.Body.Close()

		body, _ := ioutil.ReadAll(response.Body)

		_ = json.Unmarshal(body, &session)
		})
	log.Println("Starting up...")
	log.Fatalln(http.ListenAndServe(":64055", nil)) // TODO: localhost
}