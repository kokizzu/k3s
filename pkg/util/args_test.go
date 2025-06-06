package util

import (
	"reflect"
	"testing"
)

func Test_UnitAddFeatureGate(t *testing.T) {
	type args struct {
		currentArg  string
		featureGate string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "Feature gate added to empty arg",
			args: args{
				currentArg:  "",
				featureGate: "SupportPodPidsLimit=false",
			},
			want: "SupportPodPidsLimit=false",
		},
		{
			name: "Feature gate added to existing arg",
			args: args{
				currentArg:  "SupportPodPidsLimit=false",
				featureGate: "DevicePlugins=false",
			},
			want: "SupportPodPidsLimit=false,DevicePlugins=false",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := AddFeatureGate(tt.args.currentArg, tt.args.featureGate)
			if got != tt.want {
				t.Errorf("error, should be %s, but got %s", tt.want, got)
			}
		})
	}
}

func Test_UnitGetArgs(t *testing.T) {
	type args struct {
		argsMap   map[string]string
		extraArgs []string
	}
	tests := []struct {
		name string
		args args
		want []string
	}{
		{
			name: "Default Test",
			args: args{
				argsMap: map[string]string{
					"aaa": "A",
					"bbb": "B",
					"ccc": "C",
					"ddd": "d",
					"eee": "e",
					"fff": "f",
					"ggg": "g",
					"hhh": "h",
				},
				extraArgs: []string{
					"bbb=BB",
					"ddd=DD",
					"iii=II",
				},
			},

			want: []string{
				"--aaa=A",
				"--bbb=BB",
				"--ccc=C",
				"--ddd=DD",
				"--eee=e",
				"--fff=f",
				"--ggg=g",
				"--hhh=h",
				"--iii=II",
			},
		},
		{
			name: "Args with existing hyphens Test",
			args: args{
				argsMap: map[string]string{
					"aaa": "A",
					"bbb": "B",
					"ccc": "C",
					"ddd": "d",
					"eee": "e",
					"fff": "f",
					"ggg": "g",
					"hhh": "h",
				},
				extraArgs: []string{
					"--bbb=BB",
					"--ddd=DD",
					"--iii=II",
				},
			},

			want: []string{
				"--aaa=A",
				"--bbb=BB",
				"--ccc=C",
				"--ddd=DD",
				"--eee=e",
				"--fff=f",
				"--ggg=g",
				"--hhh=h",
				"--iii=II",
			},
		},
		{
			name: "Multiple args with defaults Test",
			args: args{
				argsMap: map[string]string{
					"aaa": "A",
					"bbb": "B",
				},
				extraArgs: []string{
					"--ccc=C",
					"--bbb=DD",
					"--bbb=AA",
				},
			},

			want: []string{
				"--aaa=A",
				"--bbb=DD",
				"--bbb=AA",
				"--ccc=C",
			},
		},
		{
			name: "Multiple args with defaults and prefix",
			args: args{
				argsMap: map[string]string{
					"aaa": "A",
					"bbb": "B",
				},
				extraArgs: []string{
					"--ccc=C",
					"--bbb-=DD",
				},
			},

			want: []string{
				"--aaa=A",
				"--bbb=DD",
				"--bbb=B",
				"--ccc=C",
			},
		},
		{
			name: "Multiple args with defaults and suffix",
			args: args{
				argsMap: map[string]string{
					"aaa": "A",
					"bbb": "B",
				},
				extraArgs: []string{
					"--ccc=C",
					"--bbb+=DD",
				},
			},

			want: []string{
				"--aaa=A",
				"--bbb=B",
				"--bbb=DD",
				"--ccc=C",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetArgs(tt.args.argsMap, tt.args.extraArgs); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetArgs() = %+v\nWant = %+v", got, tt.want)
			}
		})
	}
}
