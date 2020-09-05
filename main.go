package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net"

	v2 "github.com/envoyproxy/go-control-plane/envoy/service/accesslog/v2"
	v3 "github.com/envoyproxy/go-control-plane/envoy/service/accesslog/v3"
	"github.com/golang/protobuf/jsonpb"
	"google.golang.org/grpc"
)

type serverv3 struct {
	marshaler jsonpb.Marshaler
}

type serverv2 struct {
	marshaler jsonpb.Marshaler
}

// NewV3 ...
func NewV3() v3.AccessLogServiceServer {
	return &serverv3{}
}

// NewV2 ...
func NewV2() v2.AccessLogServiceServer {
	return &serverv2{}
}

func (s *serverv3) StreamAccessLogs(stream v3.AccessLogService_StreamAccessLogsServer) error {
	log.Println("Started stream")
	for {
		in, err := stream.Recv()
		log.Println("Received value")
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		str, _ := s.marshaler.MarshalToString(in)
		println(str)
	}
}

func (s *serverv2) StreamAccessLogs(stream v2.AccessLogService_StreamAccessLogsServer) error {
	log.Println("Started stream")
	for {
		in, err := stream.Recv()
		log.Println("Received value")
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		str, _ := s.marshaler.MarshalToString(in)
		println(str)
	}
}

func main() {
	var port int

	flag.IntVar(&port, "port", 15000, "listening port for ALS service")
	flag.Parse()

	grpcServer := grpc.NewServer()
	v3.RegisterAccessLogServiceServer(grpcServer, NewV3())
	v2.RegisterAccessLogServiceServer(grpcServer, NewV2())

	listen, err := net.Listen("tcp4", fmt.Sprintf(":%d", port))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	log.Printf("Listening on %s://%s", listen.Addr().Network(), listen.Addr().String())
	grpcServer.Serve(listen)
}
