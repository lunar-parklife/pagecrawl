/*
 *   Copyright (C) 2023  Luna
 *
 *   This program is free software: you can redistribute it and/or modify
 *   it under the terms of the GNU General Public License as published by
 *   the Free Software Foundation, either version 3 of the License, or
 *   (at your option) any later version.
 *
 *   This program is distributed in the hope that it will be useful,
 *   but WITHOUT ANY WARRANTY; without even the implied warranty of
 *   MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 *   GNU General Public License for more details.
 *
 *   You should have received a copy of the GNU General Public License
 *   along with this program.  If not, see <https://www.gnu.org/licenses/>.
 */

package main

import (
	"bufio"
	"bytes"
	_ "embed"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/spf13/viper"
	"golang.org/x/net/html"
)

var (
	//go:embed help-info.txt
	helpInfo string
	//go:embed license-info.txt
	licenseInfo string
)

const userAgent = "pagecrawl; 0.1.0"

var (
	shouldCache = false
	outputs     = make([]io.Writer, 0)
)

type asset struct {
	Accessed   time.Time `json:"accessed"`
	Address    string    `json:"address"`
	Data       []byte    `json:"data"`
	References []string  `json:"references"`
}

type httpOutput struct {
	sendTo string
}

func (this *httpOutput) Write(p []byte) (int, error) {
	client := http.DefaultClient
	request, err := http.NewRequest(http.MethodGet, this.sendTo, bytes.NewReader(p))
	if err != nil {
		log.Println(fmt.Sprintf("Cannot create output request: %s", err.Error()))
		return 0, err
	}
	request.Header.Add("From", viper.GetString("Network.From"))
	request.Header.Add("User-Agent", userAgent)
	_, err = client.Do(request)
	if err != nil {
		return 0, err
	}
	return len(p), nil
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
	log.Println(fmt.Sprintf("Fetching from %s", where))
	now := time.Now().UTC()
	client := http.DefaultClient
	request, err := http.NewRequest(http.MethodGet, where, nil)
	if err != nil {
		log.Println(fmt.Sprintf("Error creating creating request for page %s: %s", where, err.Error()))
	}
	request.Header.Add("From", viper.GetString("Network.From"))
	request.Header.Add("User-Agent", userAgent)
	response, err := client.Do(request)
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
	asset := &asset{
		Accessed:   now,
		Address:    where,
		References: referenceNodes,
	}
	if shouldCache {
		asset.Data = rawResponse
	}
	rawAssetJson, err := json.Marshal(asset)
	for _, nextOutput := range outputs {
		_, err = nextOutput.Write(rawAssetJson)
		if err != nil {
			log.Println(fmt.Sprintf("Error outputting asset %+v: %s", asset, err.Error()))
			return
		}
	}
	log.Println(fmt.Sprintf("Sucessfully fetched %s", where))
}

func initConfig() {
	viper.AddConfigPath(".")
	viper.SetConfigFile("pagecrawl-config.ini")
	viper.SetConfigType("ini")
	viper.SetDefault("Log.Path", ".")
	viper.SetDefault("Log.Name", "pagecrawl")
	viper.SetDefault("Network.From", "")
	viper.SetDefault("Output.Kind", "stdout")
	viper.SetDefault("Output.Path", "")
	err := viper.ReadInConfig()
	if err != nil {
		viper.WriteConfig()
	}
}

func initLog() {
	logStart := time.Now().UTC()
	logPath := viper.GetString("Log.Path")
	logName := viper.GetString("Log.Name")
	// This is evil. I'm sorry.
	logTarget := fmt.Sprintf("%s/%s-%d-%d-%d-%d:%d.log",
		logPath, logName,
		logStart.Year(), logStart.Month(), logStart.Day(),
		logStart.Hour(), logStart.Minute())
	logFile, err := os.Create(logTarget)
	if err != nil {
		panic(err.Error())
	}
	log.SetOutput(logFile)
}

func main() {
	initConfig()
	initLog()
	for _, nextFlag := range flag.Args() {
		flag := strings.ToLower(nextFlag)
		switch flag {
		case "-c":
			shouldCache = true
			continue
		case "-h":
			log.Println(helpInfo)
			continue
		case "-l":
			log.Println(licenseInfo)
			continue
		case "-v":
			log.Println("PageCrawl pre-release")
			continue
		}
		exploded := strings.Split(flag, "=")
		switch exploded[0] {
		case "--out-file":
			explodedPaths := strings.Split(exploded[1], ",")
			for _, nextPath := range explodedPaths {
				nextFile, err := os.OpenFile(nextPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, os.ModeAppend)
				if err != nil {
					log.Printf("Error opening output file %s: %s", nextPath, err.Error())
					continue
				}
				outputs = append(outputs, nextFile)
			}
		case "--out-url":
			explodedPaths := strings.Split(exploded[1], ",")
			for _, nextPath := range explodedPaths {
				outputs = append(outputs, &httpOutput{
					sendTo: nextPath,
				})
			}
		}
	}
	input := bufio.NewScanner(os.Stdin)
	group := &sync.WaitGroup{}
	for input.Scan() {
		if input.Err() != nil {
			if input.Err() == io.EOF {
				break
			}
			log.Println(fmt.Sprintf("Error scanning input: %s", input.Err().Error()))
			continue
		}
		nextLine := input.Text()
		if nextLine == "quit" {
			break
		}
		group.Add(1)
		go fetch(nextLine, group)
	}
	group.Wait()
}
