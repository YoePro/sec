package deadlock

import "sort"

// components uses iterative Kosaraju passes and canonicalizes the partition.
// The partition is independent of node/edge registration and traversal order.
// Rules: rules/analysis/deadlock_analysis.md §§45,58,82.
func components(nodes []NodeID, edges []Edge, out, reverse [][]int, index map[NodeID]int) [][]int {
	seen := make([]bool, len(nodes))
	var order []int
	type frame struct{ node, next int }
	for root := range nodes {
		if seen[root] {
			continue
		}
		seen[root] = true
		stack := []frame{{node: root}}
		for len(stack) > 0 {
			last := &stack[len(stack)-1]
			if last.next == len(out[last.node]) {
				order = append(order, last.node)
				stack = stack[:len(stack)-1]
				continue
			}
			edge := out[last.node][last.next]
			last.next++
			next := index[edges[edge].To]
			if !seen[next] {
				seen[next] = true
				stack = append(stack, frame{node: next})
			}
		}
	}
	clear(seen)
	var groups [][]int
	for i := len(order) - 1; i >= 0; i-- {
		root := order[i]
		if seen[root] {
			continue
		}
		seen[root] = true
		stack, group := []int{root}, []int{}
		for len(stack) > 0 {
			node := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			group = append(group, node)
			for _, next := range reverse[node] {
				if !seen[next] {
					seen[next] = true
					stack = append(stack, next)
				}
			}
		}
		sort.Ints(group)
		groups = append(groups, group)
	}
	sort.Slice(groups, func(i, j int) bool { return groups[i][0] < groups[j][0] })
	return groups
}

// cycleWitness takes a selected internal edge, then a deterministic
// shortest return path inside the SCC, retaining exact producer evidence IDs.
// Reachability/path/progress compatibility is deliberately not inferred here.
// Rules: rules/analysis/deadlock_analysis.md §§8,45,48-50,82.
func cycleWitness(component, first int, edges []Edge, out [][]int, index map[NodeID]int, membership []int, visited []bool, previous []int) []EdgeID {
	edge := edges[first]
	start, target := index[edge.To], index[edge.From]
	if start == target {
		return []EdgeID{edge.ID}
	}
	queue := []int{start}
	visited[start] = true
	defer func() {
		for _, node := range queue {
			visited[node] = false
		}
	}()
	for head := 0; head < len(queue) && !visited[target]; head++ {
		for _, id := range out[queue[head]] {
			next := index[edges[id].To]
			if membership[next] != component || visited[next] {
				continue
			}
			visited[next], previous[next] = true, id
			queue = append(queue, next)
		}
	}
	var path []EdgeID
	for node := target; node != start; {
		id := previous[node]
		path = append(path, edges[id].ID)
		node = index[edges[id].From]
	}
	for i, j := 0, len(path)-1; i < j; i, j = i+1, j-1 {
		path[i], path[j] = path[j], path[i]
	}
	return append([]EdgeID{edge.ID}, path...)
}
