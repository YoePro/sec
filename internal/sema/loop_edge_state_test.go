package sema

import (
	"reflect"
	"testing"
)

// Every break and continue edge records every loop-carried domain in one
// structured loopEdgeState, so a new domain cannot be carried on one edge
// kind but forgotten on another (governance frontend.loop-arena-generation-fixed-point).
//
// Rules:
//   - rules/control-flow/flowcontrol_while.md — §§28–29
//   - rules/control-flow/flowcontrol_for.md — §§30–31, 42
//   - rules/corrections/applied/correction25-20260823.md — Part I
func TestLoopEdgesCarryEveryDomain(t *testing.T) {
	a := NewAnalyzer()
	a.assigned = map[string]bool{"x": true}
	frame := a.pushLoopBreakFrame()
	a.recordLoopBreak()
	a.recordLoopContinue()
	edges := a.popLoopBreakFrame(frame)
	if len(edges.breaks) != 1 || len(edges.continues) != 1 {
		t.Fatalf("edges = %+v", edges)
	}
	for _, edge := range edges.edges() {
		value := reflect.ValueOf(edge)
		for index := 0; index < value.NumField(); index++ {
			if field := value.Field(index); field.Kind() == reflect.Map && field.IsNil() {
				t.Fatalf("loop edge domain %s was not captured", value.Type().Field(index).Name)
			}
		}
	}
	if !edges.continues[0].assigned["x"] {
		t.Fatal("continue edge lost definite-assignment state")
	}
}
