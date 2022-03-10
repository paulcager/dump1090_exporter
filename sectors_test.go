package main

import (
	"fmt"
	"testing"
)

func Test_sector(t *testing.T) {
	tests := []struct {
		numSectors       int
		directionDegrees int
		want             int
	}{
		{4, 0, 0},
		{4, 44, 0},
		{4, 46, 1},
		{4, 180, 2},
		{4, 250, 3},
		{4, 290, 3},
		{4, 360 - 46, 3},
		{4, 360 - 45, 0},
		{4, 360 - 44, 0},
		{4, 360, 0},

		{4, 360 + 90, 1},
		{4, -10, 0},
		{4, -90, 3},
		{4, -180, 2},
		{4, -270, 1},
		{4, -360, 0},

		{4, -(360*10 + 270), 1},

		{1, 0, 0},
		{1, 270, 0},
		{1, 360, 0},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%d@%d", tt.numSectors, tt.directionDegrees), func(t *testing.T) {
			if got := sector(tt.numSectors, tt.directionDegrees); got != tt.want {
				t.Errorf("sector() = %v, want %v", got, tt.want)
			}
		})
	}
}
