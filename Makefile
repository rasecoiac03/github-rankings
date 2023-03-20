install-tools:
	cat tools/tools.go | grep "_" | awk -F '"' '{print $$2}' | xargs -L1 go install

vendor:
	go mod vendor

clean:
	rm -rf vendor

install: clean install-tools vendor

lint:
	gocritic check -disable=codegenComment ./...
