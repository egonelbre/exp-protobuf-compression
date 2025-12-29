.PHONY: generate
generate:
	buf generate
	cp -r gen/github.com/egonelbre/exp-protobuf-compression/* .
	rm -rf gen
	make fmt

.PHONY: fmt
fmt:
	goimports -w -local github.com/egonelbre .
