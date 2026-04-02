package main

import (
	"flag"
	"log"

	appconfig "arcee/config"
)

func main() {
	configFile := flag.String("config", appconfig.DefaultConfigPath, "path to config file")
	mode := flag.String("mode", "", "run mode: signup or serve")
	flag.Parse()

	cfg, err := appconfig.Load(*configFile)
	if err != nil {
		log.Fatal(err)
	}

	switch cfg.ResolvedMode(*mode) {
	case "serve":
		runServer(cfg)
	default:
		runSignup(cfg)
	}
}
