// Copyright 2020 The gVisor Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package base

import (
	"context"
	"testing"
	"time"

	"gvisor.dev/gvisor/pkg/test/dockerutil"
	"gvisor.dev/gvisor/test/benchmarks/harness"
	"gvisor.dev/gvisor/test/benchmarks/tools"
)

// BenchmarkSizeEmpty creates N empty containers and memory usage from
// /proc/meminfo.
func BenchmarkSizeEmpty(b *testing.B) {
	// Time is not relevant for this benchmark.
	harness.ShutoffTimer(b)
	machine, err := h.GetMachine()
	if err != nil {
		b.Fatal("failed to get machine: %v", err)
	}
	defer machine.CleanUp()
	meminfo := tools.Meminfo{}
	ctx := context.Background()
	var containers []*dockerutil.Container

	// DropCaches before the test.
	harness.DropCaches(machine)

	// Check available memory on 'machine'.
	cmd := meminfo.MakeCmd()
	before, err := machine.RunCommand(cmd[0], cmd[1:]...)
	if err != nil {
		b.Fatalf("failed to get meminfo: %v", err)
	}

	// Make N containers.
	for i := 0; i < b.N; i++ {
		container := machine.GetContainer(ctx, b)
		containers = append(containers, container)
		if err := container.Spawn(ctx, dockerutil.RunOpts{
			Image: "benchmarks/empty",
		}, "sleep", "1000000000"); err != nil {
			cleanUpContainers(ctx, containers)
			b.Fatalf("failed to run container: %v", err)
		}
		// Wait until each is running.
		for status, err := container.Status(ctx); !status.Running; {
			if err != nil {
				cleanUpContainers(ctx, containers)
				b.Fatalf("failed get status for container: %v", err)
			}
		}
	}
	defer cleanUpContainers(ctx, containers)

	// Wait after creating containers.
	time.Sleep(time.Second)

	// Drop caches again before second measurement.
	harness.DropCaches(machine)

	// Check available memory after containers are up.
	after, err := machine.RunCommand(cmd[0], cmd[1:]...)
	if err != nil {
		b.Fatalf("failed to get meminfo: %v", err)
	}
	meminfo.Report(b, before, after)
}

// BenchmarkSizeNginx starts N containers running Nginx, checks that they're
// serving, and checks memory used based on /proc/meminfo.
func BenchmarkSizeNginx(b *testing.B) {
	// Timer is not valid for this test.
	harness.ShutoffTimer(b)
	machine, err := h.GetMachine()
	if err != nil {
		b.Fatal("failed to get machine with: %v", err)
	}
	defer machine.CleanUp()

	// DropCaches for the first measurement.
	harness.DropCaches(machine)

	// Measure MemAvailable before creating containers.
	meminfo := tools.Meminfo{}
	cmd := meminfo.MakeCmd()
	before, err := machine.RunCommand(cmd[0], cmd[1:]...)
	if err != nil {
		b.Fatalf("failed to run meminfo commend: %v", err)
	}

	// Make N Nginx containers.
	ctx := context.Background()
	runOpts := dockerutil.RunOpts{
		Image: "benchmarks/nginx",
		Ports: []int{80},
	}
	servers := startServers(ctx, b, machine, runOpts, []string{})
	defer cleanUpContainers(ctx, servers)

	// DropCaches after servers are created.
	harness.DropCaches(machine)
	// Take after measurement.
	after, err := machine.RunCommand(cmd[0], cmd[1:]...)
	if err != nil {
		b.Fatalf("failed to run meminfo commend: %v", err)
	}
	meminfo.Report(b, before, after)
}

// BenchmarkSizeNode starts N containers running a Node app, checks that
// they're serving, and checks memory used based on /proc/meminfo.
func BenchmarkSizeNode(b *testing.B) {
	// Timer is not valid for this test.
	harness.ShutoffTimer(b)
	machine, err := h.GetMachine()
	if err != nil {
		b.Fatal("failed to get machine with: %v", err)
	}
	defer machine.CleanUp()

	// Make a redis instance for Node to connect.
	ctx := context.Background()
	redis, redisIP := getRedisInstance(ctx, b, machine)
	defer redis.CleanUp(ctx)

	// DropCaches after redis is created.
	harness.DropCaches(machine)

	// Take before measurement.
	meminfo := tools.Meminfo{}
	cmd := meminfo.MakeCmd()
	before, err := machine.RunCommand(cmd[0], cmd[1:]...)
	if err != nil {
		b.Fatalf("failed to run meminfo commend: %v", err)
	}

	// Create N Node servers.
	runOpts := dockerutil.RunOpts{
		Image:   "benchmarks/node",
		WorkDir: "/usr/src/app",
		Links:   []string{redis.MakeLink("redis")},
		Ports:   []int{8080},
	}
	cmd = []string{"node", "index.js", redisIP.String()}
	servers := startServers(ctx, b, machine, runOpts, cmd)
	defer cleanUpContainers(ctx, servers)

	// DropCaches after servers are created.
	harness.DropCaches(machine)
	// Take after measurement.
	cmd = meminfo.MakeCmd()
	after, err := machine.RunCommand(cmd[0], cmd[1:]...)
	if err != nil {
		b.Fatalf("failed to run meminfo commend: %v", err)
	}
	meminfo.Report(b, before, after)
}

// run runs a server workload defined by 'runOpts' and 'cmd'.
// 'clientMachine' is used to connect to the server on 'machine'.
func startServers(ctx context.Context, b *testing.B, machine harness.Machine,
	runOpts dockerutil.RunOpts, cmd []string) []*dockerutil.Container {
	b.Helper()
	var servers []*dockerutil.Container

	// Create N servers and wait until each of them is serving.
	for i := 0; i < b.N; i++ {
		server := machine.GetContainer(ctx, b)
		servers = append(servers, server)
		if err := server.Spawn(ctx, runOpts, cmd...); err != nil {
			cleanUpContainers(ctx, servers)
			b.Fatalf("failed to spawn node instance: %v", err)
		}

		// We use only one machine, so use the containerIP.
		servingIP, err := server.FindIP(ctx, false)
		if err != nil {
			cleanUpContainers(ctx, servers)
			b.Fatalf("failed to get ip from server: %v", err)
		}

		// Wait until the server is up.
		if err = harness.WaitUntilServing(ctx, machine, servingIP, runOpts.Ports[0]); err != nil {
			cleanUpContainers(ctx, servers)
			b.Fatalf("failed to wait for serving")
		}
	}
	return servers
}

// cleanUpContainers cleans up a slice of containers.
func cleanUpContainers(ctx context.Context, servers []*dockerutil.Container) {
	for _, server := range servers {
		server.CleanUp(ctx)
	}
}
