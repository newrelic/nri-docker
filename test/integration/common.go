package integration

import (
	"bytes"
	"fmt"
	"log"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/docker/docker/client"
	"github.com/newrelic/infra-integrations-sdk/args"
	"github.com/newrelic/nri-docker/src/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	eventuallyTimeout  = time.Minute
	eventuallyTick     = time.Second
	eventuallySlowTick = time.Second * 10
	imageTag           = "stress:latest"
	containerName      = "nri_docker_test"
	cpus               = 0.5
	pidsLimit          = "2000" // random limit for testing propouses
	memLimitStr        = "100M"
	memLimit           = 100 * 1024 * 1024 // 100 MB of memory
	memReservationStr  = "95M"
	memReservation     = 95 * 1024 * 1024 // 95MB of memory reservation
	cpuShares          = 2048             // cpuShares value for testing purposes
	swapBytes          = "200M"           // swapBytes is the total Memory + swap available and should be bigger than memLimit
)

var once sync.Once
var dockerClientVersion string

func newDocker(t *testing.T) *client.Client {
	t.Helper()
	// Get DockerClientVersion from default args avoiding parsing flags twice when executing multiple test.
	once.Do(func() {
		arg := config.ArgumentList{}
		require.NoError(t, args.SetupArgs(&arg))
		dockerClientVersion = arg.DockerClientVersion
	})

	docker, err := client.NewClientWithOpts(client.FromEnv, client.WithVersion(dockerClientVersion))
	require.NoError(t, err)
	return docker
}

func stress(t *testing.T, args ...string) (containerID string, closeFunc func()) {
	t.Helper()

	arguments := []string{
		"run", "-d",
		"--name", containerName,
		"--cpus", fmt.Sprint(cpus),
		"--cpu-shares", fmt.Sprint(cpuShares),
		"--memory-swap", swapBytes,
		"--memory", memLimitStr,
		"--pids-limit", pidsLimit,
		"--memory-reservation", memReservationStr,
		imageTag}
	arguments = append(arguments, args...)
	cmd := exec.Command("docker", arguments...)
	stdout := bytes.Buffer{}
	cmd.Stdout = &stdout
	stderr := bytes.Buffer{}
	cmd.Stderr = &stderr
	err := cmd.Run()
	outb, _ := stdout.ReadBytes('\n')
	log.Println(string(outb))
	errb, _ := stderr.ReadBytes(0)
	log.Println(string(errb))
	assert.NoError(t, err)

	return strings.Trim(string(outb), "\n\r"), func() {
		cmd := exec.Command("docker", "rm", "-f", containerName)
		out, err := cmd.CombinedOutput()
		log.Println(string(out))
		if err != nil {
			log.Println("error removing container", err)
		}
	}
}

func float64ToPointer(f float64) *float64 {
	return &f
}

func uint64ToPointer(u uint64) *uint64 {
	return &u
}
