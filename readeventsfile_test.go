//go:build integration && azurite

package veracity

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEventListFromData(t *testing.T) {
	type args struct {
		data []byte
	}
	tests := []struct {
		name    string
		args    args
		want    []byte
		wantErr bool
	}{
		{
			name: "nil",
			args: args{
				data: nil,
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "empty",
			args: args{
				data: []byte{},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "empty list malformed",
			args: args{
				data: []byte(`[]`),
			},
			want:    nil,
			wantErr: true,
		},
		// We do need this, since we expect input from other processes via pipes (i.e. an events query)
		// {
		// 	name: "empty list",
		// 	args: args{
		// 		data: []byte(`{"events": []}`),
		// 	},
		// 	want:    []byte(`{"events":[]}`),
		// 	wantErr: false,
		// },
		{
			name: "single not list",
			args: args{
				data: []byte(`{"identity": "assets/1/events/2"}`),
			},
			want:    []byte(`{"events":[{"identity":"assets/1/events/2"}]}`),
			wantErr: false,
		},
		{
			name: "single list",
			args: args{
				data: []byte(`{"events": [{"identity": "assets/1/events/2"}]}`),
			},
			want:    []byte(`{"events":[{"identity":"assets/1/events/2"}]}`),
			wantErr: false,
		},
		{
			name: "multiple list",
			args: args{
				data: []byte(`{"events": [{"identity": "assets/1/events/2"},{"identity": "assets/1/events/3"}]}`),
			},
			want:    []byte(`{"events":[{"identity":"assets/1/events/2"},{"identity":"assets/1/events/3"}]}`),
			wantErr: false,
		},
		{
			name: "single list malformed",
			args: args{
				data: []byte(`[{"identity": "assets/1/events/2"}]`),
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "multiple list malformed",
			args: args{
				data: []byte(`[{"identity":"assets/1/events/2"},{"identity":"assets/1/events/3"}]`),
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "multiple not list malformed",
			args: args{
				data: []byte(`{"identity":"assets/1/events/2"},{"identity": "assets/1/events/3"}`),
			},
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := EventListFromData(tt.args.data)

			assert.Equal(t, tt.wantErr, err != nil)
			assert.Equal(t, tt.want, got)
		})
	}
}
