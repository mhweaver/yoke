package main

import (
	"container/list"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

const (
	DEFAULT_CONFIG_FILE    = "tester_config.json"
	PROFILE_FILE_NAME      = "tester_profile.json"
	MATCH_READ_BUFFER_SIZE = 1024
)

type test struct {
	testName string
	done     bool
	results  *testResults
	stdin    io.Reader
	stdout   io.Writer
	stderr   io.Writer
	config   *testProfile
}

var config struct {
	DefaultProfile testProfile `json:"defaultProfile"`
	Maxthreads     int         `json:maxthreads`
	Prefix         string      `json:"prefix"`
}

var options struct {
	verbose      *bool
	showInfo     *bool
	showWarnings *bool
}

func main() {
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
		if !currTest.config.Noconcurrent {
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
		if currTest.config.Noconcurrent {
			currTest.run()
		}
	}

	for e := tests.Front(); e != nil; e = e.Next() {
		var currTest *test
		currTest = e.Value.(*test)
		currTest.results.print()
	}

}

func init() {
	options.verbose = flag.Bool("verbose", false, "show all output")
	options.showInfo = flag.Bool("info", false, "show info output")
	options.showWarnings = flag.Bool("warnings", false, "show warnings")
	flag.Parse()

	if *options.verbose {
		*options.showInfo = true
		*options.showWarnings = true
	}

	// Load/parse default config file
	configFile, err := os.Stat(DEFAULT_CONFIG_FILE)
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

	if config.Maxthreads <= 0 {
		config.Maxthreads = runtime.NumCPU()
	}
	runtime.GOMAXPROCS(config.Maxthreads)

}

func newTest(name string) (t *test) {
	t = new(test)
	t.testName = name
	t.results = newResults()
	t.results.testName = &t.testName
	configFile, err := ioutil.ReadFile(name + "/tester_profile.json")
	if err != nil {
		t.results.info(name + ": No config file found. Using default config")
		t.config = &config.DefaultProfile
	} else {
		err := json.Unmarshal(configFile, &t.config)
		if err != nil {
			log.Println("Unable to unmarshal config JSON: ", err)
		}
		t.config.setUnsetFieldsToDefault()
	}

	t.results.info("Test loaded: " + name + " (config: " + t.config.ConfigName + ")")

	return t
}

func (c *testProfile) setUnsetFieldsToDefault() {
	if c.After == nil { // []string
		c.After = config.DefaultProfile.After
	}
	if c.Before == nil { // []string
		c.Before = config.DefaultProfile.Before
	}
	if len(c.Command) == 0 { // string
		c.Command = config.DefaultProfile.Command
	}
	// if c.Noconcurrent == nil { // bool
	// 	c.Noconcurrent = config.DefaultProfile.Noconcurrent
	// }
	if len(c.ConfigName) == 0 { // string
		c.ConfigName = config.DefaultProfile.ConfigName
	}
	if c.Next == nil { // *testProfile
		c.Next = config.DefaultProfile.Next
	}

	// Pass (struct)
	if c.Pass == nil {
		c.Pass = config.DefaultProfile.Pass
	}
	// if c.Pass.ZeroExit == nil { // bool
	// 	c.Pass.ZeroExit = config.DefaultProfile.Pass.ZeroExit
	// }
	// if c.Pass.Match == nil { // [][]string
	// 	c.Pass.Match = config.DefaultProfile.Pass.Match
	// }
	// if c.Pass.Rmatch == nil { // [][]string
	// 	c.Pass.Rmatch = config.DefaultProfile.Pass.Rmatch
	// }
	// if c.Pass.LimitReached == nil { // bool
	// 	c.Pass.LimitReached = config.DefaultProfile.Pass.LimitReached
	// }

	if c.RequiredFiles == nil { // []string
		c.RequiredFiles = config.DefaultProfile.RequiredFiles
	}
	// if c.CreateRequired == nil { // bool
	// 	c.CreateRequired = config.DefaultProfile.CreateRequired
	// }
	if len(c.Stderr) == 0 { // string
		c.Stderr = config.DefaultProfile.Stderr
	}
	if c.Stdin == nil { // []string
		c.Stdin = config.DefaultProfile.Stdin
	}
	if len(c.Stdout) == 0 { // string
		c.Stdout = config.DefaultProfile.Stdout
	}
	if c.LimitOutput == 0 { // int64
		c.LimitOutput = config.DefaultProfile.LimitOutput
	}
	if c.MaxTimePerCommand == 0 {
		c.MaxTimePerCommand = config.DefaultProfile.MaxTimePerCommand
	}
}

func (t *test) run() {
	t.checkRequiredFiles()
	t.truncateOutputFiles()
	t.runBeforeCommands()
	t.runTestCommand()
	t.parseResults()
	t.runAfterCommands()

	if t.config.Next != nil {
		t.config = t.config.Next
		t.run()
	}

}

func (t *test) runInThread(c chan bool, done chan bool) {
	t.run()
	<-c // Done
	done <- true
}

func (t *test) checkRequiredFiles() {
	if len(t.config.RequiredFiles) == 0 {
		return
	}
	for _, v := range t.config.RequiredFiles {
		file, err := os.Open(t.testName + "/" + v)
		defer file.Close()
		if err != nil {
			if t.config.CreateRequired {
				t.results.info("Unable to open required file: " + v + ": " + err.Error())
				t.results.info("Creating " + t.testName + "/" + v)
				f, err := os.Create(t.testName + "/" + v)
				defer f.Close()
				if err != nil {
					t.results.fail("Unable to create required file: " + v + ": " + err.Error())
				} else {
					t.results.info("Created file")
				}
			} else {
				t.results.fail("Unable to open required file: " + v + ": " + err.Error())
			}
		}
	}
}

func (t *test) truncateOutputFiles() {
	filename := t.testName + "/" + t.config.Stdout
	f, err := os.OpenFile(filename, os.O_TRUNC, os.ModePerm)
	defer f.Close()
	if err != nil {
		t.results.info("Unable to open " + t.config.Stdout + " for truncation: " + err.Error())
	} else {
		f.Truncate(0)
	}

	filename = t.testName + "/" + t.config.Stderr
	f, err = os.OpenFile(filename, os.O_TRUNC, os.ModePerm)
	defer f.Close()
	if err != nil {
		t.results.info("Unable to open " + t.config.Stderr + " for truncation: " + err.Error())
	} else {
		f.Truncate(0)
	}
}

func (t *test) getStdio() (stdin io.Reader, stdout io.Writer, stderr io.Writer, openFiles []*os.File) {
	openFiles = make([]*os.File, 3)

	// Set up stdin multireader
	stdin = nil
	for _, v := range t.config.Stdin {
		filename := t.testName + "/" + v
		f, err := os.Open(filename)
		openFiles = append(openFiles, f)
		if err != nil {
			t.results.fail("Unable to open file: " + filename + ": " + err.Error())
		}
		if stdin != nil {
			stdin = io.MultiReader(stdin, f)
		} else {
			stdin = f
		}
	}

	// Set up stdout limitwriter
	stdout = nil
	filename := t.testName + "/" + t.config.Stdout
	f, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_RDWR, os.ModePerm)
	openFiles = append(openFiles, f)
	if err != nil {
		t.results.info("Unable to open " + t.config.Stdout + " for use as stdout: " + err.Error())
	}

	if t.config.LimitOutput <= 0 {
		stdout = f
	} else {
		stdout = limitWriter(f, t.config.LimitOutput, t.results)
	}

	// Set up stderr limitwriter
	stderr = nil
	filename = t.testName + "/" + t.config.Stderr
	f, err = os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_RDWR, os.ModePerm)
	openFiles = append(openFiles, f)
	if err != nil {
		t.results.info("Unable to open " + t.config.Stderr + " for use as stderr: " + err.Error())
	}

	if t.config.LimitOutput <= 0 {
		stderr = f
	} else {
		stderr = limitWriter(f, t.config.LimitOutput, t.results)
	}

	return stdin, stdout, stderr, openFiles
}

func (t *test) runCommands(commands []string) {
	for _, command := range commands {
		fmt.Println(command)

		// // Run sh -c command
		cmd := exec.Command("sh", "-c", command)

		stdin, stdout, stderr, openFiles := t.getStdio()
		cmd.Stdin = stdin
		cmd.Stdout = stdout
		cmd.Stderr = stderr
		err := cmd.Start()
		if err != nil {
			t.results.fail("Error running " + command + ": " + err.Error())
		}
		time.AfterFunc(time.Duration(t.config.MaxTimePerCommand)*time.Second, func() {
			cmd := cmd
			if cmd.ProcessState != nil && !cmd.ProcessState.Exited() {
				// Process is still running
				cmd.Process.Kill()
				t.results.exceededTimeLimit = append(t.results.exceededTimeLimit, cmd)
			}
		})
		cmd.Wait()

		for _, v := range openFiles {
			v.Close()
		}
	}
}

func (t *test) runBeforeCommands() {
	t.runCommands(t.config.Before)
}

type limitedWriter struct {
	n int64
	w io.Writer
	r *testResults
}

func limitWriter(w io.Writer, n int64, r *testResults) io.Writer {
	l := new(limitedWriter)
	l.w = w
	l.n = n
	l.r = r
	return l
}

func (l *limitedWriter) Write(p []byte) (n int, err error) {
	if l.n <= 0 {
		l.r.warn("Output limit reached")
		l.r.limitReached = true
		return 0, errors.New("Output limit reached")
	}
	if int64(len(p)) > l.n {
		p = p[0:l.n]
		l.r.warn("Output limit reached")
		l.r.limitReached = true
	}
	n, err = l.w.Write(p)
	l.n -= int64(n)
	return
}

func (t *test) runTestCommand() {
	command := t.config.Command
	// // Run sh -c command
	cmd := exec.Command("sh", "-c", command)
	stdin, stdout, stderr, openFiles := t.getStdio()
	cmd.Stdin = stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	cmd.Start()
	time.AfterFunc(time.Duration(t.config.MaxTimePerCommand)*time.Second, func() {
		cmd := cmd
		if cmd.ProcessState != nil && !cmd.ProcessState.Exited() {
			// Process is still running
			cmd.Process.Kill()
			t.results.exceededTimeLimit = append(t.results.exceededTimeLimit, cmd)
		}
	})
	cmd.Wait()
	t.results.cmd = cmd
	for _, v := range openFiles {
		v.Close()
	}
}

func (t *test) parseResults() {
	match := t.config.Pass.Match
	if match != nil {
		for k, v := range match {
			t.results.passed = t.results.passed && t.results.match(k, v)
		}
	}

	rmatch := t.config.Pass.Rmatch
	if rmatch != nil {
		for k, v := range rmatch {
			t.results.passed = t.results.passed && t.results.rmatch(k, v)
		}
	}

	zeroExit := t.config.Pass.ZeroExit
	if zeroExit {
		if !t.results.cmd.ProcessState.Success() {
			t.results.fail("Non-zero exit status (zero expected)")
		}
	} else {
		if t.results.cmd.ProcessState.Success() {
			t.results.fail("Zero exit status (non-zero expected)")
		}
	}

	limitReached := t.config.Pass.LimitReached
	if !limitReached && t.results.limitReached {
		t.results.fail("Output limit reached")
	} else if limitReached && !t.results.limitReached {
		t.results.fail("Output limit not reached")
	}

}

func (t *test) runAfterCommands() {
	t.runCommands(t.config.After)
}
