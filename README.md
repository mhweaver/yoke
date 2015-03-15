# yoke
A flexible software test engine written in golang

Yoke started as a simple regression tester bash-script to execute a command and compare the stdout and stderr to the last successful test.
I quickly realized that this was inadequate for a significant portion of the testing I needed/wanted. I kept wanting things like concurrent tests, exit code checking, different test criteria for different tests, etc. For some of these, I'd pretty much need to make a new tester for each individual test.
I developed Yoke to deal with these problems.

Features - Most of the features in Yoke are things I've wanted in a test engine, for some reason or another:
* Configuration: Yoke offers a lot of customizations, allowing you to tailor your tests to your needs. With the right configuration, you can pretty much just drop Yoke in place of other test engines, no matter how the existing tests are written (this was important, since I wanted to use it to replace the different test engines I was using it various projects). One limitation here is that Yoke expects each test to be in its own directory, and the test directories should start with the same prefix (like "test-"). All Yoke configuration files are written in the JSON format, to make parsing easier (because I'm lazy).
* Profiles: To use Yoke, you specify a default profile, setting up the rules for the tests. Since not all tests are created equal, you can give tests their own profiles (if it doesn't have a profile of its own or if a setting isn't specified, Yoke uses the settings in the default profile)
* Concurrency: You can run multiple tests simultaneously. By default, Yoke will run in as many threads as you have processor codes (this can be changed). Since you may not want all tests to be concurrent, you can disable concurrency for individual tests (or all of them, if you want). After all the concurrent tests finish, the nonconcurrent ones run on at a time.
* Speed: Since Yoke compiles to binary, does most of the slow stuff (e.g., comparing files) itself instead of starting up other processes for it, and runs concurrently, it can run considerably faster than a typical Bash regression tester.
* Set up and tear down: If a test needs some set up beforehand or afterward, you can give Yoke a list of commands to run before and after the test runs
* Output limits: Do bugs in your project tend to result in infinite loops which keep dumping garbage during tests? Yoke can write a specified number of bytes to output files, then discard the rest. This may or may not mean the test fails, though (you decide!)
* Time limits: Similar to the output limits, Yoke can terminate programs which take too long.
* Pass conditions: Doesn't matter if 2 files match? Then don't fail the test if they don't; just leave the "match" rule out of the pass conditions and it won't even bother comparing the files. Do you want a test to pass when a program doesn't exit cleanly? Add that to the pass conditions.
* Muliple input: Input for the program being tested normally comes from a single file. Yoke allows you to use several files. It feeds them, in order, into the program as a single input stream. In some situations, this can make tests easier to create and keep organized.
* Test chaining: Multiple tests can be chained together in a single test. This is useful for things like compilers, where you might want to execute the output of another test. For example: test1 generates (and verifies) hello.o; test1-1 then somehow executes hello.o, to verify its output is also correct
* Regular expression file matching: Sometimes the output from a test changes every time the test runs (maybe the output has the current time or something). Regex matching allows you to specify the expected output with a little more freedom. To see an example of this, check out test-regex/

Works in progress:
* Configuration/profile generation: JSON is nice, but do you know what's even better? Not having to write JSON files by hand. Some day.
* Proper documentation: I tried to make the JSON files easy to understand, but good documentation is always nice.
