// Copyright (c) 2016 Uber Technologies, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package common

import (
	"sync"
	"testing"

	"github.com/Sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/uber-common/bark"
)

var baseLog = bark.NewLoggerFromLogrus(logrus.StandardLogger())

type UtilSuite struct {
	*require.Assertions // override suite.Suite.Assertions with require.Assertions; this means that s.NotNil(nil) will stop the test, not merely log an error
	suite.Suite
}

func TestUtilSuite(t *testing.T) {
	suite.Run(t, new(UtilSuite))
}

func (s *UtilSuite) SetupTest() {
	s.Assertions = require.New(s.T()) // Have to define our overridden assertions in the test setup. If we did it earlier, s.T() will return nil
}

var logFnInvocations int

func logFn() bark.Logger {
	logFnInvocations++
	return baseLog.WithField(TagModule, `util_test`)
}

func (s *UtilSuite) TestOverrideValueByPrefixNil() {
	logFnInvocations = 0
	x := OverrideValueByPrefix(logFn, `/foo/bar`, nil, 42, `TestOverrideValueByPrefixNil`)
	s.EqualValues(x, 42)
	s.Equal(logFnInvocations, 0)

	// Verify that we log an error every time that we pass invalid rules
	for i := 1; i < 4; i++ {
		x = OverrideValueByPrefix(logFn, `/foo/bar`, make([]string, 5), 42, `2TestOverrideValueByPrefixNil`)
		s.EqualValues(x, 42)
		s.Equal(logFnInvocations, i*5)
	}

	badRules := []string{`foo`, `/=`, `/foo=`, `/fo=bar`, `=baz`}

	logFnInvocations = 0
	for i := 1; i < 4; i++ {
		x = OverrideValueByPrefix(logFn, `/foo/bar`, badRules, 42, `3TestOverrideValueByPrefixNil`)
		s.EqualValues(x, 42)
		s.Equal(logFnInvocations, i*5)
	}
}

func (s *UtilSuite) TestOverrideValueByPrefixNormal() {

	defaultV := int64(7)
	rules := []string{
		`/foo=10`,
		`/foo$=11`,
		`/foo/bar=20`,
		`/foo/bar$=30`,
		`/bar=50`,
		`/=60`,
		`=70`,
	}

	expected := map[string]int64{
		``:            70,
		`/quz`:        60,
		`/fools`:      10,
		`/foo`:        11,
		`/foo/bar`:    30,
		`blergh`:      70,
		`/barrack`:    50,
		`/foo/barbaz`: 20,
	}

	phase := 0
	lfn := func() bark.Logger {
		return logFn().WithField(`phase`, phase)
	}

	logFnInvocations = 0

	for k, v := range expected {
		x := OverrideValueByPrefix(lfn, k, rules, defaultV, `TestOverrideValueByPrefixNormal`)
		s.EqualValues(v, x, `Unexected value for `+k)
	}

	y := logFnInvocations
	phase++

	for k, v := range expected {
		x := OverrideValueByPrefix(lfn, k, rules, defaultV, `TestOverrideValueByPrefixNormal`)
		s.EqualValues(v, x, `Unexected value for `+k)
	}

	s.Equal(y, logFnInvocations, `No additional logging should occur, reprocessing same rules`)
	phase++

	// Remove the '/' rule, verify that defaultV now prevails
	s.Equal(rules[5], `/=60`, `sanity`)
	rules[5] = `/fool=99` // override the default rule

	expected[`/fools`] = 99
	expected[`/quz`] = 70

	for j := 0; j < 2; j++ {
		for k, v := range expected {
			x := OverrideValueByPrefix(lfn, k, rules, defaultV, `TestOverrideValueByPrefixNormal`)
			s.EqualValues(v, x, `Unexected value for `+k)
		}

		s.Equal(y+1, logFnInvocations, `Change in override for fool should be logged, but only once`)
		phase++
	}

	// Remove the '' rule, verify that defaultV now prevails
	s.Equal(rules[6], `=70`, `sanity`)
	rules = rules[:6]

	expected[`blergh`] = defaultV
	expected[``] = defaultV
	expected[`/quz`] = defaultV

	for j := 0; j < 2; j++ {
		for k, v := range expected {
			x := OverrideValueByPrefix(lfn, k, rules, defaultV, `TestOverrideValueByPrefixNormal`)
			s.EqualValues(v, x, `Unexected value for `+k)
		}

		s.Equal(y+1, logFnInvocations, `Change in override for fool should be logged, but only once`)
		phase++
	}

}

// This test is only useful if the race flag is given
func (s *UtilSuite) TestOverrideValueByPrefixConcurrency() {
	var startersPistol sync.RWMutex
	var wg sync.WaitGroup

	startersPistol.Lock() // and load

	for i := 0; i < 1000; i++ {
		wg.Add(1)
		go func(i int) {
			startersPistol.RLock()
			OverrideValueByPrefix(logFn, `foo`, []string{`=1`, `fo=2`}, 3, `concurrent`)
			startersPistol.RUnlock()
			wg.Done()
		}(i)
	}

	startersPistol.Unlock() // bang!
	wg.Wait()
}
