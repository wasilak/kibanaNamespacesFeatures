package main

import (
	"context"
	// "encoding/json"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"github.com/wasilak/kibanaSpacesFeatures/libs"
)

var event libs.CmdEvent

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

	err := viper.Unmarshal(&event)
	if err != nil {
		log.Fatalf("unable to decode into struct, %v", err)
	}
}

func main() {
	initParams()

	ctx := context.Background()

	libs.HandleRequest(ctx, event.Clusters)
}
