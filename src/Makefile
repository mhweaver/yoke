tester:	tester.go results.go profile.go
	go build -o $@ $^

run:	tester.go
	go run $<

clean:	
	rm -f tester