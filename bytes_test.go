package creader

import (
	"testing"
)

func TestByteCountDecimal(t *testing.T) {
	type args struct {
		b int64
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{"bytes", args{b: 2}, "2 B"},
		{"kilobytes", args{b: 2 * 1 << 10}, "2.0 kB"},
		{"megabytes", args{b: 2 * 1 << 20}, "2.1 MB"},
		{"gigabytes", args{b: 2 * 1 << 30}, "2.1 GB"},
		{"terabytes", args{b: 2 * 1 << 40}, "2.2 TB"},
		{"petabytes", args{b: 2 * 1 << 50}, "2.3 PB"},
		{"exabytes", args{b: 2 * 1 << 60}, "2.3 EB"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ByteCountDecimal(tt.args.b); got != tt.want {
				t.Errorf("ByteCountDecimal() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestByteCountBinary(t *testing.T) {
	type args struct {
		b int64
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{"bytes", args{b: 2}, "2 B"},
		{"kibibytes", args{b: 2 * 1 << 10}, "2.0 KiB"},
		{"mebibytes", args{b: 2 * 1 << 20}, "2.0 MiB"},
		{"gibibytes", args{b: 2 * 1 << 30}, "2.0 GiB"},
		{"tebibytes", args{b: 2 * 1 << 40}, "2.0 TiB"},
		{"pebibytes", args{b: 2 * 1 << 50}, "2.0 PiB"},
		{"exbibytes", args{b: 2 * 1 << 60}, "2.0 EiB"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ByteCountBinary(tt.args.b); got != tt.want {
				t.Errorf("ByteCountBinary() = %v, want %v", got, tt.want)
			}
		})
	}
}
