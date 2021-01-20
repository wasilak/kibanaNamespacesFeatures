package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/fatih/color"
	diff "github.com/r3labs/diff/v2"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

var kibanaAddress string
var disabledFeatures []string
var excludedSpaces []string
var wg sync.WaitGroup

// Space type
type Space struct {
	ID               string   `json:"id"`
	Name             string   `json:"name"`
	Description      string   `json:"description,omitempty"`
	Color            string   `json:"color,omitempty"`
	Initials         string   `json:"initials,omitempty"`
	DisabledFeatures []string `json:"disabledFeatures,omitempty"`
	ImageURL         string   `json:"imageUrl,omitempty"`
}

// SpaceChange type
type SpaceChange struct {
	Space     Space
	Changelog diff.Changelog
}

func stringInSlice(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}

func updateSpace(kibanaAddress string, space Space, wg *sync.WaitGroup, spacesChannel chan SpaceChange) {
	defer wg.Done()

	changelog, _ := diff.Diff(disabledFeatures, space.DisabledFeatures)

	spaceChange := SpaceChange{
		Space:     space,
		Changelog: changelog,
	}

	if len(changelog) > 0 {

		space.DisabledFeatures = disabledFeatures

		requestBody, err := json.Marshal(space)
		if err != nil {
			panic(err)
		}

		req, err := http.NewRequest("PUT", kibanaAddress+"/api/spaces/space/"+space.ID, bytes.NewBuffer(requestBody))
		if err != nil {
			log.Fatal("Error reading request. ", err)
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("kbn-xsrf", "spaces updater")

		client := http.Client{Timeout: time.Second * 10}

		resp, err := client.Do(req)
		if err != nil {
			log.Fatal("Error reading response. ", err)
		}

		// body, err := ioutil.ReadAll(resp.Body)
		// if err != nil {
		// 	panic(err)
		// }
		// fmt.Printf("%s :: %s", space.ID, string(body))

		defer resp.Body.Close()
	}

	spacesChannel <- spaceChange
}

func initParams() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [kibana address]\n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.String("config", "./", "path to kibanaSpacesFeatures.yml")
	flag.String("address", "http://localhost:5601", "Kibana API endpoint")

	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)
	pflag.Parse()
	viper.BindPFlags(pflag.CommandLine)

	viper.SetConfigName("kibanaSpacesFeatures")    // name of config file (without extension)
	viper.SetConfigType("yaml")                    // REQUIRED if the config file does not have the extension in the name
	viper.AddConfigPath(viper.GetString("config")) // path to look for the config file in
	viperErr := viper.ReadInConfig()               // Find and read the config file

	if viperErr != nil { // Handle errors reading the config file
		log.Fatal(viperErr)
		panic(viperErr)
	}

	kibanaAddress = viper.GetString("address")
	excludedSpaces = viper.GetStringSlice("excludedSpaces")
	disabledFeatures = viper.GetStringSlice("disabledFeatures")
}

func main() {

	initParams()

	resp, err := http.Get(kibanaAddress + "/api/spaces/space")

	if err != nil {
		panic(err)
	}

	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)

	if err != nil {
		panic(err)
	}

	var spaces []Space

	spacesChannel := make(chan SpaceChange)

	if err := json.Unmarshal(body, &spaces); err != nil {
		panic(err)
	}

	for _, space := range spaces {

		if !stringInSlice(space.ID, excludedSpaces) {
			wg.Add(1)
			go updateSpace(kibanaAddress, space, &wg, spacesChannel)
		}
	}

	go func() {
		wg.Wait()
		close(spacesChannel)
	}()

	for space := range spacesChannel {
		fmt.Printf("%s ", space.Space.ID)
		if len(space.Changelog) == 0 {
			color.Green("✓")
		} else {
			changes := ""
			for _, change := range space.Changelog {
				changes = changes + fmt.Sprintf("[%s -> %s] ", change.From, change.Type)
			}
			color.Magenta("changes: %s\n", changes)
		}
	}
}
