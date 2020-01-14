package jobs

import (
	"bytes"
	"flag"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

var update = flag.Bool("update", false, "update golden files")

func TestLoadProcesses(t *testing.T) {
	type args struct {
		filepath string
	}
	tests := []struct {
		name    string
		args    args
		want    *Processes
		wantErr bool
	}{
		{
			name: "Load Processes",
			args: args{
				filepath: "testdata/load.yaml",
			},
			want: &Processes{
				Children: []*Process{
					&Process{
						Args:    []string{"./test.py", "$(pipe)"},
						Outputs: []PipeName{"pipe"},
					},
					&Process{
						Args:   []string{"./test2.py", "$(pipe)", "result.txt"},
						Inputs: []PipeName{"pipe"},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := LoadProcesses(tt.args.filepath)
			if (err != nil) != tt.wantErr {
				t.Errorf("LoadProcesses() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("LoadProcesses() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestProcesses_Execute(t *testing.T) {
	type args struct {
		filepath string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "Execute processes",
			args: args{
				filepath: "use_pipe.yaml",
			},
		},
		{
			name: "Execute processes",
			args: args{
				filepath: "use_stdout.yaml",
			},
		},
		{
			name: "Execute processes",
			args: args{
				filepath: "use_stdin.yaml",
			},
		},
		{
			name: "Execute processes",
			args: args{
				filepath: "use_stdoutin.yaml",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var err error
			prevDir, _ := filepath.Abs(".")
			os.Chdir("testdata")
			defer os.Chdir(prevDir)
			p, err := LoadProcesses(tt.args.filepath)
			if err = p.Execute(); (err != nil) != tt.wantErr {
				t.Errorf("Processes.Execute() error = %v, wantErr %v", err, tt.wantErr)
			}
			result := tt.args.filepath + ".result"
			golden := tt.args.filepath + ".golden"
			actual, _ := ioutil.ReadFile(result)
			if *update {
				ioutil.WriteFile(golden, actual, 0666)
			}
			expected, _ := ioutil.ReadFile(golden)
			if !bytes.Equal(actual, expected) {
				t.Errorf("Processes.Execute() actual = %v, expected %v", actual, expected)
			}
		})
	}
}

func Test_replace(t *testing.T) {
	type args struct {
		template  string
		variables map[string]string
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			name: "Replace test",
			args: args{
				"test1 $(Test2) test3",
				map[string]string{
					"Test2": "test2",
				},
			},
			want: "test1 test2 test3",
		},
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := replace(tt.args.template, tt.args.variables)
			if (err != nil) != tt.wantErr {
				t.Errorf("replace() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("replace() = %v, want %v", got, tt.want)
			}
		})
	}
}
