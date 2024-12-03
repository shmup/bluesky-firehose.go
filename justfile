run:
  go run example/main.go

test:
  go test

bench:
  go test -bench=. -benchmem
