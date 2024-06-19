package main

import (
	"flag"

	"github.com/sirupsen/logrus"

	"github.com/wuhan005/sayrud/internal/db"
	"github.com/wuhan005/sayrud/internal/route"
)

func main() {
	port := flag.Int("port", 8080, "port to listen")
	flag.Parse()

	db, err := db.Init()
	if err != nil {
		logrus.WithError(err).Fatal("Failed to initialize database")
	}

	f := route.New(db)
	f.Run(*port)
}
