# GC=go build
# GCFLAGS=-compiler gccgo
GC=gccgo
GCFLAGS=-g

tester:	tester.go results.go profile.go
	$(GC) $(GCFLAGS) -o $@ $^

run:	tester.go
	go run $<

clean:	
	rm -f tester