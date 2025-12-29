.PHONY: generate
generate:
	buf generate
	make fmt

.PHONY: fmt
fmt:
	goimports -w -local github.com/egonelbre .
