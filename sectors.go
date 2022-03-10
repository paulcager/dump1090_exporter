package main

func sector(numSectors int, directionDegrees int) int {
	directionDegrees = directionDegrees % 360
	if directionDegrees < 0 {
		directionDegrees += 360
	}
	offset := 360 / numSectors / 2
	return ((directionDegrees + offset) * (numSectors)) / 360 % numSectors
}
