package yiq

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

func yqrun(query string, json string, opts []string) (res string, err error) {
	if query == "" {
		query = "."
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	var b bytes.Buffer

	opts = append(opts, "eval")
	opts = append(opts, query)
	opts = append(opts, "-")
	cmd := exec.Command("yq", opts...)
	cmd.Stdin = bytes.NewBufferString(json)
	cmd.Stdout = &b
	cmd.Stderr = &b
	err = cmd.Start()
	if err != nil {
		return
	}

	c := make(chan error, 1)
	go func() { c <- cmd.Wait() }()
	select {
	case err = <-c:
		cancel()
	case <-ctx.Done():
		cmd.Process.Kill()
		<-c // Wait for it to return.
		cancel()
		err = fmt.Errorf("yq execution timeout")
		return
	}

	res = strings.TrimSpace(b.String())
	return
}
