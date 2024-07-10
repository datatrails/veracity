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

func WithFakeDirLister(dl DirLister) Option {
	return func(o *LocalMassifReaderOptions) {
		o.isDir = true
		o.dirLister = dl
	}
}

func TestNewLocalMassifReader(t *testing.T) {
	logger.New("TestVerifyList")
	defer logger.OnExit()

	dl := mocks.NewDirLister(t)
	dl.On("ListFiles", mock.Anything).Return(
		func(name string) ([]string, error) {
			switch name {
			case "/same/log":
				return []string{"/same/log/0.log", "/same/log/1.log"}, nil
			case "/logs/invalid/":
				return []string{"/logs/invalid/0.log", "/logs/invalid/invalid.log"}, nil
			case "/logs/short":
				return []string{"/logs/short/0.log", "/logs/short/1.log"}, nil
			case "/logs/valid":
				return []string{"/logs/valid/0.log", "/logs/valid/1.log"}, nil
			case "/logs/valid3":
				return []string{"/logs/valid3/0.log", "/logs/valid3/1.log", "/logs/valid3/255.log"}, nil
			default:
				return []string{}, nil
			}
		},
	)

	// this mock returns headers of logfiles
	// signigicant bytes we use in test are 27 for mmr height
	// and last 4 (28-32) for mmr index
	op := mocks.NewOpener(t)
	op.On("Open", mock.Anything).Return(
		func(name string) (io.ReadCloser, error) {
			switch name {
			case "/foo/bar/log.log":
				return nil, fmt.Errorf("bad file log.log")
			case "/log/massif/0.log":
				b, _ := hex.DecodeString("000000000000000090757516a9086b0000000000000000000000010e00000000")
				return io.NopCloser(bytes.NewReader(b)), nil
			case "/same/log/0.log":
				b, _ := hex.DecodeString("000000000000000090757516a9086b0000000000000000000000010e00000000")
				return io.NopCloser(bytes.NewReader(b)), nil
			case "/same/log/1.log":
				b, _ := hex.DecodeString("000000000000000090757515a9086b0000000000000000000000010e00000000")
				return io.NopCloser(bytes.NewReader(b)), nil
			case "/logs/invalid/0.log":
				b, _ := hex.DecodeString("000000000000000090757515a9086b0000000000000000000000010e00000000")
				return io.NopCloser(bytes.NewReader(b)), nil
			case "/logs/invalid/invalid.log":
				b, _ := hex.DecodeString("000000000000000090757515a9086b0000000000000000000000010f00000000")
				return io.NopCloser(bytes.NewReader(b)), nil
			case "/logs/short/0.log":
				b, _ := hex.DecodeString("000000000000000090757515a9086b0000000000000000000000010e00000000")
				return io.NopCloser(bytes.NewReader(b)), nil
			case "/logs/short/1.log":
				b, _ := hex.DecodeString("00000000000000009075")
				return io.NopCloser(bytes.NewReader(b)), nil
			case "/logs/valid/0.log":
				b, _ := hex.DecodeString("000000000000000090757515a9086b0000000000000000000000010e00000000")
				return io.NopCloser(bytes.NewReader(b)), nil
			case "/logs/valid/1.log":
				b, _ := hex.DecodeString("000000000000000090757515a9086b0000000000000000000000010e00000007")
				return io.NopCloser(bytes.NewReader(b)), nil
			case "/logs/valid3/0.log":
				b, _ := hex.DecodeString("000000000000000090757515a9086b0000000000000000000000010e00000000")
				return io.NopCloser(bytes.NewReader(b)), nil
			case "/logs/valid3/1.log":
				b, _ := hex.DecodeString("000000000000000090757515a9086b0000000000000000000000010e00000007")
				return io.NopCloser(bytes.NewReader(b)), nil
			case "/logs/valid3/255.log":
				b, _ := hex.DecodeString("000000000000000090757515a9086b0000000000000000000000010e000000ff")
				return io.NopCloser(bytes.NewReader(b)), nil
			default:
				return nil, nil
			}
		},
	)

	tests := []struct {
		name       string
		opener     Opener
		dirlister  DirLister
		logs       string
		isdir      bool
		outcome    map[uint64]string
		expectErr  bool
		errMessage string
	}{
		{
			name:      "log 0 valid",
			opener:    op,
			dirlister: dl,
			expectErr: false,
			logs:      "/log/massif/0.log",
			isdir:     false,
			outcome:   map[uint64]string{0: "/log/massif/0.log"},
		},
		{
			name:       "fail both args specified",
			opener:     op,
			dirlister:  dl,
			logs:       "",
			expectErr:  true,
			errMessage: "--data-local must be specified",
		},
		{
			name:       "fail two logs same index",
			opener:     op,
			dirlister:  dl,
			logs:       "/same/log",
			isdir:      true,
			expectErr:  true,
			errMessage: "found two log files with the same massif index: /same/log/0.log and /same/log/1.log",
		},
		{
			name:      "valid + invalid height not default",
			opener:    op,
			dirlister: dl,
			expectErr: false,
			logs:      "/logs/invalid/",
			isdir:     true,
			outcome:   map[uint64]string{0: "/logs/invalid/0.log"},
		},
		{
			name:      "valid + short file",
			opener:    op,
			dirlister: dl,
			expectErr: false,
			logs:      "/logs/short",
			isdir:     true,
			outcome:   map[uint64]string{0: "/logs/short/0.log"},
		},
		{
			name:      "two valid",
			opener:    op,
			dirlister: dl,
			expectErr: false,
			logs:      "/logs/valid",
			isdir:     true,
			outcome: map[uint64]string{
				0: "/logs/valid/0.log",
				7: "/logs/valid/1.log",
			},
		},
		{
			name:      "three valid",
			opener:    op,
			dirlister: dl,
			expectErr: false,
			logs:      "/logs/valid3",
			isdir:     true,
			outcome: map[uint64]string{
				0:   "/logs/valid3/0.log",
				7:   "/logs/valid3/1.log",
				255: "/logs/valid3/255.log",
			},
		},
		{
			name:       "fail empty config",
			expectErr:  true,
			errMessage: "--data-local must be specified",
		},
		{
			name:       "fail on bad file",
			opener:     op,
			dirlister:  dl,
			expectErr:  true,
			isdir:      false,
			logs:       "/foo/bar/log.log",
			errMessage: "bad file log.log",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			opts := []Option{}
			if tc.isdir {
				opts = append(opts, WithFakeDirLister(tc.dirlister))
			}
			r, err := NewLocalMassifReader(logger.Sugar, tc.opener, tc.logs, opts...)

			if tc.expectErr {
				assert.NotNil(t, err, "expected error got nil")
				assert.Equal(t, tc.errMessage, err.Error(), "unexpected error message")
			} else {
				assert.Nil(t, err, "unexpected error")
				assert.Equal(t, tc.outcome, r.massifs)
			}
		})
	}
}
