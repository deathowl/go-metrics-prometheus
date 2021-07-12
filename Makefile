vet:
	go vet -v ./...

test:
	go test -v ./...

fmt:
	go fmt ./...

.PHONY: test vet fmt