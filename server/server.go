package main

import (
	"context"
	"flag"
	proto "handin3/grpc" // Adjust the import path
	"log"
	"net"
	"strconv"
	"sync"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
)

type Server struct {
	proto.UnimplementedChatServiceServer // Necessary
	mu                                   sync.Mutex
	messages                             []*proto.Message
	clients                              map[int64]int64
	lastID                               int64
	port                                 int
}

var port = flag.Int("port", 8080, "The server port")

func (s *Server) Join(ctx context.Context, req *proto.Client) (*proto.Client, error) {

	s.mu.Lock()
	defer s.mu.Unlock()

	// Increment the server's Lamport timestamp
	s.lastID++

	// Set the client's timestamp to the current server timestamp
	req.Timestamp = s.lastID

	// Add the client to the map
	s.clients[req.ClientId] = req.Timestamp
	log.Printf("Participant %d joined Chitty-Chat at Lamport time %d", req.ClientId, req.Timestamp)

	return req, nil
}
func (s *Server) Leave(ctx context.Context, req *proto.Client) (*proto.Client, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if the client exists in the map
	if _, ok := s.clients[req.ClientId]; !ok {
		return nil, grpc.Errorf(codes.NotFound, "Client %d not found", req.ClientId)
	}

	// Increment the server's Lamport timestamp
	s.lastID++

	// Set the client's timestamp to the current server timestamp
	req.Timestamp = s.lastID

	// Remove the client from the map
	delete(s.clients, req.ClientId)

	log.Printf("Participant %d left Chitty-Chat at Lamport time %d", req.ClientId, req.Timestamp)
	return req, nil
}

func main() {
	flag.Parse()

	server := &Server{
		clients: make(map[int64]int64),
		port:    *port,
		lastID:  0,
	}
	go startServer(server)

	// Keep the server running until it is manually quit
	for {

	}
}

func startServer(server *Server) {

	// Create a new grpc server
	grpcServer := grpc.NewServer()

	// Make the server listen at the given port (convert int port to string)
	listener, err := net.Listen("tcp", ":"+strconv.Itoa(server.port))

	if err != nil {
		log.Fatalf("Could not create the server %v", err)
	}
	log.Printf("Started server at port: %d\n", server.port)

	// Register the service with the server
	proto.RegisterChatServiceServer(grpcServer, server)
	serveError := grpcServer.Serve(listener)
	if serveError != nil {
		log.Fatalf("Could not serve listener")
	}
}

// func (s *chatServer) PublishMessage(ctx context.Context, req *proto.Message) (*proto.Message, error) {
// 	s.mu.Lock()
// 	defer s.mu.Unlock()

// 	req.Timestamp = time.Now().UnixNano()
// 	s.messages = append(s.messages, req)

// 	grpclog.Infof("Published by client %d: %s", req.ClientId, req.Text)

// 	// Broadcast the message to all connected clients.
// 	for _, clientCh := range s.clients {
// 		go func(ch chan *proto.Message) {
// 			ch <- req
// 		}(clientCh)
// 	}

// 	return req, nil
// }

// func (s *chatServer) BroadcastMessage(stream proto.ChatService_BroadcastMessageClient) error {
// 	clientID := s.registerClient(stream)

// 	defer func() {
// 		s.mu.Lock()
// 		delete(s.clients, clientID)
// 		s.mu.Unlock()
// 		grpclog.Infof("Client %d disconnected.", clientID)
// 	}()

// 	grpclog.Infof("Client %d connected.", clientID)

// 	for {
// 		message, err := stream.Recv()
// 		if err != nil {
// 			return err
// 		}

// 		message.ClientId = clientID
// 		s.PublishMessage(context.Background(), message)
// 	}
// }

// func (s *chatServer) registerClient(stream proto.ChatService_BroadcastMessageServer) int64 {
// 	s.mu.Lock()
// 	defer s.mu.Unlock()

// 	s.lastID++
// 	clientID := s.lastID
// 	s.clients[clientID] = make(chan *proto.Message)

// 	go func() {
// 		// Continuously send messages to the client's stream.
// 		for message := range s.clients[clientID] {
// 			stream.Send(message)
// 		}
// 	}()

// 	return clientID
// }
