package veracity

import (
	"github.com/datatrails/go-datatrails-merklelog/massifs/snowflakeid"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func NewTimestamp(id uint64, epoch uint8) (*timestamppb.Timestamp, error) {
	ts := &timestamppb.Timestamp{}
	err := SetTimestamp(id, ts, epoch)
	if err != nil {
		return nil, err
	}
	return ts, nil
}

func SetTimestamp(id uint64, ts *timestamppb.Timestamp, epoch uint8) error {
	ms, err := snowflakeid.IDUnixMilli(id, epoch)
	if err != nil {
		return err
	}

	ts.Seconds = ms / 1000
	ts.Nanos = int32(uint64(ms)-(uint64(ts.GetSeconds())*1000)) * 1e6

	return nil
}
