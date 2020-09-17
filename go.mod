module github.com/wchargin/tensorboard-data-server

go 1.15

require (
	github.com/golang/protobuf v1.4.2
	github.com/tensorflow/tensorflow v0.0.0-00010101000000-000000000000
	google.golang.org/grpc v1.31.1
	google.golang.org/protobuf v1.23.0
)

replace github.com/tensorflow/tensorflow => ./genproto/github.com/tensorflow/tensorflow/
