package veracity

import "testing"

func Test_lastActivityRFC3339(t *testing.T) {
	type args struct {
		idmassif string
		idseal   string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			args: args{
				idmassif: "019107fb65391e3e00",
				idseal:   "0191048b865a073f00",
			},
			want: "2024-07-31T08:50:01Z",
		},
		{
			args: args{
				idmassif: "0191048b865a073f00",
				idseal:   "019107fb65391e3e00",
			},
			want: "2024-07-31T08:50:01Z",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := lastActivityRFC3339(tt.args.idmassif, tt.args.idseal); got != tt.want {
				t.Errorf("lastActivityRFC3339() = %v, want %v", got, tt.want)
			}
		})
	}
}
