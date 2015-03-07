package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"strconv"
	"strings"
)

// Generated with http://mervine.net/json2struct because I'm lazy
type testProfile struct {
	After             []string        `json:"after"`
	Before            []string        `json:"before"`
	Command           *string         `json:"command"`
	Noconcurrent      *bool           `json:noconcurrent`
	Name              *string         `json:"name"`
	Next              *testProfile    `json:"next"`
	Pass              *passConditions `json:"pass"`
	RequiredFiles     []string        `json:"requiredFiles"`
	CreateRequired    *bool           `json:createRequired`
	Stderr            *string         `json:"stderr"`
	Stdin             []string        `json:"stdin"`
	Stdout            *string         `json:"stdout"`
	LimitOutput       *int64          `json:limitOutput`
	MaxTimePerCommand *int64          `json:maxTimePerCommand`
}

type passConditions struct {
	ZeroExit                 *bool      `json:"zeroExit"`
	Match                    [][]string `json:"match"`
	Rmatch                   [][]string `json:"rmatch"`
	LimitReached             *bool      `json:limitReached`
	MaxTimePerCommandReached *bool      `json:maxTimePerCommandReached`
}

func newProfile(testdir string, r *testResults, defaultProfile *testProfile) (p *testProfile) {
	p = new(testProfile)

	profileBytes, err := ioutil.ReadFile(testdir + "/tester_profile.json")
	if err != nil {
		r.info("No config file found. Using default config")
	} else {
		err := json.Unmarshal(profileBytes, p)
		if err != nil {
			log.Println("Unable to unmarshal config JSON: ", err)
			r.fail("Unable to unmarshal config JSON")
		}
	}
	if false {
		fmt.Println(p.String())
	}

	p.copyUnsetFrom(defaultProfile)
	// fmt.Println(p.String())
	return
}

func (p *testProfile) fixNullReferences() {
	// Can be null (check before use):
	// Command      *string
	// Next         *testProfile
	// After        []string
	// Before       []string
	// RequiredFiles     []string
	// Stdin             []string
	// Stderr 		*string
	// Stdout 		*string
	// LimitOutput *int64
	// MaxTimePerCommand *int64
	// Pass *passConditions
	if p.Noconcurrent == nil { //  *bool
		*p.Noconcurrent = false
	}
	if p.Name == nil { //    *string
		*p.Name = "name not set"
	}
	if p.CreateRequired == nil { //     *bool
		*p.CreateRequired = false
	}

}

// Deep copy a testProfile struct
func (p *testProfile) copyUnsetFrom(defaultProfile *testProfile) {
	if p.After == nil && defaultProfile.After != nil {
		p.After = defaultProfile.After
	}
	if p.Before == nil && defaultProfile.Before != nil {
		p.Before = defaultProfile.Before
	}
	if p.RequiredFiles == nil && defaultProfile.RequiredFiles != nil {
		p.RequiredFiles = defaultProfile.RequiredFiles
	}
	if p.Stdin == nil && defaultProfile.Stdin != nil {
		p.Stdin = defaultProfile.Stdin
	}

	if p.Command == nil && defaultProfile.Command != nil {
		newCommand := *defaultProfile.Command
		p.Command = &newCommand
	}
	if p.Noconcurrent == nil && defaultProfile.Noconcurrent != nil {
		newNoconcurrent := *defaultProfile.Noconcurrent
		p.Noconcurrent = &newNoconcurrent
	}
	if p.Name == nil && defaultProfile.Name != nil {
		newName := *defaultProfile.Name
		p.Name = &newName
	}
	if p.Next == nil && defaultProfile.Next != nil {
		newNext := *defaultProfile.Next
		p.Next = &newNext
	}
	if p.Pass == nil && defaultProfile.Pass != nil {
		newPass := *defaultProfile.Pass
		p.Pass = &newPass
	}
	if p.CreateRequired == nil && defaultProfile.CreateRequired != nil {
		newCreateRequired := *defaultProfile.CreateRequired
		p.CreateRequired = &newCreateRequired
	}
	if p.Stderr == nil && defaultProfile.Stderr != nil {
		newStderr := *defaultProfile.Stderr
		p.Stderr = &newStderr
	}
	if p.Stdout == nil && defaultProfile.Stdout != nil {
		newStdout := *defaultProfile.Stdout
		p.Stdout = &newStdout
	}
	if p.LimitOutput == nil && defaultProfile.LimitOutput != nil {
		newLimitOutput := *defaultProfile.LimitOutput
		p.LimitOutput = &newLimitOutput
	}
	if p.MaxTimePerCommand == nil && defaultProfile.MaxTimePerCommand != nil {
		newMaxTimePerCommand := *defaultProfile.MaxTimePerCommand
		p.MaxTimePerCommand = &newMaxTimePerCommand
	}

	if p.Next == nil && defaultProfile.Next != nil {
		newNext := *defaultProfile.Next
		p.Next = &newNext
	}
	if p.Pass == nil && defaultProfile.Pass != nil {
		newPass := *defaultProfile.Pass
		p.Pass = &newPass
	}

	if p.Pass != nil && defaultProfile.Pass != nil {
		if p.Pass.ZeroExit == nil && defaultProfile.Pass.ZeroExit != nil {
			newZeroExit := *defaultProfile.Pass.ZeroExit
			p.Pass.ZeroExit = &newZeroExit
		}
		if p.Pass.LimitReached == nil && defaultProfile.Pass.LimitReached != nil {
			newLimitReached := *defaultProfile.Pass.LimitReached
			p.Pass.LimitReached = &newLimitReached
		}
		if p.Pass.MaxTimePerCommandReached == nil && defaultProfile.Pass.MaxTimePerCommandReached != nil {
			newMaxTimePerCommandReached := *defaultProfile.Pass.MaxTimePerCommandReached
			p.Pass.MaxTimePerCommandReached = &newMaxTimePerCommandReached
		}

		if p.Pass.Match == nil {
			copy(p.Pass.Match, defaultProfile.Pass.Match)
		}
		if p.Pass.Rmatch == nil {
			copy(p.Pass.Rmatch, defaultProfile.Pass.Rmatch)
		}
	}

	return
}

func (p *testProfile) String() (s string) {
	if p == nil {
		return "nil"
	}
	s = ""
	for _, v := range p.After {
		s += "\nAfter: " + v
	}
	for _, v := range p.Before {
		s += "\nBefore: " + v
	}
	if p.Command != nil {
		s += "\nCommand: " + *p.Command
	}
	if p.Noconcurrent != nil {
		s += "\nNonconcurrent: " + strconv.FormatBool(*p.Noconcurrent)
	}
	if p.Name != nil {
		s += "\nName: " + *p.Name
	}
	if p.Pass != nil {
		if p.Pass.ZeroExit != nil {
			s += "\nPass.ZeroExit: " + strconv.FormatBool(*p.Pass.ZeroExit)
		}
		if p.Pass.Match != nil {
			for _, v := range p.Pass.Match {
				s += "\nPass.Match: " + strings.Join(v, ", ")
			}
		}
		if p.Pass.Rmatch != nil {
			for _, v := range p.Pass.Rmatch {
				s += "\nPass.Rmatch: " + strings.Join(v, ", ")
			}
		}
		if p.Pass.LimitReached != nil {
			s += "\nPass.LimitReached: " + strconv.FormatBool(*p.Pass.LimitReached)
		}
		if p.Pass.MaxTimePerCommandReached != nil {
			s += "\nPass.MaxTimePerCommandReached: " + strconv.FormatBool(*p.Pass.MaxTimePerCommandReached)
		}
	}
	if p.RequiredFiles != nil {
		s += "\nRequiredFiles: " + strings.Join(p.RequiredFiles, ", ")
	}
	if p.CreateRequired != nil { // *bool
		s += "\nCreateRequired: " + strconv.FormatBool(*p.CreateRequired)
	}
	if p.Stderr != nil { // *string
		s += "\nStderr: " + *p.Stderr
	}
	if p.Stdin != nil {
		s += "\nStdin: " + strings.Join(p.Stdin, ", ")
	}
	if p.Stdout != nil { // *string
		s += "\nStdout: " + *p.Stdout
	}
	if p.LimitOutput != nil { // *int64
		s += "\nLimitOutput: " + strconv.FormatInt(*p.LimitOutput, 10)
	}
	if p.MaxTimePerCommand != nil { // *int64
		s += "\nMaxTimePerCommand: " + strconv.FormatInt(*p.MaxTimePerCommand, 10)
	}

	return
}
