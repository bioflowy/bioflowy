package jobs

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"strings"
	"syscall"

	"gopkg.in/yaml.v2"
)

type PipeName struct {
	name   string
	output bool
}
type Pipe struct {
	name       string
	outputPath string
	inputPath  []string
}
type Processes struct {
	pipes    map[string]*Pipe
	children []*Process
}
type Process struct {
	exitCode    int
	commands    []interface{}
	args        []string
	outputPipes []*Pipe
}

func createPipeName() string {
	file, _ := ioutil.TempFile("", "bioflowy-")
	pipePath := file.Name()
	os.Remove(pipePath)
	return pipePath
}
func (p *Pipe) getOutputFilePath() string {
	pipePath := createPipeName()
	p.outputPath = pipePath
	syscall.Mkfifo(pipePath, 0777)
	return pipePath
}
func (p *Pipe) getInputFilePath() string {
	pipePath := createPipeName()
	p.inputPath = append(p.inputPath, pipePath)
	syscall.Mkfifo(pipePath, 0777)
	return pipePath
}
func (p *Pipe) Run() {
	buf := make([]byte, 4096)
	writers := make([]io.WriteCloser, 0)
	r, _ := os.OpenFile(p.outputPath, os.O_RDONLY, os.ModeNamedPipe)
	defer r.Close()
	for _, path := range p.inputPath {
		w, _ := os.OpenFile(path, os.O_WRONLY, os.ModeNamedPipe)
		writers = append(writers, w)
		defer w.Close()
	}
	for {
		n, err := r.Read(buf)
		if n > 0 {
			for _, w := range writers {
				w.Write(buf[:n])
			}
		}
		if err != nil {
			if err != io.EOF {
				log.Fatalf("%v", err)
			}
			break
		}
	}
	log.Printf("Finished")
}

func (p *Process) Prepare(processes *Processes) {
	args := make([]string, 0)
	for _, a := range p.commands {
		switch a.(type) {
		case string:
			args = append(args, a.(string))
		case *PipeName:
			pipeName := a.(*PipeName)
			if pipeName.output {
				pipe := Pipe{
					name: pipeName.name,
				}
				processes.pipes[pipeName.name] = &pipe
				p.outputPipes = append(p.outputPipes, &pipe)
				pipe.getOutputFilePath()
				args = append(args, pipe.outputPath)
			} else {
				pipe, ok := processes.pipes[pipeName.name]
				if !ok {
					log.Fatalf("Unkown pipe name", pipeName.name)
				}
				filePath := pipe.getInputFilePath()
				args = append(args, filePath)
			}
		}
	}
	p.args = args
}
func print(w io.ReadCloser) {
	defer w.Close()
	buf := make([]byte, 1024)
	for {
		n, err := w.Read(buf)
		if n <= 0 {
			break
		}
		if err != nil {
			log.Fatalf("%v", err)
		}
		fmt.Printf("%s", buf[:n])
	}
}
func (p *Process) Execute(ch chan<- *Process) {
	cmd := exec.Command(p.args[0], p.args[1:]...)
	fmt.Printf("Process %s is started\n", strings.Join(p.args, " "))
	for _, pipes := range p.outputPipes {
		go pipes.Run()
	}
	var err error
	err = cmd.Run()
	p.exitCode = cmd.ProcessState.ExitCode()
	if err != nil {
		log.Printf("%v", err)
	}
	ch <- p
}
func NewPipeName(data map[interface{}]interface{}) (*PipeName, error) {
	name, ok := data["name"]
	if !ok {
		return nil, fmt.Errorf("name must be spefified")
	}
	output, ok := data["output"]
	if !ok {
		return nil, fmt.Errorf("output must be spefified")
	}
	return &PipeName{
		name:   name.(string),
		output: output.(bool),
	}, nil
}
func NewProcess(obj interface{}) (*Process, error) {
	data, ok := obj.(map[interface{}]interface{})
	if !ok {
		return nil, fmt.Errorf("children must be array")
	}
	as := make([]interface{}, 0)
	args, ok := data["args"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("args must be spefified")
	}
	for _, arg := range args {
		switch arg.(type) {
		case string:
			as = append(as, arg)
		case map[interface{}]interface{}:
			pipeName, err := NewPipeName(arg.(map[interface{}]interface{}))
			if err != nil {
				return nil, err
			}
			as = append(as, pipeName)
		}
	}
	return &Process{
		commands: as,
	}, nil
}
func NewProcesses(data map[interface{}]interface{}) (*Processes, error) {
	obj, ok := data["children"]
	if !ok {
		return nil, fmt.Errorf("children must be spefified")
	}
	children, ok := obj.([]interface{})
	if !ok {
		return nil, fmt.Errorf("children must be array")
	}
	procs := make([]*Process, 0)
	for _, child := range children {
		proc, err := NewProcess(child)
		if err != nil {
			return nil, err
		}
		procs = append(procs, proc)
	}
	return &Processes{
		children: procs,
		pipes:    make(map[string]*Pipe),
	}, nil
}
func LoadProcesses(filepath string) (*Processes, error) {
	var err error
	f, err := os.Open(filepath)
	if err != nil {
		return nil, err
	}
	buf, err := ioutil.ReadAll(f)
	if err != nil {
		return nil, err
	}
	m := make(map[interface{}]interface{})
	yaml.Unmarshal(buf, m)
	return NewProcesses(m)
}
func (ps *Processes) ExecuteAll() {
	ch := make(chan *Process)
	for _, p := range ps.children {
		p.Prepare(ps)
	}
	for _, p := range ps.children {
		go p.Execute(ch)
	}
	var count = 0
	for p := range ch {
		fmt.Printf("Process %s is finished %d\n", p.commands[0], p.exitCode)
		count++
		if len(ps.children) <= count {
			fmt.Printf("All processes have finished\n")
			break
		}
	}
}
func Test() {
	pipe1 := PipeName{
		name:   "pipe",
		output: true,
	}
	pipe2 := PipeName{
		name:   "pipe",
		output: false,
	}
	children := make([]*Process, 0)
	cmd1 := make([]interface{}, 2)
	cmd1[0] = "./test.py"
	cmd1[1] = pipe1
	cmd2 := make([]interface{}, 3)
	cmd2[0] = "./test2.py"
	cmd2[1] = pipe2
	cmd2[2] = "result.txt"
	children = append(children, &Process{
		commands: cmd1,
	})
	children = append(children, &Process{
		commands: cmd2,
	})
	ps := Processes{
		children: children,
		pipes:    make(map[string]*Pipe),
	}
	ps.ExecuteAll()

}
