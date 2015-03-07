all:	yoke

yoke:	FORCE
	cd src; make
	cp src/$@ .

run:	yoke
	./$< -verbose

clean:	FORCE
	cd src; make clean
	rm -f yoke

FORCE: