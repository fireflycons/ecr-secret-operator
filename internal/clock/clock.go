/*
Copyright 2023.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Clock knows how to get the current time,
// Provides a mechanism for mocking the time in tests
package clock

import "time"

type RealClock struct{}

func (RealClock) Now() time.Time { return time.Now() }

// clock knows how to get the current time.
// It can be used to fake out timing for testing.
type Clock interface {
	Now() time.Time
}

type TestClock struct {
	now time.Time
}

func (c TestClock) Now() time.Time {
	return c.now
}

func MustParseTime(RFC3339Time string) time.Time {

	t, err := time.Parse(time.RFC3339, RFC3339Time)

	if err != nil {
		panic(err)
	}

	return t
}

func MustParseDuration(d string) time.Duration {
	dur, err := time.ParseDuration(d)

	if err != nil {
		panic(err)
	}

	return dur
}

func (c *TestClock) SetTime(RFC3339Time string) {
	c.now = MustParseTime(RFC3339Time)
}

func (c *TestClock) Set(t time.Time) {
	c.now = t
}
