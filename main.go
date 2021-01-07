package main

import (
	"encoding/csv"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/cheggaaa/pb/v3"
	"github.com/dghubble/oauth1"
	"github.com/fatih/color"
	"github.com/kivikakk/go-twitter/twitter"
	lediscfg "github.com/ledisdb/ledisdb/config"
	"github.com/ledisdb/ledisdb/ledis"
	"github.com/robfig/cron"
)

var (
	l          *ledis.DB
	conf       *config
	cyan       *color.Color = color.New(color.FgCyan)
	yellow     *color.Color = color.New(color.FgYellow)
	red        *color.Color = color.New(color.FgRed)
	location   *time.Location
	client     *twitter.Client
	addr       *string
	configPath *string
	importPath *string
	err        error
)

func init() {
	addr = flag.String("addr", ":1323", "TCP address to listen to")
	configPath = flag.String("conf", "config.toml", "Config File Path")
	importPath = flag.String("import", "", "Import CSV Path")
	flag.Parse()
}

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
		ExcludeUsers     []string `toml:"ExcludeUsers"`
		MaxFollowers     int      `toml:"MaxFollowers"`
		MaxFFRatio       float64  `toml:"MaxFFRatio"`
		TweetTextFormat  string   `toml:"TweetTextFormat"`
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

func init() {
	if client, conf, err = loadConfigFrom(*configPath); err != nil {
		red.Printf("[ERROR] ")
		log.Printf("Could not parse config file: %v\n", err)
		os.Exit(1)
	}
	eu := []string{}
	for _, s := range conf.Twitter.ExcludeUsers {
		eu = append(eu, strings.ToLower(strings.ReplaceAll(s, "@", "")))
	}
	conf.Twitter.ExcludeUsers = eu
}

func initLedis() {
	cfg := lediscfg.NewConfigDefault()
	cfg.DataDir = "data/ledis"
	ldb, err := ledis.Open(cfg)
	if err != nil {
		log.Fatal(err)
	}
	l, err = ldb.Select(0)
	if err != nil {
		log.Fatal(err)
	}
}

func needsToSkip(screenName string) bool {
	screenName = strings.ToLower(screenName)
	for _, n := range conf.Twitter.ExcludeUsers {
		if n == screenName {
			return true
		}
	}
	return false
}

func ffRatio(follows, followers int) float64 {
	if follows == 0 || followers == 0 {
		return 0
	}
	return float64(followers) / float64(follows)
}

func block(id int64) {
	var stat int64
	if stat, err = l.SIsMember([]byte("blocked"), []byte(fmt.Sprintf("%d", id))); err != nil {
		red.Printf("[ERROR] ")
		log.Fatal(err)
	}
	if stat != 0 {
		client.Block.Create(&twitter.BlockCreateParams{UserID: id})
		if _, err := l.SAdd([]byte("blocked"), []byte(fmt.Sprintf("%d", id))); err != nil {
			red.Printf("[ERROR] ")
			log.Fatal(err)
		}
	}
}

func blockUser(tw *twitter.Tweet) {
	if !tw.User.Following || conf.Twitter.BlockIfFollowing || needsToSkip(tw.User.ScreenName) {
		if tw.User.FollowersCount < conf.Twitter.MaxFollowers || ffRatio(tw.User.FriendsCount, tw.User.FollowersCount) < conf.Twitter.MaxFFRatio {
			block(tw.User.ID)
		}
	}
	cyan.Printf("[BLOCK] ")
	log.Printf("@%-15s | %-20d | %s\n", tw.User.ScreenName, tw.User.ID, tw.User.Name)
}

func blockFromCSV() {
	var (
		file       *os.File
		reader     *csv.Reader
		line       [][]string
		id         int
		count      int
		lineLength int
		run        string
	)
	if file, err = os.Open(*importPath); err != nil {
		red.Printf("[ERROR] ")
		log.Fatal(err)
	}
	defer file.Close()

	reader = csv.NewReader(file)

	if line, err = reader.ReadAll(); err != nil {
		red.Printf("[ERROR] ")
		log.Fatal(err)
	}

	lineLength = len(line)

	fmt.Printf("Will it take %d second to do. Run it? [y/N] ", lineLength/5)
	fmt.Scanf("%s", &run)

	if strings.ToLower(run) != "y" {
		return
	}
	bar := pb.Simple.Start(lineLength)
	bar.SetMaxWidth(80)
	for _, l := range line {
		id, _ = strconv.Atoi(l[0])
		block(int64(id))
		time.Sleep(20 * time.Millisecond)
		bar.Increment()
		count++
	}
	bar.Finish()
	cyan.Printf("[BLOCK] ")
	log.Printf("%d users are blocked.\n", count)
}

func twitterBlocker() {
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
		switch m.(type) {
		case *twitter.Tweet:
			blockUser(m.(*twitter.Tweet))
		case *twitter.StreamLimit:
			log.Fatal(m.(*twitter.StreamLimit).Track)
		}
	}
}

func blockedHandler(w http.ResponseWriter, r *http.Request) {
	var (
		byteArrArr [][]byte
		idArr      []string
	)
	if byteArrArr, err = l.SMembers([]byte("blocked")); err != nil {
		log.Fatal(err)
	}
	for _, sbyte := range byteArrArr {
		idArr = append(idArr, string(sbyte))
	}
	w.Header().Set("Content-Disposition", "attachment; filename=blocked.csv")
	w.Header().Set("Content-Type", "text/csv")
	fmt.Fprintln(w, strings.Join(idArr, "\n"))
}

func blockWordHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Disposition", "attachment; filename=words.csv")
	w.Header().Set("Content-Type", "text/csv")
	fmt.Fprintln(w, strings.Join(conf.Twitter.SearchWords, "\n"))
}

func blockCountTweet() {
	var (
		previousDayCountByte []byte
		previousDayCount     int
		count                int64
		text                 string
	)
	if count, err = l.SCard([]byte("blocked")); err != nil {
		log.Fatal(err)
	}
	if previousDayCountByte, err = l.Get([]byte("previousDay")); err != nil {
		log.Fatal(err)
	}
	if string(previousDayCountByte) == "" {
		previousDayCountByte = []byte("0")
	}
	if previousDayCount, err = strconv.Atoi(string(previousDayCountByte)); err != nil {
		log.Fatal(err)
	}
	// Tweet text format
	text = conf.Twitter.TweetTextFormat
	text = strings.ReplaceAll(text, "{% newBlockCount %}", fmt.Sprintf("%d", count))
	text = strings.ReplaceAll(text, "{% increaseCount %}", fmt.Sprintf("%d", int(count)-previousDayCount))
	text = strings.ReplaceAll(text, "{% nowDateTime %}", time.Now().Format("2006/1/2 15:04:05"))
	if _, _, err = client.Statuses.Update(text, nil); err != nil {
		log.Fatal(err)
	}
	if err = l.Set([]byte("previousDay"), []byte(fmt.Sprintf("%d", count))); err != nil {
		log.Fatal(err)
	}
}

func main() {
	log.SetFlags(0)
	if location, err = time.LoadLocation("Asia/Tokyo"); err != nil {
		red.Printf("[ERROR] ")
		log.Fatal(err)
	}
	initLedis()
	if *importPath != "" {
		blockFromCSV()
		return
	}
	if conf.Twitter.TweetTextFormat != "" {
		c := cron.NewWithLocation(location)
		c.AddFunc("0 0 12 * * *", cron.FuncJob(blockCountTweet))
		c.Start()
	}
	http.HandleFunc("/blocked.csv", blockedHandler)
	http.HandleFunc("/words.csv", blockWordHandler)
	go http.ListenAndServe(*addr, nil)
	twitterBlocker()
}
