package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"golang.org/x/net/html"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strings"
)

var Default_destination string = "/tmp"

type Config struct {
	url         string
	destination string
	debug       bool
}

func (c *Config) ParseConfig() error {
	flag.StringVar(&c.url, "url", "", "Arte+7 url")
	flag.StringVar(&c.destination, "destination", Default_destination, "Where we put the video")
	flag.BoolVar(&c.debug, "debug", false, "debug mode")

	flag.Parse()

	if len(c.url) == 0 {
		fmt.Fprintf(os.Stderr, "You must give an url\n")
		return errors.New("Missing argument url")
	}

	return nil
}

func DownloadUrl(url string) string {
	resp, err := http.Get(url)
	if err != nil {
		panic(err)
	}

	defer resp.Body.Close()
	html, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		panic(err)
	}

	return string(html)
}

func renderNode(n *html.Node) string {
	var buf bytes.Buffer
	w := io.Writer(&buf)
	html.Render(w, n)
	return buf.String()
}

func RetrieveJsonUrl(index_url string) (string, error) {
	var json_url string
	var crawler func(*html.Node)

	index := DownloadUrl(index_url)
	doc, _ := html.Parse(strings.NewReader(index))

	crawler = func(node *html.Node) {
		if node.Type == html.ElementNode && node.Data == "script" {
			if node.FirstChild != nil && node.FirstChild.Data != "" {
				snode := strings.TrimSpace(node.FirstChild.Data)

				if strings.Contains(snode, "window.__CLASS_IDS__") && strings.Contains(snode, "json_url=") {
					words := strings.Fields(snode)
					for _, word := range words {
						word, _ := url.QueryUnescape(word)
						if strings.Contains(word, "json_url=") && !strings.Contains(word, "arte_sitefactory") {
							parts := strings.Split(word, "\"")
							for _, part := range parts {
								if strings.Contains(part, "json_url=") {
									part := strings.SplitN(part, "=", 2)
									json_url = part[1]
									return
								}
							}
						}
					}
				}
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			crawler(child)
		}
	}

	crawler(doc)
	if json_url != "" {
		return json_url, nil
	}
	return "", errors.New("Missing <script> in the node tree")
}

type JsonVid struct {
	MimeType            string `json:"mimeType"`
	VersionProg         int    `json:"versionProg"`
	VersionLibelle      string `json:"versionLibelle"`
	Quality             string `json:"quality"`
	ID                  string `json:"id"`
	Width               int    `json:"width"`
	VersionShortLibelle string `json:"versionShortLibelle"`
	URL                 string `json:"url"`
	Height              int    `json:"height"`
	VersionCode         string `json:"versionCode"`
	Bitrate             int    `json:"bitrate"`
	MediaType           string `json:"mediaType"`
}

type JsonBody struct {
	VideoJSONPlayer struct {
		VSR map[string]JsonVid `json:"VSR"`
	} `json:"videoJsonPlayer"`
}

func RetrieveMpgUrl(json_url string) (string, string, error) {
	json_data_s := DownloadUrl(json_url)
	var json_data JsonBody
	json.Unmarshal([]byte(json_data_s), &json_data)

	json_data_sub := json_data.VideoJSONPlayer.VSR

	values := make(map[string]JsonVid, len(json_data_sub))
	for _, value := range json_data_sub {
		if value.Bitrate == 2200 && value.MediaType == "mp4" {
			values[value.VersionCode] = value
		}
	}

	var mpg_url string
	var version_code string
	known_codes := []string{"VF", "VO-STF", "VOF-STF", "VOA-STF", "VO"}

	for _, version_code = range known_codes {
		if val, ok := values[version_code]; ok {
			mpg_url = val.URL
			break
		}
	}

	if mpg_url == "" {
		fmt.Printf("%s\n", values)
		return "", "", errors.New("Could not find known <VersionCode>")
	}
	return mpg_url, version_code, nil
}

func DownloadMpg(filepath string, url string) error {
	// Get the data
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Create the file
	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	// Write the body to file
	_, err = io.Copy(out, resp.Body)
	return err
}

func main() {
	var conf Config
	err := conf.ParseConfig()

	if err != nil {
		return
	}
	if conf.debug {
		fmt.Printf("URL : %s\n", conf.url)
	}

	// retrieve body data and parse it to get the json url
	json_url, err := RetrieveJsonUrl(conf.url)
	if conf.debug {
		fmt.Printf("JSON url : %s\n", json_url)
	}

	// retrieve the json and parse it to get the mpg url
	mpg_url, version_code, err := RetrieveMpgUrl(json_url)
	if err != nil {
		return
	}
	if conf.debug {
		fmt.Printf("MPG url : %s\n", mpg_url)
	}

	// forge a name for the file
	i := strings.Split(conf.url, "/")
	name := i[6] + "-" + i[5] + "-" + version_code
	filepath := conf.destination + "/" + name + ".mp4"

	if conf.debug {
		fmt.Printf("DEST : %s\n", filepath)
	}

	DownloadMpg(filepath, mpg_url)
}
