.PHONY: all clean deps

PROTO_SOURCES = $(wildcard *.proto)
GO_SOURCES = $(patsubst %.proto,%.pb.go,$(PROTO_SOURCES))
GRPC_SOURCES = $(patsubst %.proto,%_grpc.pb.go,$(PROTO_SOURCES))
COBRA_SOURCES = $(patsubst %.proto,%.cobra.pb.go,$(PROTO_SOURCES))

all:
	protoc -I. --go_out=. --go-grpc_out=. --cobra_out=. --go_opt=paths=source_relative --go-grpc_opt=paths=source_relative --cobra_opt=paths=source_relative $(PROTO_SOURCES)

clean:
	rm -f $(GO_SOURCES) $(GRPC_SOURCES) $(COBRA_SOURCES)

deps:
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
	go install github.com/NathanBaulch/protoc-gen-cobra@latest
