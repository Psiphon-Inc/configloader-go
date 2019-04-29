workflow "Go Test" {
  on = "push"
  resolves = ["test"]
}

action "test" {
  uses="cedrickring/golang-action@1.3.0"

  args="go get -v ./... && go vet ./... && go build -v ./... && go test -v -cover -race ./..."
}
