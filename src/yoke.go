package main

import (
	"container/list"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"runtime"
	"strings"
)

const (
	defaultConfigFile = "yoke_config.json"
	profileFileName   = "yoke_profile.json"
)

const (
	commandRun int = iota
	commandCreate
	commandList
)

var config struct {
	DefaultProfile testProfile `json:"defaultProfile"`
	Maxthreads     int         `json:maxthreads`
	Prefix         string      `json:"prefix"`
}

var options struct {
	verbose      *bool
	showInfo     *bool
	showWarnings *bool
	version      *bool
}

func main() {
	args := flag.Args()
	var command string
	if len(args) > 0 {
		command = args[0]
	} else {
		command = "run"
	}

	tests := list.New()

	// Build list of tests
	testFiles, _ := ioutil.ReadDir("./")
	for _, f := range testFiles {
		if f.IsDir() && strings.HasPrefix(f.Name(), config.Prefix) {
			if *options.verbose {
				fmt.Println("Test found: " + f.Name())
			}
			t := newTest(f.Name())
			tests.PushBack(t)
		}
	}

	numConcurrent := 0

	// Handle tests
	// Run concurrent tests
	c := make(chan bool, config.Maxthreads)
	done := make(chan bool)
	for e := tests.Front(); e != nil; e = e.Next() {
		var currTest *test
		currTest = e.Value.(*test)
		if currTest.profile.Noconcurrent != nil && !*currTest.profile.Noconcurrent {
			c <- true // If there are > maxthreads running, wait for one to finish
			go currTest.runInThread(c, done)
			numConcurrent++
		}
	}
	// Wait until all concurrent tests are done
	for i := 0; i < numConcurrent; i++ {
		<-done
	}
	// Run non-concurrent tests
	for e := tests.Front(); e != nil; e = e.Next() {
		var currTest *test
		currTest = e.Value.(*test)
		if currTest.profile.Noconcurrent != nil && *currTest.profile.Noconcurrent {
			currTest.run()
		}
	}

	for e := tests.Front(); e != nil; e = e.Next() {
		var currTest *test
		currTest = e.Value.(*test)
		currTest.results.print()
		// fmt.Println(currTest.profile.String())
	}

}

func init() {
	options.verbose = flag.Bool("verbose", false, "show all output")
	options.showInfo = flag.Bool("info", false, "show info output")
	options.showWarnings = flag.Bool("warnings", false, "show warnings")
	options.version = flag.Bool("version", false, "show version and exit")
	flag.Parse()

	if *options.verbose {
		*options.showInfo = true
		*options.showWarnings = true
	}

	if *options.version {
		fmt.Println("Yoke v0.9 by mhweaver")
		os.Exit(0)
	}

	// Load/parse default config file
	configFile, err := os.Stat(defaultConfigFile)
	if err != nil || configFile.IsDir() {
		log.Fatal("No default configuration file found: ", err)
	}

	configBytes, err := ioutil.ReadFile(configFile.Name())
	if err != nil {
		log.Fatal("Unable to read config file: ", err)
	}

	err = json.Unmarshal(configBytes, &config)
	if err != nil {
		log.Fatal("Unable to unmarshal default config JSON: ", err)
	}

	config.DefaultProfile.fixNullReferences()

	if config.Maxthreads <= 0 {
		config.Maxthreads = runtime.NumCPU()
	}
	runtime.GOMAXPROCS(config.Maxthreads)

}
