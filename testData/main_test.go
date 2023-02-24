package main

import "testing"

func Test_getSomeField(t *testing.T) {
	type args struct {
		someStruct SomeStruct
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{"test1", args{SomeStruct{"test", 1}}, "test add some text"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getSomeField(tt.args.someStruct); got != tt.want {
				t.Errorf("getSomeField() = %v, want %v", got, tt.want)
			}
		})
	}
}
