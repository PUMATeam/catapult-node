package service

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/firecracker-microvm/firecracker-go-sdk"
	models "github.com/firecracker-microvm/firecracker-go-sdk/client/models"

	node "github.com/PUMATeam/catapult-node/pb"
)

const (
	firecrackerBinary = "./firecracker"
	vmDataPath        = "/var/vms"
	vmLogs            = "fc-logs"
)

//TODO better name needed
type fcHandler struct {
	vmID string
}

func (f *fcHandler) runVMM(ctx context.Context,
	vmCfg *node.VmConfig,
	logger log.Logger) (*firecracker.Machine, error) {
	if _, err := os.Stat(vmDataPath); err != nil {
		os.Mkdir(vmDataPath, os.ModeDir)
	}

	if _, err := os.Stat(vmLogs); err != nil {
		os.Mkdir(vmLogs, os.ModeDir)
	}

	_, err := os.Stat(firecrackerBinary)
	if os.IsNotExist(err) {
		return nil, fmt.Errorf("Binary %q does not exist: %v", firecrackerBinary, err)
	}

	if err != nil {
		return nil, fmt.Errorf("Failed to stat binary, %q: %v", firecrackerBinary, err)
	}
	socketPath := filepath.Join(vmDataPath, vmCfg.GetVmID().GetValue())
	os.Remove(socketPath)

	cfg := firecracker.Config{
		KernelImagePath: vmCfg.GetKernelImage(),
		SocketPath:      socketPath,
		Drives: []models.Drive{{
			DriveID:      firecracker.String("1"),
			PathOnHost:   firecracker.String(vmCfg.GetRootFileSystem()),
			IsRootDevice: firecracker.Bool(true),
			IsReadOnly:   firecracker.Bool(false),
		}},
		MachineCfg: models.MachineConfiguration{
			VcpuCount:  firecracker.Int64(vmCfg.GetVcpus()),
			MemSizeMib: firecracker.Int64(vmCfg.GetMemory()),
		},
		// TODO move to a constant
		// TODO extract
		LogLevel:    "Debug",
		LogFifo:     f.getFileNameByMethod("fifo", "log"),
		MetricsFifo: f.getFileNameByMethod("fifo", "metrics"),
	}

	cmd := firecracker.VMCommandBuilder{}.
		WithBin(firecrackerBinary).
		WithSocketPath(socketPath).
		Build(ctx)

	log.Infof("Creating new machine definition %v", cfg)
	m, err := firecracker.NewMachine(ctx,
		cfg,
		firecracker.WithProcessRunner(cmd),
		firecracker.WithLogger(log.NewEntry(&logger)))
	if err != nil {
		return nil, fmt.Errorf("Failed creating machine: %s", err)
	}

	log.Info("Starting machine...")
	errChan := make(chan error, 1)
	go func() {
		errChan <- m.Start(context.Background())
	}()

	select {
	case err = <-errChan:
		if err != nil {
			log.Error("fc error", err)
			return nil, err
		}
	case <-time.After(3 * time.Second):
		log.Info("No errors after 3 seconds, assuming success")
	}

	installSignalHandlers(ctx, m)
	return m, err
}

func (f *fcHandler) getFileNameByMethod(typ, method string) string {
	var marker string
	if method == "metrics" {
		marker = "-metrics"
	}

	return filepath.Join(vmLogs, fmt.Sprintf("%s%s.%s", f.vmID, marker, typ))
}

func installSignalHandlers(ctx context.Context, m *firecracker.Machine) {
	go func() {
		// Clear some default handlers installed by the firecracker SDK:
		signal.Reset(os.Interrupt, syscall.SIGTERM, syscall.SIGQUIT)
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt, syscall.SIGTERM, syscall.SIGQUIT)

		for {
			switch s := <-c; {
			case s == syscall.SIGTERM || s == os.Interrupt:
				log.Printf("Caught SIGINT, requesting clean shutdown")
				m.Shutdown(ctx)
			case s == syscall.SIGQUIT:
				log.Printf("Caught SIGTERM, forcing shutdown")
				m.StopVMM()
			}
		}
	}()
}

func (f *fcHandler) readPipe(method string) {
	pipePath := f.getFileNameByMethod("fifo", method)
	logPath := f.getFileNameByMethod("log", method)

	pipe, err := os.OpenFile(pipePath, os.O_RDONLY, os.ModeNamedPipe)
	if err != nil {
		log.Error(err)
	}

	var output *os.File
	if _, err := os.Stat(logPath); err != nil {
		output, err = os.Create(logPath)
		if err != nil {
			log.Errorf("Failed to create log file %s, %s", logPath, err)
		}
	}

	defer output.Close()
	defer pipe.Close()

	reader := bufio.NewReader(pipe)
	writer := bufio.NewWriter(output)

	for {
		line, err := reader.ReadBytes('\n')
		if err == nil {
			writer.Write(line)
			writer.Flush()
		}

		time.Sleep(1 * time.Second)
	}
}
