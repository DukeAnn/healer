package healer

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"strconv"
	"time"

	"github.com/golang/glog"
)

// SimpleConsumer instance is built to consume messages from kafka broker
type SimpleConsumer struct {
	ClientID    string
	Brokers     *Brokers
	BrokerList  string
	TopicName   string
	Partition   uint32
	FetchOffset int64
	MaxBytes    int32
	MaxWaitTime int32
	MinBytes    int32
}

func NewSimpleConsumer(brokers *Brokers) *SimpleConsumer {
	return nil
}

// Consume consume  messages from kafka broker and send them to channels
func (simpleConsumer *SimpleConsumer) Consume(messages chan Message) {
	metadataResponse, err := simpleConsumer.Brokers.RequestMetaData(&simpleConsumer.TopicName)
	if err != nil {
		glog.Fatalf("could not get metadata of topic[%s] from %s", simpleConsumer.TopicName, simpleConsumer.TopicName)
	}

	partitionMetadatas := metadataResponse.TopicMetadatas[0].PartitionMetadatas
	//find leader
	var leader int32
	for _, partitionMetadata := range partitionMetadatas {
		if partitionMetadata.PartitionId == simpleConsumer.Partition {
			leader = partitionMetadata.Leader
			break
		}
	}

	var (
		host string
		port int32
	)
	for _, broker := range metadataResponse.Brokers {
		if broker.NodeId == leader {
			host = broker.Host
			port = broker.Port
		}
	}
	leaderAddr := net.JoinHostPort(host, strconv.Itoa(int(port)))
	conn, err := net.DialTimeout("tcp", leaderAddr, time.Second*5)
	if err != nil {
		glog.Fatal(err)
	}
	defer func() { conn.Close() }()

	correlationID := int32(0)
	partitonBlock := &PartitonBlock{
		Partition:   simpleConsumer.Partition,
		FetchOffset: simpleConsumer.FetchOffset,
		MaxBytes:    simpleConsumer.MaxBytes,
	}
	fetchRequest := FetchRequest{
		ReplicaId:   -1,
		MaxWaitTime: simpleConsumer.MaxWaitTime,
		MinBytes:    simpleConsumer.MinBytes,
		Topics:      map[string][]*PartitonBlock{simpleConsumer.TopicName: []*PartitonBlock{partitonBlock}},
	}
	fetchRequest.RequestHeader = &RequestHeader{
		ApiKey:        API_FetchRequest,
		ApiVersion:    0,
		CorrelationId: correlationID,
		ClientId:      simpleConsumer.ClientID,
	}

	// TODO when stop??
	for {
		payload := fetchRequest.Encode()
		conn.Write(payload)

		buf := make([]byte, 4)
		_, err = conn.Read(buf)
		if err != nil {
			glog.Fatal(err)
		}

		responseLength := int(binary.BigEndian.Uint32(buf))
		fmt.Println(responseLength)
		buf = make([]byte, responseLength)

		readLength := 0
		for {
			length, err := conn.Read(buf[readLength:])
			if err == io.EOF {
				break
			}
			if err != nil {
				glog.Fatal(err)
			}
			readLength += length
			if readLength > responseLength {
				glog.Fatal("fetch more data than needed")
			}
		}
		correlationID := int32(binary.BigEndian.Uint32(buf))
		fetchResponse, err := DecodeFetchResponse(buf[4:])
		if err != nil {
			glog.Fatal(err)
		}

		for _, fetchResponsePiece := range fetchResponse {
			for _, topicData := range fetchResponsePiece.TopicDatas {
				if topicData.ErrorCode == 0 {
					for _, message := range topicData.MessageSet {
						partitonBlock.FetchOffset = message.Offset + 1
						messages <- message
					}
				} else if topicData.ErrorCode == -1 {
					glog.Info(AllError[0].Error())
				} else {
					glog.Info(AllError[topicData.ErrorCode].Error())
				}
			}
		}
		correlationID++
		fetchRequest.RequestHeader.CorrelationId = correlationID
	}
}
