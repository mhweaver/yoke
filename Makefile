all:	tester

tester:	FORCE
	cd src; make
	cp src/$@ .

run:	tester
	./$< -verbose

clean:	FORCE
	cd src; make clean
	rm -f tester

FORCE: