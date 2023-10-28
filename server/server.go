package main

import (
	"context"
	"flag"
	proto "handin3/grpc" // Adjust the import path
	"io"
	"log"
	"net"
	"strconv"
	"sync"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type Server struct {
	proto.UnimplementedChatServiceServer // Necessary
	mu                                   sync.Mutex
	messages                             []*proto.Message
	clients                              map[int64]int64
	ClientStreams                        map[int64]chan *proto.StreamResponse
	Broadcast                            chan *proto.StreamResponse
	streamsMtx                           sync.RWMutex
	lastID                               int64
	port                                 int
}

var port = flag.Int("port", 5455, "The server port")

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

	s.Broadcast <- &proto.StreamResponse{
		Timestamp: req.Timestamp,
		Event: &proto.StreamResponse_ClientJoin{ClientJoin: &proto.StreamResponse_Join{
			ClientId: req.ClientId,
		}},
	}

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
	s.Broadcast <- &proto.StreamResponse{
		Timestamp: req.Timestamp,
		Event: &proto.StreamResponse_ClientLeave{ClientLeave: &proto.StreamResponse_Leave{
			ClientId: req.ClientId,
		}},
	}
	return req, nil
}

func main() {
	flag.Parse()

	server := &Server{
		clients:   make(map[int64]int64),
		port:      *port,
		lastID:    0,
		Broadcast: make(chan *proto.StreamResponse, 1000),

		ClientStreams: make(map[int64]chan *proto.StreamResponse),
	}
	startServer(server)

}

func startServer(server *Server) {
	ctx, cancel := context.WithCancel(context.Background())

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
	go server.broadcast(ctx)

	go func() {
		_ = grpcServer.Serve(listener)
		cancel()
	}()

	<-ctx.Done()
}
func (s *Server) broadcast(_ context.Context) {
	for res := range s.Broadcast {
		s.streamsMtx.RLock()
		for _, stream := range s.ClientStreams {
			select {
			case stream <- res:
				// noop
			default:
				log.Println("client stream full, dropping message")
			}
		}
		s.streamsMtx.RUnlock()
	}
}

// the server can send broadcasted messages to the client through this connection
// client establishes a server-side streaming connection with the server, and it expects to receive a stream of messages from the server.
func (s *Server) Stream(srv proto.ChatService_StreamServer) error {
	clientID, ok := s.extractClientID(srv.Context())
	if !ok {
		return status.Error(codes.Unauthenticated, "missing client ID")
	}

	// Check if the clientID is stored in the server's map of connected clients.
	if _, found := s.clients[clientID]; !found {
		return status.Error(codes.PermissionDenied, "client is not authorized to stream")
	}

	go s.sendBroadcasts(srv, clientID)

	for {
		req, err := srv.Recv()
		if err == io.EOF {
			break
		} else if err != nil {
			return err
		}

		// set maximum of local and event timestamps to be the current timestamp
		clientTimestamp := req.GetTimestamp()
		if clientTimestamp > s.lastID {
			s.lastID = clientTimestamp
		}
		s.lastID++

		s.Broadcast <- &proto.StreamResponse{
			Timestamp: s.lastID,
			Event: &proto.StreamResponse_ClientMessage{ClientMessage: &proto.StreamResponse_Message{
				ClientId: clientID,
				Message:  req.Message,
			}},
		}
	}

	<-srv.Context().Done()
	return srv.Context().Err()
}

func (s *Server) openStream(clientId int64) (stream chan *proto.StreamResponse) {
	stream = make(chan *proto.StreamResponse, 100)

	s.streamsMtx.Lock()
	s.ClientStreams[clientId] = stream
	s.streamsMtx.Unlock()

	log.Printf("opened stream for client %d", clientId)

	return
}

func (s *Server) closeStream(clientId int64) {
	s.streamsMtx.Lock()
	if stream, ok := s.ClientStreams[clientId]; ok {
		delete(s.ClientStreams, clientId)
		close(stream)
	}

	log.Printf("closed stream for client %d", clientId)

	s.streamsMtx.Unlock()
}

func (s *Server) sendBroadcasts(srv proto.ChatService_StreamServer, clientId int64) {
	stream := s.openStream(clientId)
	defer s.closeStream(clientId)

	for {
		select {
		case <-srv.Context().Done():
			return
		case res := <-stream:
			if s, ok := status.FromError(srv.Send(res)); ok {
				switch s.Code() {
				case codes.OK:
					// noop
				case codes.Unavailable, codes.Canceled, codes.DeadlineExceeded:
					log.Fatalf("client (%d) terminated connection", clientId)
					return
				default:
					log.Fatalf("failed to send to client (%d): %v", clientId, s.Err())
					return
				}
			}
		}
	}
}

func (s *Server) extractClientID(ctx context.Context) (int64, bool) {
	// Extract metadata from the context
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		log.Fatalf("missing metadata from context")
		return 0, false
	}

	// Check if the "clientID" key exists in the metadata
	if values, found := md["clientid"]; found && len(values) > 0 {
		// Try to parse the "clientID" value as an int64
		if clientID, err := strconv.ParseInt(values[0], 10, 64); err == nil {
			return clientID, true

		}
	}

	return 0, false
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
