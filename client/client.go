package main

import (
	"bufio"
	"context"
	"flag" // Adjust the import path
	proto "handin3/grpc"
	"io"
	"log"
	"os"
	"strconv"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type Client struct {
	proto.ChatServiceClient
	id         int64
	portNumber int
	timestamp  int64
}

var (
	clientId   = flag.Int64("cId", 0, "client id")
	clientPort = flag.Int("cPort", 5454, "client port number")
	serverPort = flag.Int("sPort", 5455, "server port number (should match the port used for the server)")
)

func ClientIns() *Client {
	return &Client{
		id:         *clientId,
		portNumber: *clientPort,
		timestamp:  0,
	}
}
func main() {
	// Parse the flags to get the port for the client
	flag.Parse()
	err := ClientIns().Run(context.Background())
	if err != nil {
		log.Fatalf("Failed to run client: %v", err)
		os.Exit(1)
	}
}
func (c *Client) Run(ctx context.Context) error {
	conn, err := grpc.Dial("localhost:"+strconv.Itoa(*serverPort), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Could not connect to port %d", *serverPort)
	} else {
		log.Printf("Connected to the server at port %d\n", *serverPort)
	}
	c.ChatServiceClient = proto.NewChatServiceClient(conn)
	//join
	clientReq := &proto.Client{ClientId: c.id, Timestamp: c.timestamp}
	_, err = c.Join(ctx, clientReq)
	if err != nil {
		log.Fatalf("Failed to join: %v", err)
	}
	//stream
	err = c.stream(ctx)
	if err != nil {
		log.Fatalf("Failed to stream: %v", err)
	}

	//leave
	// _, err = c.Leave(ctx, clientReq)
	// if err != nil {
	// 	log.Fatalf("Failed to leave: %v", err)
	// }

	return err
}

func (c *Client) send(client proto.ChatService_StreamClient) {
	sc := bufio.NewScanner(os.Stdin)

	for {
		select {
		case <-client.Context().Done():
			log.Printf("client send loop disconnected")
		default:
			if sc.Scan() {
				if err := client.Send(&proto.StreamRequest{Timestamp: c.timestamp, Message: sc.Text(), ClientId: c.id}); err != nil {
					log.Printf("failed to send message")
					return
				}
			} else {
				log.Printf("input scanner failure")
				return
			}
		}
	}

}
func (c *Client) stream(ctx context.Context) error {
	md := metadata.New(map[string]string{"clientid": strconv.FormatInt(c.id, 10)})
	//Metadata: Key: clientid, Value: 1

	// add clientID metadata to the context
	ctx = metadata.NewOutgoingContext(ctx, md)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	client, err := c.ChatServiceClient.Stream(ctx)
	if err != nil {
		log.Fatalf("failed to create stream: %v", err)
		return err
	}
	defer client.CloseSend()

	log.Printf("Client %d connected to the stream", c.id)

	go c.send(client)
	return c.receive(client)

}

func (c *Client) receive(client proto.ChatService_StreamClient) error {
	for {
		res, err := client.Recv()

		if s, ok := status.FromError(err); ok && s.Code() == codes.Canceled {
			log.Printf("stream canceled (usually indicates shutdown)")
			return nil
		} else if err == io.EOF {
			log.Printf("stream closed by server")
			return nil
		} else if err != nil {
			return err
		}
		receivedTimestamp := res.Timestamp
		if receivedTimestamp > c.timestamp {
			c.timestamp = receivedTimestamp
		}
		c.timestamp++
		switch evt := res.Event.(type) {
		case *proto.StreamResponse_ClientJoin:
			log.Printf("Participant %d joined Chitty-Chat at Lamport time %d", evt.ClientJoin.ClientId, receivedTimestamp)
		case *proto.StreamResponse_ClientLeave:
			log.Printf("Participant %d Left Chitty-Chat at Lamport time %d", evt.ClientLeave.ClientId, receivedTimestamp)
		case *proto.StreamResponse_ClientMessage:
			log.Printf("Received Msg %s originally from Participant %d at Lamport time %d", evt.ClientMessage.Message, evt.ClientMessage.ClientId, c.timestamp)
		case *proto.StreamResponse_ServerShutdown:
			log.Printf("Server is shutting down!")
			return nil
		default:
			log.Println("unexpected event from the server")
			return nil
		}
	}
}
func connectToServer() (proto.ChatServiceClient, error) {
	// Dial the server at the specified port.
	conn, err := grpc.Dial("localhost:"+strconv.Itoa(*serverPort), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Could not connect to port %d", *serverPort)
	} else {
		log.Printf("Connected to the server at port %d\n", *serverPort)
	}
	return proto.NewChatServiceClient(conn), nil
}

// Create a client
// client := &Client{
// 	id:         *clientId,
// 	portNumber: *clientPort,
// 	timestamp:  0,
// }
//done := make(chan struct{}) // Create a channel to signal termination

//go waitForCommand(client, done) // Wait for input from the client terminal

//<-done // Wait for the client to be done

func waitForCommand(client *Client, done chan struct{}) {
	// Connect to the server
	serverConnection, _ := connectToServer()

	// Wait for input in the client terminal
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		input := scanner.Text()
		if input == "j" {
			// Join the chat
			_, err := serverConnection.Join(context.Background(), &proto.Client{ClientId: client.id})
			if err != nil {
				log.Fatalf("Failed to join: %v", err)
			} else {
				log.Printf("Joined the chat as client ID %d", client.id)
			}
		}
		if input == "l" {
			// Leave the chat
			_, err := serverConnection.Leave(context.Background(), &proto.Client{ClientId: client.id})
			if err != nil {
				log.Fatalf("Failed to leave: %v", err)
			} else {
				log.Printf("Left the chat as client ID %d", client.id)
			}
			close(done)
		}
		if input == "m" {
			//publish message
			// scan one line of input from stdin as the message text
			// Publish a message
			log.Print("Enter your message: ")
			//client.send(serverConnection, scanner)

		}

	}
}

// func main() {
// 	conn, err := grpc.Dial("localhost:50051", grpc.WithInsecure())
// 	if err != nil {
// 		log.Fatalf("Failed to connect: %v", err)
// 	}
// 	defer conn.Close()

// 	client := pb.NewChatServiceClient(conn)

// 	// Join the chat
// 	clientID := int64(time.Now().UnixNano())
// 	// Join the chat
// 	_, err = client.Join(context.Background(), &pb.Client{ClientId: clientID})
// 	if err != nil {
// 		log.Fatalf("Failed to join: %v", err)
// 	}
// 	log.Printf("Joined the chat as client ID %d", clientID)

// 	// Publish messages
// 	for i := 1; i <= 5; i++ {
// 		msg := &pb.ChatMessage{
// 			Text: fmt.Sprintf("Message %d from %s", i, clientID),
// 		}

// 		_, err := client.Publish(context.Background(), msg)
// 		if err != nil {
// 			log.Printf("Failed to publish: %v", err)
// 		} else {
// 			log.Printf("Published: %s", msg.Text)
// 		}
// 	}

// 	// Leave the chat
// 	_, err = client.Leave(context.Background(), &pb.Client{ClientId: clientID})
// 	if err != nil {
// 		log.Fatalf("Failed to leave: %v", err)
// 	}
// 	log.Printf("Left the chat")

// 	// Receive chat messages
// 	ctx := context.Background()
// 	stream, err := client.GetChatLog(ctx, &emptypb.Empty{})
// 	if err != nil {
// 		log.Fatalf("Error receiving chat messages: %v", err)
// 	}

// 	for {
// 		msg, err := stream.Recv()
// 		if err != nil {
// 			st, ok := status.FromError(err)
// 			if ok && st.Code() == codes.Canceled {
// 				// The stream was canceled, which means the server terminated.
// 				break
// 			}
// 			log.Fatalf("Error receiving chat message: %v", err)
// 		}

// 		log.Printf("Received: [%d] %s", msg.Timestamp, msg.Text)
// 	}
// }
