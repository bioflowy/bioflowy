package jobs

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"

	"gopkg.in/yaml.v2"
)

type PipeName string

type Std struct {
	Pipe string
	File string
}
type Pipe struct {
	name       PipeName
	outputPath string
	reader     func() (io.ReadCloser, error)
	writers    []func() (io.WriteCloser, error)
}

type ProcessContext struct {
	pipes map[PipeName]*Pipe
}

var re = regexp.MustCompile("[a-z]{3}")

type Process struct {
	cmd         *exec.Cmd
	Args        []string
	Inputs      []PipeName
	Outputs     []PipeName
	pipeFiles   map[string]string
	outputPipes []*Pipe
	Stdout      Std
	Stderr      Std
	Stdin       Std
}
type Processes struct {
	Children []*Process `yaml:"children"`
}

func createPipeName() string {
	file, _ := ioutil.TempFile("", "bioflowy-")
	pipePath := file.Name()
	os.Remove(pipePath)
	return pipePath
}
func replace(template string, variables map[string]string) (string, error) {
	str := ""
	for {
		idx := strings.Index(template, "$(")
		if idx < 0 {
			return str + template, nil
		}
		str = str + template[:idx]
		template = template[idx:]
		endIndex := strings.Index(template, ")")
		if endIndex < 0 {
			return "", fmt.Errorf("parenthes must be closed")
		}
		name := template[2:endIndex]
		value, ok := variables[name]
		if !ok {
			return "", fmt.Errorf("Unkown Variable %s", name)
		}
		str = str + value
		template = template[(endIndex + 1):]

	}
	return template, nil
}
func (p *Processes) preparePipeOut() (*ProcessContext, error) {
	pc := ProcessContext{
		pipes: make(map[PipeName]*Pipe),
	}
	for _, process := range p.Children {
		process.pipeFiles = make(map[string]string)
		for _, o := range process.Outputs {
			fileName := createPipeName()
			pipe := &Pipe{
				name:       o,
				outputPath: fileName,
			}
			pc.pipes[o] = pipe
			process.pipeFiles[string(o)] = fileName
			syscall.Mkfifo(fileName, 0777)
			process.outputPipes = append(process.outputPipes, pipe)
		}
		if process.Stdout.Pipe != "" {
			o := PipeName(process.Stdout.Pipe)
			pipe := &Pipe{
				name:       o,
				outputPath: "STDOUT",
			}
			pc.pipes[o] = pipe
			process.outputPipes = append(process.outputPipes, pipe)
		}
	}
	return &pc, nil
}
func (p *Processes) preparePipeIn(pc *ProcessContext) error {
	for _, process := range p.Children {
		for _, o := range process.Inputs {
			_, ok := pc.pipes[o]
			if !ok {
				return fmt.Errorf("Unkown Pipe Name %s", o)
			}
			fileName := createPipeName()
			process.pipeFiles[string(o)] = fileName
			syscall.Mkfifo(fileName, 0777)
			pipe := pc.pipes[o]
			pipe.addWriter(func() (io.WriteCloser, error) {
				return os.OpenFile(fileName, os.O_WRONLY, 0)
			})
		}
	}
	return nil
}
func (p *Pipe) addWriter(writer func() (io.WriteCloser, error)) {
	p.writers = append(p.writers, writer)
}
func (p *Pipe) Prepare(cmd *exec.Cmd) error {
	if p.outputPath == "STDOUT" {
		r, err := cmd.StdoutPipe()
		p.reader = func() (io.ReadCloser, error) {
			return r, err
		}
	} else {
		p.reader = func() (io.ReadCloser, error) {
			return os.OpenFile(p.outputPath, os.O_RDONLY, 0)
		}
	}
	return nil
}
func (p *Pipe) Run() {
	var err error
	r, err := p.reader()
	if err != nil {
		log.Fatalf("Error:%v", err)
	}
	log.Printf("Reader get:%v", r)
	ws := make([]io.WriteCloser, len(p.writers))
	for i, w := range p.writers {
		w, err := w()
		if err != nil {
			log.Fatalf("Error:%v", err)
		}
		log.Printf("Writer get:%v", w)
		defer w.Close()
		ws[i] = w
	}
	buf := make([]byte, 4096)
	for {
		n, err := r.Read(buf)
		if n > 0 {
			for _, w := range ws {
				_, err := w.Write(buf[:n])
				fmt.Println(string(buf[:n]))
				if err != nil {
					log.Fatalf("Error:%v", err)
				}
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
func (p *Process) Prepare(pc *ProcessContext) error {
	log.Printf("start process2 %s", p.Args[0])
	args := make([]string, 0)
	for _, arg := range p.Args {
		arg1, err := replace(arg, p.pipeFiles)
		if err != nil {
			log.Fatalf("%v", err)
			return err
		}
		args = append(args, arg1)
	}
	log.Printf("start process %s", strings.Join(args, " "))
	p.cmd = exec.Command(args[0], args[1:]...)
	if p.Stdin.Pipe != "" {
		pipe := pc.pipes[PipeName(p.Stdin.Pipe)]
		w, err := p.cmd.StdinPipe()
		pipe.addWriter(func() (io.WriteCloser, error) {
			return w, err
		})
	}
	for _, pipe := range p.outputPipes {
		pipe.Prepare(p.cmd)
	}
	return nil
}

func (p *Process) Execute(pc *ProcessContext, ch chan<- *Process) error {
	for _, pipe := range p.outputPipes {
		go pipe.Run()
	}
	var err error
	err = p.cmd.Start()
	if err != nil {
		log.Fatalf("Command Start %v", err)
	}
	log.Printf("Started %s", strings.Join(p.Args, " "))
	log.Printf("Waiting %s", strings.Join(p.Args, " "))
	processState, err := p.cmd.Process.Wait()
	if err != nil {
		log.Fatalf("Command Wait %v", err)
	}
	log.Printf("stop process %s exitCode = %d ", strings.Join(p.Args, " "), processState.ExitCode())
	ch <- p
	return nil
}
func (p *Processes) Execute() error {
	var err error
	pc, err := p.preparePipeOut()
	if err != nil {
		return err
	}
	err = p.preparePipeIn(pc)
	ch := make(chan *Process)
	for _, process := range p.Children {
		log.Printf("Prepare process %s", process.Args[0])
		process.Prepare(pc)
	}
	for _, process := range p.Children {
		log.Printf("start process1 %s", process.Args[0])
		go process.Execute(pc, ch)
	}
	var count = 0
	for process := range ch {
		fmt.Printf("Process %s is finished\n", process.Args[0])
		count++
		if len(p.Children) <= count {
			fmt.Printf("All processes have finished\n")
			break
		}
	}
	return err
}
func LoadProcesses(path string) (*Processes, error) {
	prevDir, _ := filepath.Abs(".")
	fmt.Println(prevDir)
	buf, err := ioutil.ReadFile(path)
	if err != nil {
		panic(err)
	}

	d := Processes{}
	err = yaml.UnmarshalStrict(buf, &d)
	if err != nil {
		return nil, err
	}
	fmt.Println(d)
	return &d, nil
}
