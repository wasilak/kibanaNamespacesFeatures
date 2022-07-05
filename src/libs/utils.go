package libs

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/fatih/color"
	diff "github.com/r3labs/diff/v2"
)

var wg sync.WaitGroup
var wgCluster sync.WaitGroup

type ClusterDetails struct {
	Address          string   `json:"address" yaml:"address"`
	Username         string   `json:"username"`
	Password         string   `json:"password"`
	ExcludedSpaces   []string `json:"excludedSpaces"`
	DisabledFeatures []string `json:"disabledFeatures"`
}

type Event []ClusterDetails

type CmdEvent struct {
	Clusters []ClusterDetails `yaml:"clusters"`
}

type Result struct {
	Space  string `json:"space"`
	Result string `json:"result"`
}

type Results []Result

type ClusterResult struct {
	Address string  `json:"address"`
	Results Results `json:"results"`
}

type AllResults []ClusterResult

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

func updateSpace(event ClusterDetails, space Space, wg *sync.WaitGroup, spacesChannel chan SpaceChange) {
	defer wg.Done()

	changelog, _ := diff.Diff(event.DisabledFeatures, space.DisabledFeatures)

	spaceChange := SpaceChange{
		Space:     space,
		Changelog: changelog,
	}

	if len(changelog) > 0 {

		space.DisabledFeatures = event.DisabledFeatures

		requestBody, err := json.Marshal(space)
		if err != nil {
			panic(err)
		}

		client := http.Client{Timeout: time.Second * 10}

		req, err := http.NewRequest("PUT", event.Address+"/api/spaces/space/"+space.ID, bytes.NewBuffer(requestBody))
		req.Header.Add("Authorization", "Basic "+basicAuth(event.Username, event.Password))
		req.Header.Add("Content-Type", "application/json")
		req.Header.Add("kbn-xsrf", "spaces updater")

		if err != nil {
			log.Fatal("Error reading request. ", err)
		}

		resp, err := client.Do(req)
		if err != nil {
			log.Fatal("Error reading response. ", err)
		}

		defer resp.Body.Close()
	}

	spacesChannel <- spaceChange
}

func HandleRequest(ctx context.Context, event []ClusterDetails) (AllResults, error) {
	clusterResultChannel := make(chan ClusterResult)

	for _, cluster := range event {
		wgCluster.Add(1)
		go HandleCluster(&wgCluster, cluster, clusterResultChannel)
	}

	go func() {
		wgCluster.Wait()
		close(clusterResultChannel)
	}()

	var allResults AllResults

	for clusterResult := range clusterResultChannel {
		allResults = append(allResults, clusterResult)
	}

	return allResults, nil
}

func HandleCluster(wgCluster *sync.WaitGroup, event ClusterDetails, clusterResultChannel chan ClusterResult) (Results, error) {
	defer wgCluster.Done()

	client := &http.Client{}

	req, err := http.NewRequest("GET", event.Address+"/api/spaces/space", nil)
	req.Header.Add("Authorization", "Basic "+basicAuth(event.Username, event.Password))
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

		if !stringInSlice(space.ID, event.ExcludedSpaces) {
			wg.Add(1)
			go updateSpace(event, space, &wg, spacesChannel)
		}
	}

	go func() {
		wg.Wait()
		close(spacesChannel)
	}()

	var results Results

	for space := range spacesChannel {
		if len(space.Changelog) == 0 {
			log.Printf("%s %s\n", space.Space.ID, color.GreenString("✓"))
			res := Result{
				Space:  space.Space.ID,
				Result: "0 changes",
			}
			results = append(results, res)
		} else {
			changes := ""
			for _, change := range space.Changelog {
				changes = changes + fmt.Sprintf("[%+v -> %s] ", change, change.Type)
			}
			log.Printf("%s %s\n", space.Space.ID, color.MagentaString("changes: %s\n", changes))
			res := Result{
				Space:  space.Space.ID,
				Result: changes,
			}
			results = append(results, res)
		}
	}

	clusterResults := ClusterResult{
		Address: event.Address,
		Results: results,
	}

	clusterResultChannel <- clusterResults

	return results, nil
}
