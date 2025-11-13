package slicex_test

import (
	"testing"

	"github.com/berquerant/cmdcomp/pkg/slicex"
	"github.com/stretchr/testify/assert"
)

func TestSplit(t *testing.T) {
	for _, tc := range []struct {
		title         string
		s             []int
		v             int
		before, after []int
	}{
		{
			title:  "middle",
			s:      []int{1, 2, 3},
			v:      2,
			before: []int{1},
			after:  []int{3},
		},
		{
			title:  "2v",
			s:      []int{1, 2, 1},
			v:      1,
			before: []int{},
			after:  []int{2, 1},
		},
		{
			title:  "tail",
			s:      []int{1, 2},
			v:      2,
			before: []int{1},
			after:  []int{},
		},
		{
			title:  "head",
			s:      []int{1, 2},
			v:      1,
			before: []int{},
			after:  []int{2},
		},
		{
			title:  "an element",
			s:      []int{1},
			v:      1,
			before: []int{},
			after:  []int{},
		},
		{
			title:  "not found",
			s:      []int{1},
			v:      0,
			before: []int{1},
		},
		{
			title: "empty",
			v:     1,
		},
	} {
		t.Run(tc.title, func(t *testing.T) {
			before, after := slicex.Split(tc.s, tc.v)
			assert.Equal(t, tc.before, before, "before")
			assert.Equal(t, tc.after, after, "after")
		})
	}
}
