package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"github.com/wchargin/tensorboard-data-server/fs"
	ioLogdir "github.com/wchargin/tensorboard-data-server/io/logdir"
	dppb "github.com/wchargin/tensorboard-data-server/proto/data_provider_proto"
	"github.com/wchargin/tensorboard-data-server/server"
)

var logdir = flag.String("logdir", "", "log directory")
var port = flag.Int("port", 6106, "server port")
var reloadInterval = flag.Duration("reload_interval", 5*time.Second, "duration to wait between reloads")

func main() {
	flag.Parse()
	if len(*logdir) == 0 {
		log.Fatalf("must specify log directory")
	}

	ll := ioLogdir.LoaderBuilder{FS: fs.OS{}, Logdir: *logdir}.Start()
	go func() {
		ll.Reload()
		log.Printf("logdir loaded; now polling")
		for {
			ll.Reload()
			time.Sleep(*reloadInterval)
		}
	}()

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", *port))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	log.Printf("listening on %s", lis.Addr())

	s := grpc.NewServer()
	dppb.RegisterTensorBoardDataProviderServer(s, server.NewServer(ll))
	reflection.Register(s)
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
