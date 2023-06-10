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
	_ "embed"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
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
	log.Println(fmt.Sprintf("Fetching from %s", where))
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
				viper.GetString("Network.From"),
			},
			"User-Agent": {
				"pagecrawl",
				"0.1.0",
				"https://github.com/lunar-parklife/pagecrawl",
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
	_, err = os.Stdout.Write(rawAssetJson)
	if err != nil {
		log.Println(fmt.Sprintf("Error outputting asset %+v: %s", asset, err.Error()))
		return
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
		case "-h":
			log.Println(helpInfo)
		case "-l":
			log.Println(licenseInfo)
		case "-v":
			log.Println("PageCrawl pre-release")
		}
	}
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
