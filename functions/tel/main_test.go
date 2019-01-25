package main

import "testing"

func Test_parseTo(t *testing.T) {
	type args struct {
		toAddress string
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			name: "basic",
			args: args{
				toAddress: "tel+6584812030@dev.unee-t.com",
			},
			want:    "+6584812030",
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseTo(tt.args.toAddress)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseTo() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("parseTo() = %v, want %v", got, tt.want)
			}
		})
	}
}
