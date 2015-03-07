# GC=go build
# GCFLAGS=-compiler gccgo
GC=gccgo
GCFLAGS=-g

yoke:	yoke.go results.go profile.go
	$(GC) $(GCFLAGS) -o $@ $^

run:	yoke.go
	go run $<

clean:	
	rm -f yoke