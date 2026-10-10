package config

import (
	"io"
	"testing"
)

func TestLoad(t *testing.T) {
	tests := []struct {
		name string
		args []string
		env  map[string]string
		want Config
	}{
		{
			name: "defaults",
			want: Config{HTTPAddr: ":8085", GRPCAddr: ":8086", MocksDir: "mocks"},
		},
		{
			name: "environment overrides defaults",
			env:  map[string]string{"PROTOMOCK_HTTP_ADDR": ":9085", "PROTOMOCK_MOCKS_DIR": "/data/mocks"},
			want: Config{HTTPAddr: ":9085", GRPCAddr: ":8086", MocksDir: "/data/mocks"},
		},
		{
			name: "flags override environment",
			args: []string{"-http-addr", ":7085", "-grpc-addr=:7086", "-web-dir", "/srv/web"},
			env:  map[string]string{"PROTOMOCK_HTTP_ADDR": ":9085"},
			want: Config{HTTPAddr: ":7085", GRPCAddr: ":7086", MocksDir: "mocks", WebDir: "/srv/web"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			getenv := func(key string) string { return tt.env[key] }
			got, err := Load(tt.args, getenv, io.Discard)
			if err != nil {
				t.Fatalf("Load() error: %v", err)
			}
			if got != tt.want {
				t.Errorf("Load() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestLoad_UnknownFlag(t *testing.T) {
	if _, err := Load([]string{"-nope"}, func(string) string { return "" }, io.Discard); err == nil {
		t.Error("Load() with an unknown flag returned nil error")
	}
}
