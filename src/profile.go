package main

// Generated with http://mervine.net/json2struct because I'm lazy
type testProfile struct {
	After        []string     `json:"after"`
	Before       []string     `json:"before"`
	Command      string       `json:"command"`
	Noconcurrent bool         `json:noconcurrent`
	ConfigName   string       `json:"name"`
	Next         *testProfile `json:"next"`
	Pass         *struct {
		ZeroExit     bool       `json:"zeroExit"`
		Match        [][]string `json:"match"`
		Rmatch       [][]string `json:"rmatch"`
		LimitReached bool       `json:limitReached`
	} `json:"pass"`
	RequiredFiles     []string `json:"requiredFiles"`
	CreateRequired    bool     `json:createRequired`
	Stderr            string   `json:"stderr"`
	Stdin             []string `json:"stdin"`
	Stdout            string   `json:"stdout"`
	LimitOutput       int64    `json:limitOutput`
	MaxTimePerCommand int64    `json:maxTimePerCommand`
}
