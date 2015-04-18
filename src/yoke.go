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
	"sync"
)

const (
	defaultConfigFile = "yoke_config.json"
)

var config struct {
	DefaultProfile testProfile `json:"defaultProfile"`
	Maxthreads     int         `json:maxthreads`
	Prefix         string      `json:"prefix"`
}

func main() {

	loadConfig()

	command, args := parseArgs(os.Args)
	// fmt.Printf("command: %s\nargs: %v\n", command, args)

	switch command {
	case "help":
		printUsage(args)
		os.Exit(0)
	case "run":
		runTests(args)
		os.Exit(0)
	case "create": // TODO
		fmt.Println("Create is not yet implemented. Sorry.")
		os.Exit(1)
	case "list":
		listTests(args)
		os.Exit(0)
	case "version":
		// TODO: Do this dynamically, rather than hard-coding the version number
		fmt.Println("Yoke v0.9 by mhweaver")
		os.Exit(0)
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", command)
		printUsage(nil)
		os.Exit(1)
	}

}

func printUsage(args []string) {
	if len(args) < 1 {
		fmt.Println("yoke usage: yoke [command [...]]")
		fmt.Println()
		fmt.Println("If a command is not specified, all tests are run (this is the same is running 'yoke run')")
		fmt.Println("Commands (for more information, 'yoke help [command]' is your friend):")
		fmt.Println("\tyoke create\tNot yet implemented. Create a new test")
		fmt.Println("\tyoke help\tView usage information for a command")
		fmt.Println("\tyoke list\tList recognized tests. Does not run tests")
		fmt.Println("\tyoke run\tRun tests")
		fmt.Println("\tyoke version\tShow version information")
		return
	}
	switch args[0] {
	case "help":
		printUsage(nil)
	case "run":
		fmt.Println("Usage:")
		fmt.Println("\tyoke run [test1 [test2 [...]]]")
		fmt.Println()
		fmt.Println("Runs the specified tests.")
		fmt.Println("If no tests are all given, all tests will be run.")
		fmt.Println("To see a list of all tests, use 'yoke list'")
	case "create":
		fmt.Println("Usage:")
		fmt.Println("\tyoke create name")
		fmt.Println()
		fmt.Println("***NOT YET IMPLEMENTED!***")
		fmt.Println("Creates a new test.")
		fmt.Println("The test will be placed in the directory [prefix]name, where [prefix] is the test name prefix specified in the yoke configuration file.")
	case "list":
		fmt.Println("Usage:")
		fmt.Println("\tyoke list [-profiles]")
		fmt.Println()
		fmt.Println("Lists tests.")
		fmt.Println("-profiles\t- Show test profiles")
	case "version":
		fmt.Println("Usage:")
		fmt.Println("\tyoke version")
		fmt.Println()
		fmt.Println("Show version information.")
		fmt.Println("Why are you even looking at the help for this command?!")
	default:
		fmt.Println("Unknown command: ", args[0])

	}
}

func loadConfig() {

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

func parseArgs(args []string) (command string, parsedArgs []string) {
	parsedArgs = args[1:] // strip off the "yoke"
	// fmt.Println("parsed args:", parsedArgs)

	// If the arg[0] isn't a command or doesn't exist, command = run
	if len(parsedArgs) < 1 {
		command = "run"
	} else {
		switch parsedArgs[0] {
		case "help":
			fallthrough
		case "run":
			fallthrough
		case "create":
			fallthrough
		case "list":
			fallthrough
		case "version":
			command = parsedArgs[0]
			parsedArgs = parsedArgs[1:]
			break
		default:
			command = "run"
		}
	}
	return
}

func runTests(args []string) {
	runFlags := flag.NewFlagSet("run", flag.ExitOnError)

	verbose := runFlags.Bool("verbose", false, "show all output")
	showInfo := runFlags.Bool("info", false, "show info output")
	showWarnings := runFlags.Bool("warnings", false, "show warnings")
	runFlags.Parse(args)

	if *verbose {
		*showInfo = true
		*showWarnings = true
	}
	tests := list.New()

	// Build list of tests and load profiles
	if len(runFlags.Args()) > 0 { // Get specified tests
		if *verbose {
			fmt.Printf("Attempting to run tests: %v\n", runFlags.Args())
		}
		for _, filename := range runFlags.Args() {
			fi, err := os.Stat(filename)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Unable to open: %s\n", filename)
				continue
			}
			if fi.IsDir() {
				t := newTest(fi.Name())
				tests.PushBack(t)
			}
		}

	} else { // no files specified, so do all of them
		testFiles, _ := ioutil.ReadDir("./")
		for _, f := range testFiles {
			if f.IsDir() && strings.HasPrefix(f.Name(), config.Prefix) {
				if *verbose {
					fmt.Println("Test found: " + f.Name())
				}
				t := newTest(f.Name())
				tests.PushBack(t)
			}
		}
	}

	numConcurrent := 0

	// Handle tests
	// Run concurrent tests
	c := make(chan bool, config.Maxthreads)
	var wg sync.WaitGroup
	for e := tests.Front(); e != nil; e = e.Next() {
		var currTest *test
		currTest = e.Value.(*test)
		if currTest.profile.Noconcurrent != nil && !*currTest.profile.Noconcurrent {
			c <- true // If there are > maxthreads running, wait for one to finish
			wg.Add(1)
			go currTest.runInThread(c, &wg)

			numConcurrent++
		}
	}
	// Wait until all concurrent tests are done
	wg.Wait()

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
		currTest.results.print(*showWarnings, *showInfo)
		// fmt.Println(currTest.profile.String())
	}

}

func listTests(args []string) {
	listFlags := flag.NewFlagSet("list", flag.ExitOnError)

	showProfiles := listFlags.Bool("profiles", false, "show test profiles details")
	listFlags.Parse(args)

	testFiles, _ := ioutil.ReadDir("./")
	for _, f := range testFiles {
		if f.IsDir() && strings.HasPrefix(f.Name(), config.Prefix) {
			if *showProfiles {
				// print test profile
				fmt.Println("\n==============================")
				fmt.Println("Test: " + f.Name())
				t := newTest(f.Name())
				fmt.Println(t.profile.String())
				fmt.Println("==============================")
			} else {
				fmt.Println(f.Name())
			}
		}
	}
}
