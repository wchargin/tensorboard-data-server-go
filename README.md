# tensorboard-data-server

Standalone service to read TensorBoard data from event files and expose it to
clients over an RPC interface.

## Building

Clone [TensorBoard][tensorboard]. Run:

```
./build_protos.sh --bootstrap
go get github.com/golang/protobuf/protoc-gen-go
go get google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.0
./build_protos.sh PATH_TO_TENSORBOARD_REPO
```

Then use the standard `go(1)` CLI:

```
go build ./...
go test ./...
go run ./io/cmd/tfrecord_bench --help
```

[tensorboard]: https://github.com/tensorflow/tensorboard
