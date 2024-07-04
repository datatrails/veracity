package veracity

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"io"
	"testing"

	"github.com/datatrails/go-datatrails-common/logger"
	"github.com/datatrails/veracity/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestNewLocalMassifReader(t *testing.T) {
	logger.New("TestVerifyList")
	defer logger.OnExit()

	dl := mocks.NewDirLister(t)
	dl.On("ListFiles", mock.Anything).Return()

	op := mocks.NewOpener(t)
	op.On("Open", mock.Anything).Return(
		func(name string) (io.ReadCloser, error) {
			switch name {
			case "/foo/bar/log.log":
				return nil, fmt.Errorf("bad file log.log")
			case "/log/massif/0.log":
				b, _ := hex.DecodeString("000000000000000090757516a9086b0000000000000000000000010e00000000")
				return io.NopCloser(bytes.NewReader(b)), nil
			default:
				return nil, nil
			}
		},
	)

	tests := []struct {
		name            string
		opener          Opener
		dirlister       DirLister
		logdir, logfile string
		outcome         map[uint64]string
		expectErr       bool
		errMessage      string
	}{
		{
			name:      "log 0 valid",
			opener:    op,
			expectErr: false,
			logfile:   "/log/massif/0.log",
			outcome:   map[uint64]string{0: "/log/massif/0.log"},
		},
		{
			name:      "fail both args specified",
			opener:    op,
			dirlister: dl,
			outcome:   map[uint64]string{},
		},
		{
			name:    "fail two logs same index",
			outcome: map[uint64]string{},
		},
		{
			name:    "valid + invalid height not default",
			outcome: map[uint64]string{},
		},
		{
			name:    "valid + short file",
			outcome: map[uint64]string{},
		},
		{
			name:    "two valid",
			outcome: map[uint64]string{},
		},
		{
			name:    "three valid",
			outcome: map[uint64]string{},
		},
		{
			name:    "fail empty config",
			outcome: map[uint64]string{},
		},
		{
			name:       "fail on bad file",
			opener:     op,
			dirlister:  dl,
			expectErr:  true,
			logfile:    "/foo/bar/log.log",
			errMessage: "bad file log.log",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r, err := NewLocalMassifReader(logger.Sugar, tc.opener, tc.dirlister, tc.logdir, tc.logfile)

			if tc.expectErr {
				assert.NotNil(t, err, "expected error got nil")
				assert.Equal(t, tc.errMessage, err.Error(), "unexpected error message")
			} else {
				assert.Equal(t, tc.outcome, r.massifs)
			}
		})
	}
}
