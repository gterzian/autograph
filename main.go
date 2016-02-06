// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// Contributor: Julien Vehent jvehent@mozilla.com [:ulfr]

package main

//go:generate ./version.sh

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"

	"github.com/mozilla-services/go-mozlog"

	"gopkg.in/yaml.v2"
)

func init() {
	// initialize the logger
	mozlog.Logger.LoggerName = "Autograph"
}

// configuration loads a yaml file that contains the configuration of Autograph
type configuration struct {
	Server struct {
		Listen         string
		NonceCacheSize int
	}
	Signers        []signer
	Authorizations []authorization
}

func main() {
	var (
		ag          *autographer
		conf        configuration
		cfgFile     string
		showVersion bool
		err         error
	)
	flag.StringVar(&cfgFile, "c", "autograph.yaml", "Path to configuration file")
	flag.BoolVar(&showVersion, "V", false, "Show build version and exit")
	flag.Parse()

	if showVersion {
		fmt.Println(version)
		os.Exit(0)
	}

	err = conf.loadFromFile(cfgFile)
	if err != nil {
		log.Fatal(err)
	}

	// initialize signers from the configuration
	// and store them into the autographer handler
	ag, err = newAutographer(conf.Server.NonceCacheSize)
	if err != nil {
		log.Fatal(err)
	}
	ag.addSigners(conf.Signers)
	ag.addAuthorizations(conf.Authorizations)
	ag.makeSignerIndex()

	// start serving
	mux := http.NewServeMux()
	mux.HandleFunc("/__heartbeat__", ag.handleHeartbeat)
	mux.HandleFunc("/signature", ag.handleSignature)
	server := &http.Server{
		Addr:    conf.Server.Listen,
		Handler: mux,
	}
	log.Println("Starting Autograph API on", conf.Server.Listen)
	err = server.ListenAndServe()
	if err != nil {
		log.Fatal(err)
	}
}

func (c *configuration) loadFromFile(path string) error {
	fd, err := ioutil.ReadFile(path)
	if err != nil {
		return err
	}
	err = yaml.Unmarshal(fd, &c)
	if err != nil {
		return err
	}
	return nil
}