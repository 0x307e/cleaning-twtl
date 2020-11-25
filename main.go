package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/BurntSushi/toml"
	"github.com/dghubble/oauth1"
	"github.com/fatih/color"
	"github.com/kivikakk/go-twitter/twitter"
)

type config struct {
	TwitterService struct {
		APIKey            string   `toml:"APIKey"`
		APISecretKey      string   `toml:"APISecretKey"`
		AccessToken       string   `toml:"AccessToken"`
		AccessTokenSecret string   `toml:"AccessTokenSecret"`
		Protect           []string `toml:"Protect"`
	} `toml:"TwitterService"`
	Twitter struct {
		BlockIfFollowing bool     `toml:"BlockIfFollowing"`
		SearchWords      []string `toml:"SearchWords"`
	} `toml:"Twitter"`
}

func loadConfigFrom(configFile string) (client *twitter.Client, c *config, err error) {
	if _, err := toml.DecodeFile(configFile, &c); err != nil {
		log.Fatal(err)
	}

	twitterConfig := oauth1.NewConfig(
		c.TwitterService.APIKey,
		c.TwitterService.APISecretKey,
	)
	token := oauth1.NewToken(
		c.TwitterService.AccessToken,
		c.TwitterService.AccessTokenSecret,
	)
	httpClient := twitterConfig.Client(oauth1.NoContext, token)
	client = twitter.NewClient(httpClient)
	return
}

func main() {
	var (
		client *twitter.Client
		conf   *config
		cyan   *color.Color = color.New(color.FgCyan)
		yellow *color.Color = color.New(color.FgYellow)
		red    *color.Color = color.New(color.FgRed)
		err    error
	)
	if client, conf, err = loadConfigFrom(os.Args[1]); err != nil {
		red.Printf("[ERROR] ")
		log.Printf("Could not parse config file: %v\n", err)
		os.Exit(1)
	}

	yellow.Println("Starting Stream...")

	filterParams := &twitter.StreamFilterParams{
		Track:         conf.Twitter.SearchWords,
		StallWarnings: twitter.Bool(true),
	}
	stream, err := client.Streams.Filter(filterParams)
	if err != nil {
		red.Printf("[ERROR] ")
		log.Fatal(err)
	}

	for m := range stream.Messages {
		tw := m.(*twitter.Tweet)
		if !tw.User.Following || conf.Twitter.BlockIfFollowing {
			client.Block.Create(&twitter.BlockCreateParams{
				UserID: tw.User.ID,
			})
		}
		cyan.Printf("[BLOCK] ")
		log.Printf("%s [@%s] (%d)\n", tw.User.Name, tw.User.ScreenName, tw.User.ID)
	}

	ch := make(chan os.Signal)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)

	yellow.Println("Stopping Stream...")
	stream.Stop()
}
