package main

import (
	"encoding/csv"
	"fmt"
	"log"
	"os"

	lediscfg "github.com/ledisdb/ledisdb/config"
	"github.com/ledisdb/ledisdb/ledis"
)

var l *ledis.DB

func initLedis() {
	cfg := lediscfg.NewConfigDefault()
	cfg.DataDir = "../data/ledis"
	ldb, err := ledis.Open(cfg)
	if err != nil {
		log.Fatal(err)
	}
	l, err = ldb.Select(0)
	if err != nil {
		log.Fatal(err)
	}
}

func main() {
	initLedis()
	var (
		err        error
		file       *os.File
		byteArrArr [][]byte
		idArr      []string
	)

	if len(os.Args) > 1 {
		if file, err = os.OpenFile(os.Args[1], os.O_WRONLY|os.O_CREATE, 0600); err != nil {
			log.Fatal(err)
		}
	}

	if byteArrArr, err = l.SMembers([]byte("blocked")); err != nil {
		log.Fatal(err)
	}
	for _, sbyte := range byteArrArr {
		idArr = append(idArr, string(sbyte))
	}

	if file != nil {
		writer := csv.NewWriter(file)
		for _, id := range idArr {
			writer.Write([]string{id})
		}
		writer.Flush()
	} else {
		for _, id := range idArr {
			fmt.Println(id)
		}
	}
	defer file.Close()
}
