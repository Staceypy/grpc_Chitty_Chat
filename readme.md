protoc --go_out=. --go_opt=paths=source_relative --go-grpc_out=. --go-grpc_opt=paths=source_relative grpc/proto.proto

go get google.golang.org/grpc

NOTE: After adding "google.golang.org/grpc", you will need to run go mod tidy (specifically to be able to use the NewServer function).

go run server/server.go

go run client/client.go -cId 1 -cPort 5454

go run client/client.go -cId 2 -cPort 5455

rpc PublishMessage(Message) returns (Message);
  rpc BroadcastMessage(Message) returns (Message);



## Join

timestamp	event

0	client initialize

1	client join

2	server receive "client join"

3	server broadcast "Participant 1 joined Chitty-Chat at Lamport time 2"

4	client 2 received broadcasted message

## Leave