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
	profile  *testProfile
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

	config.DefaultProfile.fixNullReferences()

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

	t.profile = newProfile(name, t.results, &config.DefaultProfile)

	t.results.info("Test loaded: " + name + " (config: " + *t.profile.Name + ")")

	return t
}

func (t *test) run() {
	t.checkRequiredFiles()
	t.truncateOutputFiles()
	t.runBeforeCommands()
	t.runTestCommand()
	t.parseResults()
	t.runAfterCommands()

	if t.profile.Next != nil {
		t.profile = t.profile.Next
		t.run()
	}
}

func (t *test) runInThread(c chan bool, done chan bool) {
	t.run()
	<-c // Done
	done <- true
}

func (t *test) checkRequiredFiles() {
	if t.profile.RequiredFiles == nil {
		return
	}
	for _, v := range t.profile.RequiredFiles {
		if *t.profile.CreateRequired {
			file, err := os.OpenFile(t.testName+"/"+v, os.O_CREATE|os.O_RDWR, os.ModePerm)
			defer file.Close()
			if err != nil {
				t.results.fail("Unable to open or create required file: " + v + ": " + err.Error())
			}
		} else {
			file, err := os.Open(t.testName + "/" + v)
			defer file.Close()
			if err != nil {
				t.results.fail("Unable to open required file: " + v + ": " + err.Error())
			}
		}
	}
}

func (t *test) truncateOutputFiles() {
	filename := t.testName + "/" + *t.profile.Stdout
	f, err := os.OpenFile(filename, os.O_TRUNC, os.ModePerm)
	defer f.Close()
	if err != nil {
		t.results.info("Unable to open " + *t.profile.Stdout + " for truncation: " + err.Error())
	} else {
		f.Truncate(0)
	}

	filename = t.testName + "/" + *t.profile.Stderr
	f, err = os.OpenFile(filename, os.O_TRUNC, os.ModePerm)
	defer f.Close()
	if err != nil {
		t.results.info("Unable to open " + *t.profile.Stderr + " for truncation: " + err.Error())
	} else {
		f.Truncate(0)
	}
}

func (t *test) getStdio() (stdin io.Reader, stdout io.Writer, stderr io.Writer, openFiles []*os.File) {
	openFiles = make([]*os.File, 3)

	if t.profile.Stdin == nil {
		stdin = os.Stdin
	} else {
		// Set up stdin multireader
		stdin = nil
		for _, v := range t.profile.Stdin {
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
	}

	// Set up stdout limitwriter
	if t.profile.Stdout == nil {
		if t.profile.LimitOutput != nil {
			stdout = limitWriter(os.Stdout, *t.profile.LimitOutput, t.results)
		} else {
			stdout = os.Stdout
		}
	} else {

		stdout = nil
		filename := t.testName + "/" + *t.profile.Stdout
		f, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_RDWR, os.ModePerm)
		openFiles = append(openFiles, f)
		if err != nil {
			t.results.info("Unable to open " + *t.profile.Stdout + " for use as stdout: " + err.Error())
		}

		if *t.profile.LimitOutput <= 0 {
			stdout = f
		} else {
			stdout = limitWriter(f, *t.profile.LimitOutput, t.results)
		}
	}

	// Set up stderr limitwriter
	if t.profile.Stderr == nil {
		if t.profile.LimitOutput != nil {
			stderr = limitWriter(os.Stderr, *t.profile.LimitOutput, t.results)
		} else {
			stderr = os.Stderr
		}
	} else {
		stderr = nil
		filename := t.testName + "/" + *t.profile.Stderr
		f, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_RDWR, os.ModePerm)
		openFiles = append(openFiles, f)
		if err != nil {
			t.results.info("Unable to open " + *t.profile.Stderr + " for use as stderr: " + err.Error())
		}

		if *t.profile.LimitOutput <= 0 {
			stderr = f
		} else {
			stderr = limitWriter(f, *t.profile.LimitOutput, t.results)
		}
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
		time.AfterFunc(time.Duration(*t.profile.MaxTimePerCommand)*time.Second, func() {
			command := command
			cmd := cmd
			if cmd.ProcessState != nil && !cmd.ProcessState.Exited() {
				// Process is still running
				cmd.Process.Kill()
				t.results.exceededTimeLimit = append(t.results.exceededTimeLimit, command)
			}
		})
		cmd.Wait()

		for _, v := range openFiles {
			v.Close()
		}
	}
}

func (t *test) runBeforeCommands() {
	if t.profile.Before != nil {
		t.runCommands(t.profile.Before)
	}
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
	if t.profile.Command == nil {
		t.results.fail("No test command specified")
		return
	}
	command := *t.profile.Command
	// // Run sh -c command
	cmd := exec.Command("sh", "-c", command)
	stdin, stdout, stderr, openFiles := t.getStdio()
	cmd.Stdin = stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	cmd.Start()
	if t.profile.MaxTimePerCommand != nil {
		time.AfterFunc(time.Duration(*t.profile.MaxTimePerCommand)*time.Second, func() {
			command := command
			cmd := cmd
			if cmd.ProcessState != nil && !cmd.ProcessState.Exited() {
				// Process is still running
				cmd.Process.Kill()
				t.results.exceededTimeLimit = append(t.results.exceededTimeLimit, command)
			}
		})
	}
	cmd.Wait()
	t.results.cmd = cmd
	for _, v := range openFiles {
		v.Close()
	}
}

func (t *test) parseResults() {
	if t.profile.Pass == nil {
		t.results.info("No pass conditions specified")
		return
	}

	match := t.profile.Pass.Match
	if match != nil {
		for k, v := range match {
			t.results.passed = t.results.passed && t.results.match(k, v)
		}
	}

	rmatch := t.profile.Pass.Rmatch
	if rmatch != nil {
		for k, v := range rmatch {
			t.results.passed = t.results.passed && t.results.rmatch(k, v)
		}
	}

	if t.profile.Pass.ZeroExit != nil {
		zeroExit := *t.profile.Pass.ZeroExit
		if zeroExit {
			if !t.results.cmd.ProcessState.Success() {
				t.results.fail("Non-zero exit status (zero expected)")
			}
		} else {
			if t.results.cmd.ProcessState.Success() {
				t.results.fail("Zero exit status (non-zero expected)")
			}
		}
	}

	if t.profile.Pass.LimitReached != nil {
		limitReached := *t.profile.Pass.LimitReached
		if !limitReached && t.results.limitReached {
			t.results.fail("Output limit reached")
		} else if limitReached && !t.results.limitReached {
			t.results.fail("Output limit not reached")
		}
	}

	if t.profile.Pass.MaxTimePerCommandReached != nil {
		mtpcReached := *t.profile.Pass.MaxTimePerCommandReached
		if mtpcReached {
			for _, v := range t.results.exceededTimeLimit {
				t.results.fail("Command time limit reached: " + v)
			}
		} else {
			for _, v := range t.results.exceededTimeLimit {
				t.results.fail("Command time limit not reached: " + v)
			}
		}
	}

}

func (t *test) runAfterCommands() {
	if t.profile.After != nil {
		t.runCommands(t.profile.After)
	}
}
