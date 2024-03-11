package biz

import (
	"reflect"
	"testing"

	"github.com/docker/docker/api/types/system"
)

func Test_ParseDeviceMapperStats(t *testing.T) {
	type args struct {
		info system.Info
	}
	tests := []struct {
		name    string
		args    args
		want    *DeviceMapperStats
		wantErr bool
	}{
		{
			name: "parse ok",
			args: args{
				info: system.Info{
					Driver: "devicemapper",
					DriverStatus: [][2]string{
						{"Data Space Used", "1920.92 MB"},
						{"Data Space Total", "102 GB"},
						{"Data Space Available", "100.13 GB"},
						{"Metadata Space Used", "147.5 kB"},
						{"Metadata Space Total", "1.07 GB"},
						{"Metadata Space Available", "1.069 GB"},
					},
				},
			},
			want: &DeviceMapperStats{
				DataUsed:             1920920000,
				DataTotal:            102000000000,
				DataAvailable:        100130000000,
				DataUsagePercent:     1.8832549019607843,
				MetadataUsed:         147500,
				MetadataTotal:        1070000000,
				MetadataAvailable:    1069000000,
				MetadataUsagePercent: 0.013785046728971963,
			},
		},
		{
			name: "missing metrics",
			args: args{
				info: system.Info{
					Driver: "devicemapper",
					DriverStatus: [][2]string{
						{"Data Space Used", "1920.92 MB"},
						{"Data Space Available", "100.13 GB"},
						{"Metadata Space Used", "147.5 kB"},
						{"Metadata Space Available", "1.069 GB"},
					},
				},
			},
			want: &DeviceMapperStats{
				DataUsed:             1920920000,
				DataAvailable:        100130000000,
				DataUsagePercent:     0.0,
				MetadataUsed:         147500,
				MetadataAvailable:    1069000000,
				MetadataUsagePercent: 0.0,
			},
		},
		{
			name: "not supported Driver",
			args: args{
				info: system.Info{
					Driver: "overlay2",
				},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "empty DriverStatus",
			args: args{
				info: system.Info{
					Driver: "devicemapper",
				},
			},
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseDeviceMapperStats(tt.args.info)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseDeviceMapperStats() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseDeviceMapperStats() = %v, want %v", got, tt.want)
			}
		})
	}
}
