package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEventListFromJson(t *testing.T) {
	type args struct {
		data []byte
	}
	tests := []struct {
		name     string
		args     args
		expected []byte
		wantErr  bool
	}{
		{
			name: "nil",
			args: args{
				data: nil,
			},
			expected: nil,
			wantErr:  true,
		},
		{
			name: "empty",
			args: args{
				data: []byte{},
			},
			expected: nil,
			wantErr:  true,
		},
		// We do need this, since we expect input from other processes via pipes (i.e. an events query)
		{
			name: "empty list",
			args: args{
				data: []byte(`{"events":[]}`),
			},
			expected: []byte(`{"events":[]}`),
			wantErr:  false,
		},
		{
			name: "single event",
			args: args{
				data: []byte(`{"identity":"assets/1/events/2"}`),
			},
			expected: []byte(`{"events":[{"identity":"assets/1/events/2"}]}`),
			wantErr:  false,
		},
		{
			name: "single list",
			args: args{
				data: []byte(`{"events":[{"identity":"assets/1/events/2"}]}`),
			},
			expected: []byte(`{"events":[{"identity":"assets/1/events/2"}]}`),
			wantErr:  false,
		},
		{
			name: "multiple list",
			args: args{
				data: []byte(`{"events":[{"identity":"assets/1/events/2"},{"identity":"assets/1/events/3"}]}`),
			},
			expected: []byte(`{"events":[{"identity":"assets/1/events/2"},{"identity":"assets/1/events/3"}]}`),
			wantErr:  false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actual, err := eventListFromJson(test.args.data)

			assert.Equal(t, test.wantErr, err != nil)
			assert.Equal(t, test.expected, actual)
		})
	}
}

// Test_appDomain tests:
//
// 1. a list of assetsv2 events return assetsv2 app domain
// 2. a list of eventsv1 events reutrn eventsv1 app domain
func Test_appDomain(t *testing.T) {
	type args struct {
		appData []byte
	}
	tests := []struct {
		name     string
		args     args
		expected byte
	}{
		{
			name: "positive assetsv2",
			args: args{
				appData: singleAssetsv2EventJsonList,
			},
			expected: 0,
		},
		{
			name: "positive eventsv1",
			args: args{
				appData: singleEventsv1EventJsonList,
			},
			expected: 1,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actual := appDomain(test.args.appData)

			assert.Equal(t, test.expected, actual)
		})
	}
}
