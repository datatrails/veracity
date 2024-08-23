package veracity

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"testing"

	"github.com/datatrails/go-datatrails-common/logger"
	"github.com/datatrails/go-datatrails-logverification/logverification"
	"github.com/datatrails/go-datatrails-merklelog/massifs"
	"github.com/stretchr/testify/assert"
)

type MockMR struct {
	data map[uint64][]byte
}

func (*MockMR) GetFirstMassif(ctx context.Context, tenantIdentity string, opts ...massifs.ReaderOption) (massifs.MassifContext, error) {
	return massifs.MassifContext{}, fmt.Errorf("not implemented")
}
func (*MockMR) GetHeadMassif(ctx context.Context, tenantIdentity string, opts ...massifs.ReaderOption) (massifs.MassifContext, error) {
	return massifs.MassifContext{}, fmt.Errorf("not implemented")
}
func (*MockMR) GetLazyContext(ctx context.Context, tenantIdentity string, which massifs.LogicalBlob, opts ...massifs.ReaderOption) (massifs.LogBlobContext, uint64, error) {
	return massifs.LogBlobContext{}, 0, fmt.Errorf("not implemented")
}
func (m *MockMR) GetMassif(ctx context.Context, tenantIdentity string, massifIndex uint64, opts ...massifs.ReaderOption) (massifs.MassifContext, error) {
	mc := massifs.MassifContext{}
	data, ok := m.data[massifIndex]
	if !ok {
		return mc, fmt.Errorf("massif not found")
	}
	mc.Data = data
	return mc, nil
}

func (m *MockMR) GetHeadVerifiedContext(
	ctx context.Context, tenantIdentity string,
	opts ...massifs.ReaderOption,
) (*massifs.VerifiedContext, error) {
	return nil, errors.New("not implemented")
}

func (m *MockMR) GetVerifiedContext(
	ctx context.Context, tenantIdentity string, massifIndex uint64,
	opts ...massifs.ReaderOption,
) (*massifs.VerifiedContext, error) {
	return nil, errors.New("not implemented")
}

func NewMockMR(massifIndex uint64, data string) *MockMR {
	b, e := hex.DecodeString(data)
	if e != nil {
		return nil
	}
	return &MockMR{
		data: map[uint64][]byte{massifIndex: b},
	}
}

func TestVerifyEvent(t *testing.T) {
	logger.New("TestVerifyList")
	defer logger.OnExit()
	event := []byte(`{
		"identity": "publicassets/87dd2e5a-42b4-49a5-8693-97f40a5af7f8/events/a022f458-8e55-4d63-a200-4172a42fc2aa", 
		"asset_identity": "publicassets/87dd2e5a-42b4-49a5-8693-97f40a5af7f8", 
		"event_attributes": {
			"arc_access_policy_asset_attributes_read":  [ 
				{"attribute" :"*","0x4609ea6bbe85F61bc64760273ce6D89A632B569f" :"wallet","SmL4PHAHXLdpkj/c6Xs+2br+hxqLmhcRk75Hkj5DyEQ=" :"tessera"}], 
			"arc_access_policy_event_arc_display_type_read":  [ 
				{"SmL4PHAHXLdpkj/c6Xs+2br+hxqLmhcRk75Hkj5DyEQ=" :"tessera","value" :"*","0x4609ea6bbe85F61bc64760273ce6D89A632B569f" :"wallet"}], 
			"arc_access_policy_always_read":  [ 
				{"wallet" :"0x0E29670b420B7f2E8E699647b632cdE49D868dA7","tessera" :"SmL4PHAHXLdpkj/c6Xs+2br+hxqLmhcRk75Hkj5DyEQ="}]
		}, 
		"asset_attributes": {"arc_display_name": "Dava Derby", "arc_display_type": "public-test"}, 
		"operation": "NewAsset", 
		"behaviour": "AssetCreator", 
		"timestamp_declared": "2024-05-24T07:26:58Z", 
		"timestamp_accepted": "2024-05-24T07:26:58Z", 
		"timestamp_committed": "2024-05-24T07:27:00.200Z", 
		"principal_declared": {"issuer":"", "subject":"", "display_name":"", "email":""}, 
		"principal_accepted": {"issuer":"", "subject":"", "display_name":"", "email":""}, 
		"confirmation_status": "CONFIRMED", 
		"transaction_id": "0xc891533b1806555fff9ab853cd9ce1bb2c00753609070a875a44ec53a6c1213b", 
		"block_number": 7932, 
		"transaction_index": 1, 
		"from": "0x0E29670b420B7f2E8E699647b632cdE49D868dA7", 
		"tenant_identity": "tenant/7dfaa5ef-226f-4f40-90a5-c015e59998a8", 
		"merklelog_entry": {"commit":{"index":"0", "idtimestamp":"018fa97ef269039b00"}, 
		"confirm":{
			"mmr_size":"7", 
			"root":"/rlMNJhlay9CUuO3LgX4lSSDK6dDhtKesCO50CtrHr4=", 
			"timestamp":"1716535620409", 
			"idtimestamp":"", 
			"signed_tree_head":""}, 
		"unequivocal":null}
	}`)

	eventOK, _ := logverification.NewVerifiableEvent(event)

	justDecode := func(in string) []byte {
		b, _ := hex.DecodeString(in)
		return b
	}

	tests := []struct {
		name          string
		event         *logverification.VerifiableEvent
		massifReader  MassifReader
		expectedProof [][]byte
		expectedError bool
	}{
		{
			name:  "smiple OK",
			event: eventOK,
			massifReader: NewMockMR(0,
				//       7
				//    3       6
				// 1    2  4     5
				"000000000000000090757516a9086b0000000000000000000000010e00000000"+
					"0000000000000000000000000000000000000000000000000000000000000000"+
					"0000000000000000000000000000000000000000000000000000000000000000"+
					"0000000000000000000000000000000000000000000000000000000000000000"+
					"0000000000000000000000000000000000000000000000000000000000000000"+
					"0000000000000000000000000000000000000000000000000000000000000000"+
					"0000000000000000000000000000000000000000000000000000000000000000"+
					"0000000000000000000000000000000000000000000000000000000000000000"+
					"0000000000000000000000000000000000000000000000000000000000000000"+
					"0000000000000000000000000000000000000000000000000000000000000000"+
					"0000000000000000000000000000000000000000000000000000000000000000"+
					"bfc511ab1b880b24bb2358e07472e3383cdeddfbc4de9d66d652197dfb2b6633"+ // this is  hash(event)
					"0000000000000000000000000000000000000000000000000000000000000002"+ // leaf 2
					"dbdc36cf2b46382c810ecef9a423bf7e7f222d72c221baf2e840cc94428e966c"+ // this is node 1-2 hash(3 + hash(event) + leaf 2)
					"0000000000000000000000000000000000000000000000000000000000000004"+ // leaf 3
					"0000000000000000000000000000000000000000000000000000000000000005"+ // leaf 4
					"0000000000000000000000000000000000000000000000000000000000000006"+ // node 3 - 4
					"c1dc2d0cf9982d94f97597193cce3a42c21a1b02c346c0fada0aa1d48ed2089f", // this is root hash(7 + node 1-2 + node 3-4)
			),
			expectedError: false,
			expectedProof: [][]byte{
				justDecode("0000000000000000000000000000000000000000000000000000000000000002"),
				justDecode("0000000000000000000000000000000000000000000000000000000000000006"),
			},
		},
		{
			name:          "No mmr log",
			event:         eventOK,
			massifReader:  NewMockMR(6, "000000000000000090757516a9086b0000000000000000000000010e00000000"),
			expectedError: true,
		},
		{
			name:  "Not valid proof",
			event: eventOK,
			massifReader: NewMockMR(0,
				//       7
				//    3       6
				// 1    2  4     5
				"000000000000000090757516a9086b0000000000000000000000010e00000000"+
					"0000000000000000000000000000000000000000000000000000000000000000"+
					"0000000000000000000000000000000000000000000000000000000000000000"+
					"0000000000000000000000000000000000000000000000000000000000000000"+
					"0000000000000000000000000000000000000000000000000000000000000000"+
					"0000000000000000000000000000000000000000000000000000000000000000"+
					"0000000000000000000000000000000000000000000000000000000000000000"+
					"0000000000000000000000000000000000000000000000000000000000000000"+
					"0000000000000000000000000000000000000000000000000000000000000000"+
					"0000000000000000000000000000000000000000000000000000000000000000"+
					"0000000000000000000000000000000000000000000000000000000000000000"+
					"bfc511ab1b880b24bb2358e07472e3383cdeddfbc4de9d66d652197dfb2b6633"+ // this is  hash(event)
					"0000000000000000000000000000000000000000000000000000000000000002"+ // leaf 2
					"dbdc36cf2b46382c810ecef9a423bf7e7f222d72c221baf2e840cc94428e966c"+ // this is node 1-2 hash(3 + hash(event) + leaf 2)
					"0000000000000000000000000000000000000000000000000000000000000004"+ // leaf 3
					"0000000000000000000000000000000000000000000000000000000000000005"+ // leaf 4
					"0000000000000000000000000000000000000000000000000000000000000006"+ // node 3 - 4
					"c1dc2d0cf9982d94f97597193cce3a42c21a1b02c346c0fada0aa1d48ed208ff", // this is fake root hash - end if ff instead of 9f
			),
			expectedError: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			proof, err := verifyEvent(tc.event, defaultMassifHeight, tc.massifReader, "", "")

			if tc.expectedError {
				assert.NotNil(t, err, "expected error got nil")
			} else {
				assert.Nil(t, err, "unexpected error")
				assert.Equal(t, tc.expectedProof, proof)
			}
		})
	}
}
