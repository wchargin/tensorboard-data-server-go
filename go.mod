module github.com/wchargin/tensorboard-data-server

go 1.15

require (
	github.com/golang/protobuf v1.4.2
	github.com/tensorflow/tensorflow v0.0.0-00010101000000-000000000000
	github.com/wchargin/tensorboard-data-server/proto v0.0.0-00010101000000-000000000000
	google.golang.org/grpc v1.32.0
	google.golang.org/grpc/cmd/protoc-gen-go-grpc v1.0.0 // indirect
	google.golang.org/protobuf v1.25.0
)

replace github.com/tensorflow/tensorflow => ./genproto/github.com/tensorflow/tensorflow/

replace github.com/wchargin/tensorboard-data-server/proto => ./genproto/github.com/wchargin/tensorboard-data-server/proto/
