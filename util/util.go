package util

import (
	"bytes"
	"fmt"
	"os/exec"

	uuid "github.com/satori/go.uuid"
)

func StringToUUID(str string) uuid.UUID {
	s, err := uuid.FromString(str)
	if err != nil {
		return uuid.Nil
	}

	return s
}

func ExecuteCommand(command string, args ...string) (string, error) {
	cmd := exec.Command(command, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("command %q exited with %q: %v", cmd.Args, out, err)
	}

	return string(bytes.TrimSpace(out)), nil
}

// TODO make more generic with a map
func ExecuteCommandsPiped(command string, args []string, command2 string, args2 []string) (string, error) {
	cmd1 := exec.Command(command, args...)
	cmd2 := exec.Command(command, args2...)

	cmd2.Stdin, _ = cmd1.StdoutPipe()
	var out bytes.Buffer
	cmd2.Stdout = &out
	_ = cmd2.Start()
	_ = cmd1.Run()
	_ = cmd2.Wait()
	return string(bytes.TrimSpace(out.Bytes())), nil
}
