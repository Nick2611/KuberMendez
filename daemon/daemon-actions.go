package daemon

import (
	"context"
	"fmt"
	apiserver "kuberMendez/api-server"
	"kuberMendez/events"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
	"syscall"
)

const stateDirectory = ".kubermendez"

func StartBackground() (int, error) {
	if err := os.MkdirAll(stateDirectory, 0755); err != nil {
		return 0, fmt.Errorf("create daemon state directory: %w", err)
	}

	executable, err := os.Executable()
	if err != nil {
		return 0, fmt.Errorf("find kubermendez executable: %w", err)
	}

	logFile, err := os.OpenFile(filepath.Join(stateDirectory, "daemon.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return 0, fmt.Errorf("open daemon log file: %w", err)
	}
	defer logFile.Close()

	cmd := exec.Command(executable, "daemon")
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}

	if err := cmd.Start(); err != nil {
		return 0, fmt.Errorf("start daemon process: %w", err)
	}

	pid := cmd.Process.Pid
	if err := os.WriteFile(filepath.Join(stateDirectory, "daemon.pid"), []byte(strconv.Itoa(pid)+"\n"), 0644); err != nil {
		return pid, fmt.Errorf("write daemon pid file: %w", err)
	}

	if err := cmd.Process.Release(); err != nil {
		return pid, fmt.Errorf("release daemon process: %w", err)
	}

	return pid, nil
}

func InitDaemon(ctx context.Context) {
	eventStream := make(chan events.Message, 1)
	var wg sync.WaitGroup

	wg.Add(2)
	go func() {
		defer wg.Done()
		apiserver.Start(ctx, eventStream)
	}()
	go func() {
		defer wg.Done()
		InitReconcile(ctx, eventStream)
	}()

	<-ctx.Done()
	deletePid()
	wg.Wait()

	fmt.Println("All kubermendez components closed")
}

func deletePid() {
	os.Remove(filepath.Join(stateDirectory, "daemon.pid"))
}
