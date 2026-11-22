package main

const (
	cellWalkable = 0
	cellWall     = 1
	cellDoor     = 2
)

func isPassable(grid *[gridSize][gridSize]int, r, c int, hasKey bool) bool {
	if r < 0 || r >= gridSize || c < 0 || c >= gridSize {
		return false
	}
	v := grid[r][c]
	if v == cellWalkable {
		return true
	}
	if v == cellDoor && hasKey {
		return true
	}
	return false
}

// findNextGridStep returns the next grid cell (nextR, nextC) toward (goalR, goalC)
// using BFS on the 15x15 maze so hunters navigate around walls cleanly.
func findNextGridStep(grid *[gridSize][gridSize]int, startR, startC, goalR, goalC int, hasKey bool) (int, int, bool) {
	if !isPassable(grid, startR, startC, hasKey) {
		return startR, startC, false
	}
	if !isPassable(grid, goalR, goalC, hasKey) {
		// Pick nearest passable neighbor to goal if goal tile is a wall
		dirs := [][2]int{{-1, 0}, {1, 0}, {0, -1}, {0, 1}}
		found := false
		for _, d := range dirs {
			nr, nc := goalR+d[0], goalC+d[1]
			if isPassable(grid, nr, nc, hasKey) {
				goalR, goalC = nr, nc
				found = true
				break
			}
		}
		if !found {
			return startR, startC, false
		}
	}
	if startR == goalR && startC == goalC {
		return startR, startC, false
	}

	var visited [gridSize][gridSize]bool
	var parentR [gridSize][gridSize]int
	var parentC [gridSize][gridSize]int
	for r := 0; r < gridSize; r++ {
		for c := 0; c < gridSize; c++ {
			parentR[r][c] = -1
			parentC[r][c] = -1
		}
	}

	type node struct {
		r, c int
	}
	queue := make([]node, 0, gridSize*gridSize)
	visited[startR][startC] = true
	queue = append(queue, node{r: startR, c: startC})

	dirs := [][2]int{{-1, 0}, {1, 0}, {0, -1}, {0, 1}}
	reached := false

	for len(queue) > 0 {
		curr := queue[0]
		queue = queue[1:]
		if curr.r == goalR && curr.c == goalC {
			reached = true
			break
		}
		for _, d := range dirs {
			nr, nc := curr.r+d[0], curr.c+d[1]
			if !visited[nr][nc] && isPassable(grid, nr, nc, hasKey) {
				visited[nr][nc] = true
				parentR[nr][nc] = curr.r
				parentC[nr][nc] = curr.c
				queue = append(queue, node{r: nr, c: nc})
			}
		}
	}

	if !reached {
		return startR, startC, false
	}

	curR, curC := goalR, goalC
	for parentR[curR][curC] != -1 && !(parentR[curR][curC] == startR && parentC[curR][curC] == startC) {
		pr, pc := parentR[curR][curC], parentC[curR][curC]
		curR, curC = pr, pc
	}
	return curR, curC, true
}
