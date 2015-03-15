# GC=go build
# GCFLAGS=-compiler gccgo
GC=gccgo
GCFLAGS=-g

GOFILES=$(wildcard *.go)

yoke:	$(GOFILES)
	$(GC) $(GCFLAGS) -o $@ $^

run:	yoke.go
	go run $<

clean:	
	rm -f yoke