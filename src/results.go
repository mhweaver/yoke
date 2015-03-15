package main

import (
	"bufio"
	"bytes"
	"container/list"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"regexp"
)

type testResults struct {
	testName          *string
	passed            bool
	limitReached      bool
	exceededTimeLimit []string
	errorList         *list.List
	infoList          *list.List
	warningList       *list.List
	cmd               *exec.Cmd
}

func newResults() (r *testResults) {
	r = new(testResults)
	r.passed = true
	r.errorList = list.New()
	r.infoList = list.New()
	r.warningList = list.New()
	r.passed = true
	r.limitReached = false
	r.exceededTimeLimit = make([]string, 0)
	return
}

func (r *testResults) match(index int, files []string) (ret bool) {
	if len(files) < 2 {
		r.info("Not enough filenames provided for match rule (" + string(index) + ")")
		return true
	}

	ret = true
	// Build a slice of *os.File objects
	fs := list.New()
	for _, v := range files {
		f, err := os.Open(*r.testName + "/" + v)
		defer f.Close()
		if err != nil {
			r.fail("Unable to open file for comparison: " + v)
			ret = false
			continue
		}
		fs.PushBack(f)
	}
	for e := fs.Front(); e != nil; e = e.Next() {
		if e.Next() != nil {
			// Set up limited readers for the files, so we don't read too much
			f1 := e.Value.(*os.File)
			f1lr := io.LimitReader(f1, MATCH_READ_BUFFER_SIZE)
			f2 := e.Next().Value.(*os.File)
			f2lr := io.LimitReader(f2, MATCH_READ_BUFFER_SIZE)
			// fmt.Printf("f1: %s, f2: %s\n", f1.Name(), f2.Name())
			var f1lrBuf, f2lrBuf []byte // Buffers
			f1lrBuf = make([]byte, MATCH_READ_BUFFER_SIZE)
			f2lrBuf = make([]byte, MATCH_READ_BUFFER_SIZE)
			for {
				f1bytesRead, _ := f1lr.Read(f1lrBuf)
				f2bytesRead, _ := f2lr.Read(f2lrBuf)
				// fmt.Printf("f1buf: %s\nf2buf: %s\n", string(f1lrBuf), string(f2lrBuf))
				if f1bytesRead != f2bytesRead || !bytes.Equal(f1lrBuf, f2lrBuf) {
					ret = false
					r.fail("Files don't match: " + f1.Name() + ", " + f2.Name())
					break
				}
				if f1bytesRead == 0 && f2bytesRead == 0 {
					r.info("Files match: " + f1.Name() + ", " + f2.Name())
					// Files match, so we can move onto the next file
					break
				}
			}
		}
	}
	// If we made it down to here, all the files matched (or weren't accessible)
	return
}

func (r *testResults) rmatch(index int, files []string) {
	if len(files) < 2 {
		r.warn("Not enough filenames provided for match rule (" + string(index) + ")")
		return
	}

	// Open the first file (regexp)
	reFile, err := os.Open(*r.testName + "/" + files[0])
	defer reFile.Close()
	if err != nil {
		r.fail("Unable to open regular expression file: " + files[0])
		return
	}

	// Read regexp file
	reBytes, err := ioutil.ReadAll(reFile)
	if err != nil {
		r.fail("Unable to read regular expression file: " + files[0])
		return
	}
	if len(reBytes) == 0 {
		r.warn("Zero-length regular expression file: " + files[0])
	}

	// Compile regexp file
	re, err := regexp.Compile(string(reBytes))
	if err != nil {
		r.fail("Unable to compile regular expression file: " + files[0])
		return
	}

	// Build a list of *os.File objects
	fs := list.New()
	for _, v := range files[1:] { // Skip the first file
		f, err := os.Open(*r.testName + "/" + v)
		defer f.Close()
		if err != nil {
			r.fail("Unable to open file for comparison: " + v)
			continue
		}
		fs.PushBack(f)
	}

	// Compare files to the regexp
	for e := fs.Front(); e != nil; e = e.Next() {
		// Set up limited readers for the files, so we don't read too much
		f := e.Value.(*os.File)
		if !re.MatchReader(bufio.NewReader(f)) {
			r.fail("Files don't match (using regular expression): " + reFile.Name() + ", " + f.Name())
		}
	}
	// If we made it down to here, all the files matched (or weren't accessible)
	return

	// r.fail("rmatch not yet implemented, so using regular match")
	// t.match(index, files)
	// return false
}

func (r *testResults) fail(msg string) {
	r.errorList.PushBack(msg)
	r.passed = false
}

func (r *testResults) info(msg string) {
	r.infoList.PushBack(msg)
}

func (r *testResults) warn(msg string) {
	r.warningList.PushBack(msg)
}

func (r *testResults) print() {
	for e := r.infoList.Front(); *options.showInfo && e != nil; e = e.Next() {
		var result string
		result = e.Value.(string)
		fmt.Println(*r.testName + "(info): " + result)
	}
	for e := r.warningList.Front(); *options.showWarnings && e != nil; e = e.Next() {
		var result string
		result = e.Value.(string)
		fmt.Fprintln(os.Stderr, *r.testName+"(warning): "+result)
	}
	for e := r.errorList.Front(); e != nil; e = e.Next() {
		var result string
		result = e.Value.(string)
		fmt.Fprintln(os.Stderr, *r.testName+"(failure): "+result)
	}
	if !r.passed {
		fmt.Println(*r.testName + ": failed")
	}

}
