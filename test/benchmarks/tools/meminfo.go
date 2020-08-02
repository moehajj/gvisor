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

package tools

import (
	"fmt"
	regex "regexp"
	"strconv"
	"testing"
)

// Meminfo wraps measurements of MemAvailable using /proc/meminfo.
type Meminfo struct {
}

// MakeCmd returns a command for checking meminfo.
func (m *Meminfo) MakeCmd() []string {
	return []string{"cat", "/proc/meminfo"}
}

// Report takes two reads of meminfo, parses them, and reports the difference
// divided by b.N.
func (m *Meminfo) Report(b *testing.B, before, after string) {
	b.Helper()

	var beforeVal, afterVal float64
	var err error
	if beforeVal, err = m.parseMemAvailable(before); err != nil {
		b.Fatalf("could not parse before value %s: %v", before, err)
	}

	if afterVal, err = m.parseMemAvailable(after); err != nil {
		b.Fatalf("could not parse before value %s: %v", before, err)
	}
	val := 1024 * ((beforeVal - afterVal) / float64(b.N))
	b.ReportMetric(val, "average_container_size_bytes")
}

var memInfoRE = regex.MustCompile(`MemAvailable:\s*(\d+)\skB\n`)

// parseMemAvailable grabs the MemAvailable number from /proc/meminfo.
func (m *Meminfo) parseMemAvailable(data string) (float64, error) {
	match := memInfoRE.FindStringSubmatch(data)
	if len(match) < 2 {
		return 0, fmt.Errorf("couldn't find MemAvailable in %s", data)
	}
	return strconv.ParseFloat(match[1], 64)
}
