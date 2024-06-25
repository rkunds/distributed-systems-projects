package common

import "google.golang.org/protobuf/types/known/timestamppb"

type DataMessage struct {
	Transaction   string
	TransactionID uint64
	SenderNode    string

	Priority    uint64
	PropserNode string

	Deliverable bool
} 

type ChanStruct struct {
	Message string
	
	GenTime *timestamppb.Timestamp
}
