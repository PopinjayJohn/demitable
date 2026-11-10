package wasm

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/PopinjayJohn/demitable/internal/vault"

	"github.com/tetratelabs/wazero"
)

// TestHostileSpinGuestKilledWithinDeadline proves the CPU mitigation: a guest
// that never returns is terminated when its call context expires, and the
// instance is marked dead afterwards.
func TestHostileSpinGuestKilledWithinDeadline(t *testing.T) {
	eng, err := NewEngine(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer eng.Close()
	inst, err := eng.Instantiate(context.Background(), nil, InfiniteLoopModule(), vault.GM(), &fakeVault{}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer inst.Close(context.Background())

	timeout := 100 * time.Millisecond
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	start := time.Now()
	done := make(chan Result, 1)
	go func() { done <- inst.Invoke(ctx, Call{Op: "x"}) }()

	var res Result
	select {
	case res = <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("guest was not killed: Invoke did not return within 2s")
	}
	elapsed := time.Since(start)
	if res.OK {
		t.Fatal("expected a failed result from the spin guest")
	}
	if res.Error == nil || !strings.Contains(res.Error.Message, "context deadline exceeded") {
		t.Fatalf("expected context deadline exceeded, got %+v", res.Error)
	}
	// Killed no more than 50 ms past the deadline.
	if elapsed > timeout+50*time.Millisecond {
		t.Fatalf("guest not killed within 50ms of deadline: elapsed=%v timeout=%v", elapsed, timeout)
	}
	t.Logf("spin guest killed in %v (deadline %v)", elapsed, timeout)

	// Instance is dead after a deadline kill.
	res2 := inst.Invoke(context.Background(), Call{Op: "x"})
	if res2.Error == nil || res2.Error.Code != "not_available" {
		t.Fatalf("expected instance to be dead, got %+v", res2.Error)
	}
}

// TestHostileMemoryGrowPastCapFailsCleanly proves the memory mitigation: a
// memory.grow that would exceed the per-instance page cap fails (returns -1)
// instead of allocating.
func TestHostileMemoryGrowPastCapFailsCleanly(t *testing.T) {
	eng, err := NewEngine(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer eng.Close()

	compiled, err := eng.Runtime().CompileModule(context.Background(), MemoryGrowModule())
	if err != nil {
		t.Fatal(err)
	}
	defer compiled.Close(context.Background())
	mod, err := eng.Runtime().InstantiateModule(context.Background(), compiled, wazero.NewModuleConfig().WithName("growmod"))
	if err != nil {
		t.Fatal(err)
	}
	defer mod.Close(context.Background())

	grow := mod.ExportedFunction("grow")
	if grow == nil {
		t.Fatal("grow not exported")
	}

	// A small grow inside the cap succeeds (returns previous page count).
	res, err := grow.Call(context.Background(), 4)
	if err != nil {
		t.Fatal(err)
	}
	if len(res) == 0 {
		t.Fatal("grow returned no result")
	}
	if int32(res[0]) == -1 {
		t.Fatalf("small grow unexpectedly failed: %d", res[0])
	}

	// A grow that would exceed the cap fails cleanly with -1 and no error.
	res, err = grow.Call(context.Background(), 1000)
	if err != nil {
		t.Fatalf("grow past cap returned an error instead of -1: %v", err)
	}
	if len(res) == 0 || int32(res[0]) != -1 {
		t.Fatalf("expected grow past cap to return -1, got %v", res)
	}
}
