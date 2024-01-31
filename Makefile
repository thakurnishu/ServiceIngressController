
run: build
	@./bin/SIK-controller

build:
	@cd cmd && go build -o ../bin/SIK-controller && cd ../ 
	
