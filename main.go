// shippy-cli-consignment/main.go

package main

import (
	"context"
	"encoding/json"
	"github.com/micro/go-micro/v2"
	pb "github.com/suliar/shippy-service-consignment/proto/consignment"
	"os"

	"io/ioutil"
	"log"
)

const (
	address = "localhost:50051"
	defaultFilename = "consignment.json"
)

func parseFile(file string) (*pb.Consignment, error) {
	var consignment *pb.Consignment
	data, err := ioutil.ReadFile(file)

	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(data, &consignment)
	if err != nil {
		return nil, err
	}

	return consignment, nil

}

func main() {

	// Set up a connection to the server
	//conn, err := grpc.Dial(address, grpc.WithInsecure())
	////if err != nil {
	////	log.Fatalf("Did not connect: %v", err)
	////}
	////defer conn.Close()
	////
	////client := pb.NewShippingServiceClient(conn)

	service := micro.NewService(micro.Name("shippy.client.consignment"))


	service.Init()


	client := pb.NewShippingService("shippy.service.consignment", service.Client())



	// Contact the server and print out its response

	file := defaultFilename
	if len(os.Args) > 1 {
		file = os.Args[1]
	}


	consignment, err := parseFile(file)

	if err != nil {
		log.Fatalf("Could not parse file: %v", err)
	}


	cc, err := client.CreateConsignment(context.Background(), consignment)
	if err != nil {
		log.Fatalf("Could not create consignment: %v", err)
	}

	log.Printf("Created: %t", cc.Created)

	getAll, err := client.GetConsignments(context.Background(), &pb.GetRequest{})
	if err != nil {
		log.Fatalf("Could not list consignments: %v", err)
	}

	for _, v := range getAll.Consignments {
		log.Println(v)
	}


}
