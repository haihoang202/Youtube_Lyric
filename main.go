package main

import (
	"encoding/json"
	// "fmt"
	"github.com/julienschmidt/httprouter"
	// "golang.org/x/oauth2"
	"golang.org/x/net/html"
	"google.golang.org/api/googleapi/transport"
	"google.golang.org/api/youtube/v3"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
)

var developerKey = os.Getenv("YT_API_KEY")
var geniusKey = os.Getenv("GENIUS_ACCESS_KEY")
var geniusPath = "https://api.genius.com/search?q="

func main() {
	r := httprouter.New()

	r.GET("/", HomeHandler)
	r.GET("/songs/:id", ListSong)
	r.POST("/play", PlaySong)

	http.ListenAndServe(":8080", r)
}

func HomeHandler(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	var i interface{}
	t, _ := template.ParseFiles("index1.html")
	t.Execute(w, i)

}

func ListSong(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	song := p.ByName("id")[3:]
	genres, err := MakeGenRequest(song)

	if err == nil {
		res := make([][]string, 0)

		for _, i := range genres.Response.Hits {
			a := []string{i.Result.FullTitle, i.Result.HeaderImageThumbnailURL, i.Result.Path}
			res = append(res, a)
		}

		ret, err := json.Marshal(res)
		if err == nil {
			w.Header().Set("Content-type", "application/json")
			w.Write(ret)
		}
	}
}

func MakeGenRequest(request string) (GenRes, error) {
	client := &http.Client{}
	genURL := geniusPath + strings.Replace(request, " ", "%20", -1)
	req, _ := http.NewRequest("GET", genURL, nil)
	req.Header.Set("Authorization", "Bearer "+geniusKey)
	res, _ := client.Do(req)
	defer res.Body.Close()
	body, _ := ioutil.ReadAll(res.Body)
	var genres GenRes
	err := json.Unmarshal(body, &genres)

	if err != nil {
		println("ERROR", err)
		return genres, err
	} else {
		genres.SongID = request
		return genres, nil
	}
}

func MakeYTRequest(request string) (string, error) {
	client := &http.Client{
		Transport: &transport.APIKey{Key: developerKey},
	}

	service, err := youtube.New(client)
	if err != nil {
		log.Fatalf("Error creating new Youtube client: %v", err)
		return "", err
	}

	call := service.Search.List("snippet").
		Q(request).
		MaxResults(1).
		Type("video")

	response, err := call.Do()

	if err != nil {
		log.Fatalf("Error making Youtube API call: %v", err)
		return "", err
	}

	// videos := make(map[string]string)
	// videoID := response.Items[0].
	var videoID string
	for _, item := range response.Items {
		videoID = item.Id.VideoId
	}

	// json1, err := json.Marshal(videoID)

	// if err != nil {
	// 	log.Fatalf("Error making Youtube API call: %v", err)
	// 	return []byte(""), err
	// }

	// println(videoID)
	return videoID, nil
	// w.Header().Set("Content-type", "application/json")
	// w.WriteHeader(http.StatusOK)
	// w.Write(json1)
}
func PlaySong(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	body, _ := ioutil.ReadAll(r.Body)
	var postdata struct {
		Name string `json:name`
		Link string `json:link`
	}
	videoChan := make(chan string)
	lyricChan := make(chan []string)

	err := json.Unmarshal(body, &postdata)
	if err != nil {
		panic(err)
	}

	var videoID string
	var lyric []string

	go func(videoChan chan string) {
		ID, err := MakeYTRequest(postdata.Name)
		if err != nil {
			println("ERROR", err)
		}
		videoChan <- ID

	}(videoChan)

	go func(lyricChan chan []string) {
		lyrics, err := ParseGen4Lyric(postdata.Link)
		if err != nil {
			println("ERROR", err)
		}
		lyricChan <- lyrics
	}(lyricChan)

	videoID = <-videoChan
	lyric = <-lyricChan
	// println(string(lyric))
	var astruct struct {
		VideoID string
		Lyric   []string
	}

	astruct.VideoID = videoID
	astruct.Lyric = lyric

	res, _ := json.Marshal(astruct)

	w.Header().Set("Content-type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(res)
}

func ParseGen4Lyric(link string) ([]string, error) {
	client := &http.Client{}
	genURL := "https://genius.com" + link
	req, _ := http.NewRequest("GET", genURL, nil)
	resp, _ := client.Do(req)
	defer resp.Body.Close()
	lyrics := make([]string, 0)
	tokens := html.NewTokenizer(resp.Body)
	for {
		token := tokens.Next()
		switch token {
		case html.ErrorToken:
			break
		case html.StartTagToken:
			t := tokens.Token()
			if t.Data != "p" {
				continue
			}
			lyrics = GetLyrics(tokens, t)

			return lyrics, nil
		}
	}

	panic("Nothing get return")
}

func GetLyrics(tokens *html.Tokenizer, t html.Token) []string {
	res := make([]string, 0)
	if t.Type == html.EndTagToken && t.Data == "p" {
		return res
	}
	for {
		token := tokens.Next()
		switch token {
		case html.ErrorToken:
			return res
		case html.StartTagToken:
			continue
		case html.TextToken:
			res = append(res, tokens.Token().Data)
		case html.EndTagToken:
			if tokens.Token().Data == "p" {
				return res
			}
		}
	}
}

type GenRes struct {
	SongID string
	Meta   struct {
		Status int `json:"status"`
	} `json:"meta"`
	Response struct {
		Hits []struct {
			Highlights []interface{} `json:"highlights"`
			Index      string        `json:"index"`
			Type       string        `json:"type"`
			Result     struct {
				AnnotationCount          int    `json:"annotation_count"`
				APIPath                  string `json:"api_path"`
				FullTitle                string `json:"full_title"`
				HeaderImageThumbnailURL  string `json:"header_image_thumbnail_url"`
				HeaderImageURL           string `json:"header_image_url"`
				ID                       int    `json:"id"`
				LyricsOwnerID            int    `json:"lyrics_owner_id"`
				LyricsState              string `json:"lyrics_state"`
				Path                     string `json:"path"`
				PyongsCount              int    `json:"pyongs_count"`
				SongArtImageThumbnailURL string `json:"song_art_image_thumbnail_url"`
				Stats                    struct {
					Hot                   bool `json:"hot"`
					UnreviewedAnnotations int  `json:"unreviewed_annotations"`
					Concurrents           int  `json:"concurrents"`
					Pageviews             int  `json:"pageviews"`
				} `json:"stats"`
				Title             string `json:"title"`
				TitleWithFeatured string `json:"title_with_featured"`
				URL               string `json:"url"`
				PrimaryArtist     struct {
					APIPath        string `json:"api_path"`
					HeaderImageURL string `json:"header_image_url"`
					ID             int    `json:"id"`
					ImageURL       string `json:"image_url"`
					IsMemeVerified bool   `json:"is_meme_verified"`
					IsVerified     bool   `json:"is_verified"`
					Name           string `json:"name"`
					URL            string `json:"url"`
				} `json:"primary_artist"`
			} `json:"result"`
		} `json:"hits"`
	} `json:"response"`
}

type Responser struct {
	res []string
}
