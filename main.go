package main
import (
	"fmt"
	"os"
	"errors"
	"flag"
	"io/ioutil"
	"net/http"
	"net/url"
	"bytes"
	"golang.org/x/net/html"
	"io"
	"strings"
	"encoding/json"
)


var Default_destination string = "/tmp"


type Config struct {
	url string
	destination string
}


func (c *Config) ParseConfig() error {
	flag.StringVar(&c.url, "url", "", "Arte+7 url")
	flag.StringVar(&c.destination, "destination", "", "Where we put the video")

	flag.Parse()

	if len(c.url) == 0 {
		fmt.Fprintf(os.Stderr, "You must give an url\n")
		return errors.New("Missing argument url")
	}

	if len(c.destination) == 0 {
		c.destination = Default_destination
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
	MimeType			string `json:"mimeType"`
	VersionProg		 int	`json:"versionProg"`
	VersionLibelle	  string `json:"versionLibelle"`
	Quality			 string `json:"quality"`
	ID				  string `json:"id"`
	Width			   int	`json:"width"`
	VersionShortLibelle string `json:"versionShortLibelle"`
	URL				 string `json:"url"`
	Height			  int	`json:"height"`
	VersionCode		 string `json:"versionCode"`
	Bitrate			 int	`json:"bitrate"`
	MediaType		   string `json:"mediaType"`
}


type JsonBody struct {
	VideoJSONPlayer struct {
		VSR struct {
			HTTPSEQ2 JsonVid `json:"HTTPS_EQ_2"`
			HTTPSEQ1 JsonVid `json:"HTTPS_EQ_1"`
			HLSXQ2 JsonVid `json:"HLS_XQ_2"`
			HLSXQ1 JsonVid `json:"HLS_XQ_1"`
			HTTPSMQ1 JsonVid `json:"HTTPS_MQ_1"`
			HTTPSSQ1 JsonVid `json:"HTTPS_SQ_1"`
			HTTPSHQ1 JsonVid `json:"HTTPS_HQ_1"`
			HTTPSHQ2 JsonVid `json:"HTTPS_HQ_2"`
			HTTPSSQ2 JsonVid `json:"HTTPS_SQ_2"`
			HTTPSMQ2 JsonVid `json:"HTTPS_MQ_2"`
		} `json:"VSR"`
	} `json:"videoJsonPlayer"`
}


func RetrieveMpgUrl(json_url string) string {
	json_data_s := DownloadUrl(json_url)
	var json_data JsonBody
	json.Unmarshal([]byte(json_data_s), &json_data)

	json_data_sub := json_data.VideoJSONPlayer.VSR

	var mpg_url string
	switch {
		case json_data_sub.HTTPSSQ1 != JsonVid{}:
			mpg_url = json_data_sub.HTTPSSQ1.URL
		case json_data_sub.HTTPSSQ2 != JsonVid{}:
			mpg_url = json_data_sub.HTTPSSQ2.URL
	}
        return mpg_url
}


/*func DisplayProgress(progress chan int) {
    for i := range progress {
        fmt.Printf("\r", "*" * i)
    }
}*/


func percent(total_size int, current_size int) int {
    return 100 * current_size / total_size
}


func DownloadMpg(filepath string, url string) error {
//func DownloadMpg(filepath string, url string, progress chan int) error {
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
        fmt.Printf("URL : %s\n", conf.url)

	// retrieve body data and parse it to get the json url
	json_url, err := RetrieveJsonUrl(conf.url)
	fmt.Printf("JSON url : %s\n", json_url)

	// retrieve the json and parse it to get the mpg url
        mpg_url := RetrieveMpgUrl(json_url)
	fmt.Printf("MPG url : %s\n", mpg_url)

        // retrieve the mpg file with a nice progress bar
//        progress := make(chan int)

        i := strings.Split(conf.url, "/")
        name := i[6] + "-" + i[5]
        filepath := conf.destination + "/" + name + ".mp4"

        fmt.Printf("DEST : %s\n", filepath)

//        go DisplayProgress(progress)
        DownloadMpg(filepath, mpg_url) //, progress)
}
