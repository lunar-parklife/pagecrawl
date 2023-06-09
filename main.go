package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/html"
)

type Asset struct {
	Accessed   time.Time `json:"accessed"`
	Address    string    `json:"address"`
	Data       []byte    `json:"data"`
	References []string  `json:"references"`
}

func crawl(doc *html.Node) []string {
	buf := make([]string, 0)
	for _, attribute := range doc.Attr {
		if strings.ToLower(attribute.Key) == "href" {
			buf = append(buf, attribute.Val)
		}
	}
	for next := doc.FirstChild; next != nil; next = next.NextSibling {
		buf = append(buf, crawl(next)...)
	}
	return buf
}

func fetch(where string, group *sync.WaitGroup) {
	defer group.Done()
	now := time.Now().UTC()
	client := http.DefaultClient
	url, err := url.Parse(where)
	if err != nil {
		log.Println(err.Error())
		return
	}
	response, err := client.Do(&http.Request{
		Method: http.MethodGet,
		Header: http.Header{
			"From": {
				//
			},
		},
		URL: url,
	})
	if err != nil {
		log.Println(fmt.Sprintf("Error fetching %s: %s", where, err.Error()))
		return
	}
	defer response.Body.Close()
	rawResponse, err := io.ReadAll(response.Body)
	if err != nil {
		log.Println(fmt.Sprintf("Error reading response: %s", err.Error()))
		return
	}
	doc, err := html.Parse(strings.NewReader(string(rawResponse)))
	referenceNodes := crawl(doc)
	asset := &Asset{
		Accessed:   now,
		Address:    where,
		Data:       rawResponse,
		References: referenceNodes,
	}
	rawAssetJson, err := json.Marshal(asset)
	_, err = os.Stdin.Write(rawAssetJson)
	if err != nil {
		log.Println(fmt.Sprintf("Error outputting asset %+v: %s", asset, err.Error()))
		return
	}
}

func main() {
	input := bufio.NewScanner(os.Stdin)
	group := &sync.WaitGroup{}
	for input.Scan() {
		if input.Err() != nil {
			log.Println(fmt.Sprintf("Error scanning input: %s", input.Err().Error()))
			continue
		}
		group.Add(1)
		go fetch(input.Text(), group)
	}
	group.Wait()
}
