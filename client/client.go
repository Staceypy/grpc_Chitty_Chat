package main

import (
	"bufio"
	"context"
	"flag" // Adjust the import path
	proto "handin3/grpc"
	"log"
	"os"
	"strconv"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Client struct {
	id         int64
	portNumber int
	timestamp  int64
}

var (
	clientId   = flag.Int64("cId", 0, "client id")
	clientPort = flag.Int("cPort", 5454, "client port number")
	serverPort = flag.Int("sPort", 8080, "server port number (should match the port used for the server)")
)

func main() {
	// Parse the flags to get the port for the client
	flag.Parse()

	// Create a client
	client := &Client{
		id:         *clientId,
		portNumber: *clientPort,
		timestamp:  0,
	}
	done := make(chan struct{}) // Create a channel to signal termination

	go waitForCommand(client, done) // Wait for input from the client terminal

	<-done // Wait for the client to be done

}

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
