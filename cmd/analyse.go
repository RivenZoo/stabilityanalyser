// Copyright © 2019 NAME HERE <EMAIL ADDRESS>
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

package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"github.com/RivenZoo/backbone/logger"
	"github.com/spf13/cobra"
	"os"
	"regexp"
	"sort"
)

const (
	orderByFanIn    = "fan-in"
	orderByFanOut   = "fan-out"
	orderByVolatile = "volatile"
)

var analyseParam analyseArgs

type analyseArgs struct {
	orderBy string
	limit   int
}

// analyseCmd represents the analyse command
var analyseCmd = &cobra.Command{
	Use:   "analyse",
	Short: "Analyse module dependency",
	Long: `Receive module dependency from stdin, do statistics about module dependency fan-in and fan-out.
Module dependency described as ["module_name_A" -> "module_name_B";], one item per line.`,
	Run: func(cmd *cobra.Command, args []string) {
		analyseModuleDependency(analyseParam)
	},
}

func init() {
	rootCmd.AddCommand(analyseCmd)

	analyseCmd.Flags().StringVar(&analyseParam.orderBy, "order", "", "order by [fan-in | fan-out | volatile]")
	analyseCmd.Flags().IntVar(&analyseParam.limit, "limit", 0, "limit output, order should be set. default no limit")
	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// analyseCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// analyseCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

type depList []string

type moduleDepStat struct {
	Module      string  `json:"module"`
	FanIn       depList `json:"fan_in"`
	FanOut      depList `json:"fan_out"`
	FanInCount  int     `json:"fan_in_count"`
	FanOutCount int     `json:"fan_out_count"`
	I           float64 `json:"volatile"`
}

type depStatsortBy func(s1, s2 *moduleDepStat) bool

func (by depStatsortBy) Sort(l []*moduleDepStat) {
	sorter := &depStatSorter{
		depsStat: l,
		sortBy:   by,
	}
	sort.Sort(sorter)
}

type depStatSorter struct {
	depsStat []*moduleDepStat
	sortBy   depStatsortBy
}

// Len is part of sort.Interface.
func (s *depStatSorter) Len() int {
	return len(s.depsStat)
}

// Swap is part of sort.Interface.
func (s *depStatSorter) Swap(i, j int) {
	s.depsStat[i], s.depsStat[j] = s.depsStat[j], s.depsStat[i]
}

// Less is part of sort.Interface. It is implemented by calling the "by" closure in the sorter.
func (s *depStatSorter) Less(i, j int) bool {
	return s.sortBy(s.depsStat[i], s.depsStat[j])
}

func newModuleDepStat(moduleName string) *moduleDepStat {
	return &moduleDepStat{
		Module: moduleName,
		FanIn:  make(depList, 0),
		FanOut: make(depList, 0),
	}
}

func (s *moduleDepStat) String() string {
	b, _ := json.Marshal(s)
	return string(b)
}

func (s *moduleDepStat) UpdateIMetrics() {
	if len(s.FanIn)+len(s.FanOut) <= 0 {
		return
	}
	s.I = float64(len(s.FanOut)) / float64(len(s.FanIn)+len(s.FanOut))
}

func (s *moduleDepStat) UpdateFanIn(fanInModule string) {
	s.FanIn = append(s.FanIn, fanInModule)
	s.FanInCount = len(s.FanIn)
}

func (s *moduleDepStat) UpdateFanOut(fanOutModule string) {
	s.FanOut = append(s.FanOut, fanOutModule)
	s.FanOutCount = len(s.FanOut)
}

func analyseModuleDependency(args analyseArgs) {
	m := make(map[string]*moduleDepStat)
	s := bufio.NewScanner(os.Stdin)
	for s.Scan() {
		line := s.Text()
		src, dst, err := parseModuleDepItem(line)
		if err != nil {
			logger.Errorf("parseModuleDepItem line %s error %v", line, err)
			return
		}
		updateModuleDep(m, src, dst)
	}
	if s.Err() != nil {
		logger.Errorf("scan input error %v", s.Err())
		return
	}
	// output
	outputModuleDepStat(m, args)
}

var lineRegex = regexp.MustCompile(`"(.*)" -> "(.*)";`)

func parseModuleDepItem(line string) (src, dst string, err error) {
	s := lineRegex.FindStringSubmatch(line)
	if len(s) < 3 {
		err = fmt.Errorf("match line %s got %d fields", line, len(s))
		return
	}
	return s[1], s[2], nil
}

func updateModuleDep(m map[string]*moduleDepStat, src, dst string) {
	st, ok := m[src]
	if !ok {
		st = newModuleDepStat(src)
		m[src] = st
	}
	st.UpdateFanOut(dst)
	st.UpdateIMetrics()

	st, ok = m[dst]
	if !ok {
		st = newModuleDepStat(dst)
		m[dst] = st
	}
	st.UpdateFanIn(src)
	st.UpdateIMetrics()
}

func outputModuleDepStat(m map[string]*moduleDepStat, args analyseArgs) {
	switch args.orderBy {
	case orderByFanIn, orderByFanOut, orderByVolatile:
		outputOrdered(m, args)
	default:
		outputJSON(m)
	}
}

func outputJSON(m map[string]*moduleDepStat) {
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		logger.Errorf("output error %v", err)
		return
	}
	os.Stdout.Write(b)
}

func outputOrdered(m map[string]*moduleDepStat, args analyseArgs) {
	depsStat := make([]*moduleDepStat, 0, len(m))
	for _, st := range m {
		depsStat = append(depsStat, st)
	}
	var sortBy depStatsortBy
	switch args.orderBy {
	case orderByFanIn:
		sortBy = depStatsortBy(func(s1, s2 *moduleDepStat) bool {
			return s1.FanInCount >= s2.FanInCount
		})
	case orderByFanOut:
		sortBy = depStatsortBy(func(s1, s2 *moduleDepStat) bool {
			return s1.FanOutCount >= s2.FanOutCount
		})
	case orderByVolatile:
		sortBy = depStatsortBy(func(s1, s2 *moduleDepStat) bool {
			return s1.I >= s2.I
		})
	default:
		return
	}
	sortBy.Sort(depsStat)
	if args.limit > 0 {
		depsStat = depsStat[:args.limit]
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	encoder.Encode(depsStat)
}
