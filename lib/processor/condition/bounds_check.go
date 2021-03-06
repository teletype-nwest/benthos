// Copyright (c) 2018 Ashley Jeffs
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

package condition

import (
	"errors"

	"github.com/Jeffail/benthos/lib/log"
	"github.com/Jeffail/benthos/lib/metrics"
	"github.com/Jeffail/benthos/lib/types"
)

//------------------------------------------------------------------------------

func init() {
	Constructors[TypeBoundsCheck] = TypeSpec{
		constructor: NewBoundsCheck,
		description: `
BoundsCheck is a condition that checks a message against a set of bounds.`,
	}
}

//------------------------------------------------------------------------------

// BoundsCheckConfig contains configuration fields for the BoundsCheck
// processor.
type BoundsCheckConfig struct {
	MaxParts    int `json:"max_parts" yaml:"max_parts"`
	MinParts    int `json:"min_parts" yaml:"min_parts"`
	MaxPartSize int `json:"max_part_size" yaml:"max_part_size"`
	MinPartSize int `json:"min_part_size" yaml:"min_part_size"`
}

// NewBoundsCheckConfig returns a BoundsCheckConfig with default values.
func NewBoundsCheckConfig() BoundsCheckConfig {
	return BoundsCheckConfig{
		MaxParts:    100,
		MinParts:    1,
		MaxPartSize: 1 * 1024 * 1024 * 1024, // 1GB
		MinPartSize: 1,
	}
}

//------------------------------------------------------------------------------

// BoundsCheck is a condition that checks a message against a set of bounds.
type BoundsCheck struct {
	conf  Config
	log   log.Modular
	stats metrics.Type

	maxParts    int
	maxPartSize int
	minParts    int
	minPartSize int

	mApplied         metrics.StatCounter
	mSkipped         metrics.StatCounter
	mSkippedEmpty    metrics.StatCounter
	mSkippedNumParts metrics.StatCounter
	mSkippedPartSize metrics.StatCounter
}

// NewBoundsCheck returns a BoundsCheck condition.
func NewBoundsCheck(
	conf Config, mgr types.Manager, log log.Modular, stats metrics.Type,
) (Type, error) {
	return &BoundsCheck{
		log:              log.NewModule(".condition.bounds_check"),
		stats:            stats,
		maxParts:         conf.BoundsCheck.MaxParts,
		maxPartSize:      conf.BoundsCheck.MaxPartSize,
		minParts:         conf.BoundsCheck.MinParts,
		minPartSize:      conf.BoundsCheck.MinPartSize,
		mApplied:         stats.GetCounter("condition.bounds_check.applied"),
		mSkipped:         stats.GetCounter("condition.bounds_check.skipped"),
		mSkippedEmpty:    stats.GetCounter("condition.bounds_check.skipped.empty_message"),
		mSkippedNumParts: stats.GetCounter("condition.bounds_check.skipped.num_parts"),
		mSkippedPartSize: stats.GetCounter("condition.bounds_check.skipped.part_size"),
	}, nil
}

//------------------------------------------------------------------------------

// Check attempts to check a message part against a configured condition
func (c *BoundsCheck) Check(msg types.Message) bool {
	switch lParts := msg.Len(); {
	case lParts == 0:
		c.log.Debugln("Rejecting empty message")
		c.mSkipped.Incr(1)
		c.mSkippedEmpty.Incr(1)
		return false
	case lParts < c.minParts:
		c.log.Debugf(
			"Rejecting message due to parts below minimum (%v): %v\n",
			c.minParts, lParts,
		)
		c.mSkipped.Incr(1)
		c.mSkippedNumParts.Incr(1)
		return false
	case lParts > c.maxParts:
		c.log.Debugf(
			"Rejecting message due to parts exceeding limit (%v): %v\n",
			c.maxParts, lParts,
		)
		c.mSkipped.Incr(1)
		c.mSkippedNumParts.Incr(1)
		return false
	}

	var reject bool
	msg.Iter(func(i int, p types.Part) error {
		if size := len(p.Get()); size > c.maxPartSize || size < c.minPartSize {
			c.log.Debugf(
				"Rejecting message due to message part size (%v -> %v): %v\n",
				c.minPartSize, c.maxPartSize, size,
			)
			reject = true
			return errors.New("bounds_check part error")
		}
		return nil
	})

	if reject {
		c.mSkipped.Incr(1)
		c.mSkippedPartSize.Incr(1)
		return false
	}

	c.mApplied.Incr(1)
	return true
}
