package main

import "testing"

func Test_extractURL(t *testing.T) {
	type args struct {
		message string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "simple url extraction",
			args: args{message: "lorem ipsum https://www.bla.com/?x.pdf huhuhu"},
			want: "https://www.bla.com/?x.pdf",
		},
		{
			name: "takes the first link if mulitple urls are provided url extraction",
			args: args{message: "lorem ipsum https://www.bla.com/?x.pdf some more https://www.bla.com/?y.pdf huhuhu"},
			want: "https://www.bla.com/?x.pdf",
		},
		{
			name: "returns empty string if there is no match",
			args: args{message: "no url here"},
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := extractURL(tt.args.message); got != tt.want {
				t.Errorf("extractURL() = %v, want %v", got, tt.want)
			}
		})
	}
}
