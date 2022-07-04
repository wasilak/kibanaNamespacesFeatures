package main

import (
	"bytes"
	"encoding/base64"
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
var username string
var password string
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

func basicAuth(username, password string) string {
	auth := username + ":" + password
	return base64.StdEncoding.EncodeToString([]byte(auth))
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

		client := http.Client{Timeout: time.Second * 10}

		req, err := http.NewRequest("PUT", kibanaAddress+"/api/spaces/space/"+space.ID, bytes.NewBuffer(requestBody))
		req.Header.Add("Authorization", "Basic "+basicAuth(username, password))
		req.Header.Add("Content-Type", "application/json")
		req.Header.Add("kbn-xsrf", "spaces updater")

		if err != nil {
			log.Fatal("Error reading request. ", err)
		}

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
	flag.String("username", "username", "username")
	flag.String("password", "password", "password")

	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)
	pflag.Parse()
	viper.BindPFlags(pflag.CommandLine)

	viper.SetConfigName("kibanaSpacesFeatures")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(viper.GetString("config"))
	viper.SetEnvPrefix("kibanaSpacesFeatures")
	viper.AutomaticEnv()

	viperErr := viper.ReadInConfig()

	if viperErr != nil {
		log.Fatal(viperErr)
		panic(viperErr)
	}

	kibanaAddress = viper.GetString("address")
	username = viper.GetString("username")
	password = viper.GetString("password")
	excludedSpaces = viper.GetStringSlice("excludedSpaces")
	disabledFeatures = viper.GetStringSlice("disabledFeatures")
}

func main() {

	initParams()

	client := &http.Client{}

	req, err := http.NewRequest("GET", kibanaAddress+"/api/spaces/space", nil)
	req.Header.Add("Authorization", "Basic "+basicAuth(username, password))
	resp, err := client.Do(req)

	if err != nil {
		panic(err)
	}

	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)

	if err != nil {
		panic(err)
	}

	// log.Println(string(body))

	var spaces []Space

	spacesChannel := make(chan SpaceChange)

	if err := json.Unmarshal(body, &spaces); err != nil {
		log.Println(string(body))
		log.Fatal(err)
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
		if len(space.Changelog) == 0 {
			log.Printf("%s %s\n", space.Space.ID, color.GreenString("✓"))
		} else {
			changes := ""
			for _, change := range space.Changelog {
				changes = changes + fmt.Sprintf("[%s -> %s] ", change.From, change.Type)
			}
			color.Magenta("changes: %s\n", changes)
		}
	}
}
