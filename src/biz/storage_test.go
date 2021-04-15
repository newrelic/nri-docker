package biz

import (
	"reflect"
	"testing"

	"github.com/docker/docker/api/types"
)

func Test_ParseDeviceMapperStats(t *testing.T) {
	type args struct {
		info types.Info
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
				info: types.Info{
					Driver: "devicemapper",
					DriverStatus: [][2]string{
						{"Data Space Used", "19.92 MB"},
						{"Data Space Total", "102 GB"},
						{"Data Space Available", "102 GB"},
						{"Metadata Space Used", "147.5 kB"},
						{"Metadata Space Total", "1.07 GB"},
						{"Metadata Space Available", "1.069 GB"},
					},
				},
			},
			want: &DeviceMapperStats{
				DataUsed:          19920000,
				DataTotal:         102000000000,
				DataAvailable:     102000000000,
				MetadataUsed:      147500,
				MetadataTotal:     1070000000,
				MetadataAvailable: 1069000000,
			},
		},
		{
			name: "not supported Driver",
			args: args{
				info: types.Info{
					Driver: "overlay2",
				},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "empty DriverStatus",
			args: args{
				info: types.Info{
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
