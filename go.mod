module github.com/wchargin/tensorboard-data-server

go 1.15

require (
	github.com/golang/protobuf v1.4.2 // indirect
	github.com/tensorflow/tensorflow v0.0.0-00010101000000-000000000000
	google.golang.org/grpc v1.31.1
)

replace github.com/tensorflow/tensorflow => ./genproto/github.com/tensorflow/tensorflow/
